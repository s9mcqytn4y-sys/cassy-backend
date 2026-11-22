package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/domain"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/pkg/database"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/repository/postgres"
)

const (
	guestSplitPayWindow = 15 * time.Minute
	maxGuestSplitLabel  = 40
	splitShareTokenLen  = 43 // 32 random bytes, base64url without padding
)

var (
	ErrGuestSplitNotAllowed = errors.New("this order can no longer be divided")
	ErrSplitShareNotFound   = errors.New("split link not found or expired")
)

// GuestSplitLink carries the only copy of a share token; it is never readable again.
type GuestSplitLink struct {
	SplitNumber int    `json:"split_number"`
	Label       string `json:"label"`
	Amount      int64  `json:"amount"`
	Token       string `json:"token"`
}

// GuestSplitStatus is what the ordering guest sees while the shares come in.
type GuestSplitStatus struct {
	SplitNumber int    `json:"split_number"`
	Label       string `json:"label"`
	Amount      int64  `json:"amount"`
	Status      string `json:"status"`
}

// GuestSharePayment is the charge or cashier-intent view shown to a link holder, without order or payer details.
type GuestSharePayment struct {
	ID           uuid.UUID  `json:"id,omitempty"`
	Method       string     `json:"method,omitempty"`
	Bank         string     `json:"bank,omitempty"`
	QRString     string     `json:"qr_string,omitempty"`
	VANumber     string     `json:"va_number,omitempty"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	Amount       int64      `json:"amount"`
	Status       string     `json:"status"`
	Instructions string     `json:"instructions,omitempty"`
}

func (s *GuestService) ownsOrder(ctx context.Context, tx *sqlx.Tx, g *GuestIdentity, orderID uuid.UUID) error {
	owner, err := s.repo.OrderGuestSession(ctx, tx, g.StoreID, orderID)
	if errors.Is(err, postgres.ErrOrderNotFound) {
		return ErrGuestOrderNotFound
	}
	if err != nil {
		return err
	}
	if owner == nil || *owner != g.SessionID {
		return ErrGuestOrderNotFound
	}
	return nil
}

// CreateSplits divides an untouched pay-first order into per-person shares. It works once per order,
// before any QR exists, so no guest can be charged against an amount that later changes.
func (s *GuestService) CreateSplits(ctx context.Context, g *GuestIdentity, orderID uuid.UUID, in CloseTabInput) ([]GuestSplitLink, error) {
	if err := validateSplitInput(&in); err != nil {
		return nil, err
	}
	var links []GuestSplitLink
	err := database.WithTenantTx(ctx, s.db, g.StoreID, func(tx *sqlx.Tx) error {
		if in.Strategy == "equal" {
			if err := requireFeatureFlagTx(ctx, tx, g.StoreID, FeatureGuestSplitEqual, ErrSplitBillDisabled); err != nil {
				return err
			}
		} else if in.Strategy == "by_item" {
			if err := requireFeatureFlagTx(ctx, tx, g.StoreID, FeatureGuestSplitByItem, ErrSplitBillDisabled); err != nil {
				return err
			}
		}
		if err := s.ownsOrder(ctx, tx, g, orderID); err != nil {
			return err
		}
		now := s.nowFunc().UTC()
		_, splits, err := dividePayFirstOrderTx(ctx, tx, s.payments.orderRepo, s.payments.paymentRepo, s.tabs, g.StoreID, orderID, in, now)
		if err != nil {
			return err
		}
		links = make([]GuestSplitLink, len(splits))
		for i, sp := range splits {
			token, hash, err := randomToken()
			if err != nil {
				return err
			}
			if err := s.repo.InsertSplitShare(ctx, tx, g.StoreID, orderID, sp.ID, hash, now.Add(guestSessionTTL)); err != nil {
				return err
			}
			links[i] = GuestSplitLink{SplitNumber: sp.SplitNumber, Label: sp.Label, Amount: sp.TotalAmount, Token: token}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return links, nil
}

// validateSplitInput checks the strategy and trims share labels; amounts are always computed later.
func validateSplitInput(in *CloseTabInput) error {
	if in.Strategy != "equal" && in.Strategy != "by_item" {
		return fmt.Errorf("%w: %q", ErrInvalidSplitStrategy, in.Strategy)
	}
	f := fieldErrors{}
	for i := range in.Splits {
		in.Splits[i].Label = f.text("label", in.Splits[i].Label, 0, maxGuestSplitLabel)
	}
	return f.err()
}

// dividePayFirstOrderTx replaces the single unpaid share of a pending pay-first order with
// server-priced shares (equal or by item) and gives them a fresh pay window. It refuses
// (ErrGuestSplitNotAllowed) once any money or payment attempt is attached, the order expired, or it was
// already divided, so no share is ever priced against an amount that later changes. The order row and
// its active payment are locked. Kitchen tickets are not touched: a pay-first order reaches the KDS only
// when the final share settles.
func dividePayFirstOrderTx(
	ctx context.Context, tx *sqlx.Tx,
	orders *postgres.OrderRepository, payments *postgres.PaymentRepository, tabs *postgres.TabRepository,
	storeID, orderID uuid.UUID, in CloseTabInput, now time.Time,
) (*domain.Order, []domain.PaymentSplit, error) {
	if err := requireSplitBill(ctx, tx, storeID); err != nil {
		return nil, nil, err
	}
	order, existing, err := orders.GetOrderByIDForUpdate(ctx, tx, storeID, orderID)
	if err != nil {
		return nil, nil, err
	}
	if order.BillingMode != domain.BillingModePayFirst || order.Status != domain.OrderStatusPendingPayment ||
		order.PaidAmount != 0 || order.ExpiresAt == nil || !now.Before(*order.ExpiresAt) ||
		len(existing) != 1 || existing[0].Status != domain.SplitBillStatusUnpaid {
		return nil, nil, ErrGuestSplitNotAllowed
	}
	active, err := payments.GetActivePaymentForSplitForUpdate(ctx, tx, storeID, existing[0].ID)
	if err != nil && !errors.Is(err, postgres.ErrPaymentNotFound) {
		return nil, nil, err
	}
	if active != nil {
		return nil, nil, ErrGuestSplitNotAllowed
	}
	items, err := tabs.GetOrderItemsWithOptionsByOrder(ctx, tx, storeID, orderID)
	if err != nil {
		return nil, nil, err
	}
	splits, splitItems, err := allocateSplits(order, items, in, in.Strategy, now)
	if err != nil {
		return nil, nil, err
	}
	if err := tabs.CancelPaymentSplitsTx(ctx, tx, storeID, orderID); err != nil {
		return nil, nil, err
	}
	if err := tabs.CreatePaymentSplitsTx(ctx, tx, splits, splitItems); err != nil {
		return nil, nil, fmt.Errorf("failed to save payment splits: %w", err)
	}
	deadline := now.Add(guestSplitPayWindow)
	order.ExpiresAt = &deadline
	if err := orders.UpdateOrderStatusAndStock(ctx, tx, order); err != nil {
		return nil, nil, err
	}
	return order, splits, nil
}

// ListSplits reports each live share of one of this guest's orders, without tokens.
func (s *GuestService) ListSplits(ctx context.Context, g *GuestIdentity, orderID uuid.UUID) ([]GuestSplitStatus, error) {
	out := []GuestSplitStatus{}
	err := database.WithTenantTx(ctx, s.db, g.StoreID, func(tx *sqlx.Tx) error {
		if err := s.ownsOrder(ctx, tx, g, orderID); err != nil {
			return err
		}
		splits, err := s.tabs.GetPaymentSplitsByOrderID(ctx, tx, g.StoreID, orderID)
		if err != nil {
			return err
		}
		for _, sp := range splits {
			if sp.Status != domain.SplitBillStatusCancelled {
				out = append(out, GuestSplitStatus{SplitNumber: sp.SplitNumber, Label: sp.Label, Amount: sp.TotalAmount, Status: string(sp.Status)})
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (s *GuestService) resolveShare(ctx context.Context, token string) (*postgres.SplitShare, error) {
	if len(token) != splitShareTokenLen {
		return nil, ErrSplitShareNotFound
	}
	sum := sha256.Sum256([]byte(token))
	share, err := s.repo.ResolveSplitShare(ctx, s.db, sum[:])
	if err != nil {
		return nil, err
	}
	if share == nil || !s.nowFunc().Before(share.ExpiresAt) {
		return nil, ErrSplitShareNotFound
	}
	return share, nil
}

// SplitShare is the public view behind a link: store, label, amount and status only.
func (s *GuestService) SplitShare(ctx context.Context, token string) (*postgres.SplitShareView, error) {
	share, err := s.resolveShare(ctx, token)
	if err != nil {
		return nil, err
	}
	var view *postgres.SplitShareView
	err = database.WithTenantTx(ctx, s.db, share.StoreID, func(tx *sqlx.Tx) error {
		var err error
		view, err = s.repo.GetSplitShareView(ctx, tx, share.StoreID, share.SplitID)
		if err != nil {
			return err
		}
		flags, err := loadFeatureFlagsTx(ctx, tx, share.StoreID)
		if err != nil {
			return err
		}
		view.MultiPaymentEnabled = flags[FeatureGuestSplitMultiPayment]
		return nil
	})
	if err != nil {
		return nil, err
	}
	return view, nil
}

// PaySplitShare starts (or replays) a payment charge (qris or va) or records a cashier payment intent
// (method="cashier") for one split share (P-06).
func (s *GuestService) PaySplitShare(ctx context.Context, token, idempotencyKey, method, bank string) (*GuestSharePayment, error) {
	f := fieldErrors{}
	key := f.text("idempotency_key", idempotencyKey, 1, maxGuestIdempotentKey)
	normalizedMethod := strings.ToLower(strings.TrimSpace(method))
	if normalizedMethod == "" {
		normalizedMethod = "qris"
	}
	if normalizedMethod == "bank_transfer" {
		normalizedMethod = "va"
	}
	normalizedBank := strings.ToLower(strings.TrimSpace(bank))
	switch normalizedMethod {
	case "qris", "cashier":
	case "va":
		if !allowedVABanks[normalizedBank] {
			f.add("bank", "harus bca, bni, bri, mandiri, atau permata")
		}
	default:
		f.add("method", "harus qris, va, atau cashier")
	}
	if err := f.err(); err != nil {
		return nil, err
	}

	share, err := s.resolveShare(ctx, token)
	if err != nil {
		return nil, err
	}

	if normalizedMethod != "qris" {
		err = database.WithTenantTx(ctx, s.db, share.StoreID, func(tx *sqlx.Tx) error {
			return requireFeatureFlagTx(ctx, tx, share.StoreID, FeatureGuestSplitMultiPayment, ErrSplitMultiPaymentDisabled)
		})
		if err != nil {
			return nil, err
		}
	}

	if normalizedMethod == "cashier" {
		var out *GuestSharePayment
		err = database.WithTenantTx(ctx, s.db, share.StoreID, func(tx *sqlx.Tx) error {
			var split struct {
				ID          uuid.UUID `db:"id"`
				SplitNumber int       `db:"split_number"`
				Label       string    `db:"label"`
				TotalAmount int64     `db:"total_amount"`
				Status      string    `db:"status"`
			}
			if err := tx.GetContext(ctx, &split, `
				SELECT id, split_number, label, total_amount, status
				FROM payment_splits
				WHERE store_id = $1 AND id = $2
				FOR UPDATE`, share.StoreID, share.SplitID); err != nil {
				return fmt.Errorf("failed to lock split share: %w", err)
			}
			if split.Status != string(domain.SplitBillStatusUnpaid) && split.Status != string(domain.SplitBillStatusPending) {
				return ErrGuestSplitNotAllowed
			}
			now := s.nowFunc().UTC()
			if _, err := tx.ExecContext(ctx, `
				UPDATE payment_splits SET cashier_requested_at = $3, updated_at = $3
				WHERE store_id = $1 AND id = $2`, share.StoreID, share.SplitID, now); err != nil {
				return fmt.Errorf("failed to mark split cashier request: %w", err)
			}
			payload, err := json.Marshal(map[string]any{
				"store_id":     share.StoreID.String(),
				"order_id":     share.OrderID.String(),
				"split_id":     split.ID.String(),
				"split_number": split.SplitNumber,
				"label":        split.Label,
				"amount":       split.TotalAmount,
				"requested_at": now.Format(time.RFC3339),
			})
			if err != nil {
				return err
			}
			if err := s.payments.orderRepo.InsertOutboxEvent(ctx, tx, &domain.OutboxEvent{
				ID:            uuid.New(),
				StoreID:       share.StoreID,
				EventType:     "split.cashier_requested",
				AggregateType: "order",
				AggregateID:   share.OrderID.String(),
				Payload:       string(payload),
			}); err != nil {
				return err
			}
			out = &GuestSharePayment{
				Method:       "cashier",
				Amount:       split.TotalAmount,
				Status:       "cashier_requested",
				Instructions: "Tunjukkan layar ini ke kasir untuk menyelesaikan pembayaran bagian Anda.",
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		return out, nil
	}

	payMethod := domain.PaymentMethodQRIS
	if normalizedMethod == "va" {
		payMethod = domain.PaymentMethodBankTransfer
	}
	res, err := s.payments.InitiatePayment(ctx, share.StoreID, InitiatePaymentInput{
		OrderID:        share.OrderID,
		SplitID:        &share.SplitID,
		IdempotencyKey: "s:" + share.SplitID.String() + ":" + key,
		Method:         payMethod,
		Bank:           normalizedBank,
	})
	if err != nil {
		return nil, err
	}
	p := res.Payment
	return &GuestSharePayment{
		ID:        p.ID,
		Method:    normalizedMethod,
		Bank:      p.Bank,
		QRString:  p.QRString,
		VANumber:  p.VANumber,
		ExpiresAt: p.ExpiresAt,
		Amount:    p.Amount,
		Status:    string(p.Status),
	}, nil
}
