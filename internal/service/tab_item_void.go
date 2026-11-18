package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/domain"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/pkg/rbac"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/repository/postgres"
)

// ErrTabItemNotFound is returned when the line does not belong to the open tab.
var ErrTabItemNotFound = errors.New("item tidak ditemukan pada tab ini")

// VoidTabItemInput takes back part or all of one line of an open tab. Authority follows decision O1
// on the voided amount: the cashier's own PIN up to the owner threshold, above it an owner PIN or an
// approved void_item request (ADR-007) for exactly this line and quantity.
type VoidTabItemInput struct {
	Quantity   int        `json:"quantity"`
	Reason     string     `json:"reason"`
	PIN        string     `json:"pin"`
	OwnerPIN   *string    `json:"owner_pin"`
	ApprovalID *uuid.UUID `json:"approval_id"`
}

type VoidTabItemResult struct {
	Order *domain.Order `json:"order"`
	// RemainingQuantity is 0 when the whole line was removed.
	RemainingQuantity int `json:"remaining_quantity"`
	// StockReturned is false when the kitchen had already started the line, so the units are waste.
	StockReturned bool `json:"stock_returned"`
}

type tabItemRow struct {
	ID          uuid.UUID  `db:"id"`
	Quantity    int        `db:"quantity"`
	Price       int64      `db:"snapshot_price"`
	Name        string     `db:"snapshot_name"`
	RoundID     *uuid.UUID `db:"round_id"`
	PrepStation string     `db:"snapshot_prep_station"`
}

// voidTabItemAmount is what the voided units are worth before tax and service; O1 and the
// void_item approval are both judged on it.
func voidTabItemAmount(ctx context.Context, tx *sqlx.Tx, storeID, itemID uuid.UUID, qty int) (tabItemRow, int64, error) {
	var item tabItemRow
	err := tx.GetContext(ctx, &item, `SELECT oi.id, oi.quantity, oi.snapshot_price, oi.snapshot_name, oi.round_id, oi.snapshot_prep_station
		FROM order_items oi JOIN orders o ON o.store_id = oi.store_id AND o.id = oi.order_id
		WHERE oi.store_id = $1 AND oi.id = $2 AND o.billing_mode = $3 AND o.status = $4`,
		storeID, itemID, domain.BillingModeOpenTab, domain.OrderStatusOpen)
	if errors.Is(err, sql.ErrNoRows) {
		return item, 0, ErrTabItemNotFound
	}
	if err != nil {
		return item, 0, fmt.Errorf("failed to load tab item: %w", err)
	}
	return item, item.Price * int64(qty), nil
}

// VoidTabItem shrinks or removes one line of an open tab, returns its stock when the kitchen has not
// started it, recalculates the tab total, and records VOID_ORDER_ITEM. Closed tabs must be reopened
// first, so no split or refund line ever points at a changed item.
func (s *OperationalService) VoidTabItem(ctx context.Context, actor *AuthenticatedStaff, orderID, itemID uuid.UUID, in VoidTabItemInput) (*VoidTabItemResult, error) {
	if actor == nil {
		return nil, ErrInvalidSession
	}
	if !actor.Can(rbac.OrderVoid) {
		return nil, ErrForbidden
	}
	reason := strings.TrimSpace(in.Reason)
	fields := map[string]string{}
	if reason == "" {
		fields["reason"] = "alasan pembatalan item wajib diisi"
	}
	if in.Quantity < 1 {
		fields["quantity"] = "jumlah yang dibatalkan minimal 1"
	}
	if len(fields) > 0 {
		return nil, &ValidationError{Fields: fields}
	}

	storeID := actor.Staff.Store.ID
	var result *VoidTabItemResult
	err := s.pins.withPINTx(ctx, storeID, func(tx *sqlx.Tx) error {
		order, _, err := s.orderRepo.GetOrderByIDForUpdate(ctx, tx, storeID, orderID)
		if errors.Is(err, postgres.ErrOrderNotFound) {
			return ErrTabNotFound
		}
		if err != nil {
			return err
		}
		if order.BillingMode != domain.BillingModeOpenTab {
			return ErrTabNotFound
		}
		if order.Status != domain.OrderStatusOpen {
			return ErrTabNotOpen
		}
		item, amount, err := voidTabItemAmount(ctx, tx, storeID, itemID, in.Quantity)
		if err != nil {
			return err
		}
		if !orderHasItem(order, item.ID) {
			return ErrTabItemNotFound
		}
		if in.Quantity > item.Quantity {
			return &ValidationError{Fields: map[string]string{"quantity": fmt.Sprintf("maksimal %d", item.Quantity)}}
		}

		qty := in.Quantity
		claim := &approvalClaim{ID: in.ApprovalID, Action: ApprovalVoidItem, Ref: item.ID, Params: approvalParams{Quantity: &qty}}
		ownerOverrideID, err := s.authorizeAmount(ctx, tx, actor, amount, in.PIN, in.OwnerPIN, false, claim)
		if err != nil {
			return err
		}

		var started bool
		if err := tx.GetContext(ctx, &started, `SELECT EXISTS (SELECT 1 FROM kds_tickets
			WHERE store_id = $1 AND order_id = $2 AND round_id IS NOT DISTINCT FROM $3::uuid AND station = $4::text
			  AND status IN ('preparing', 'ready', 'served'))`, storeID, order.ID, item.RoundID, item.PrepStation); err != nil {
			return fmt.Errorf("failed to check kitchen progress: %w", err)
		}
		stockReturned := false
		if !started && (order.StockStatus == domain.OrderStockStatusReserved || order.StockStatus == domain.OrderStockStatusCommitted) {
			products, options := computeItemStockQuantities(order, map[uuid.UUID]int{item.ID: qty})
			if err := restoreStockTx(ctx, tx, s.orderRepo, storeID, products, options); err != nil {
				return err
			}
			stockReturned = true
		}

		now := s.nowFunc().UTC()
		remaining := item.Quantity - qty
		if remaining == 0 {
			// Options cascade; split rows of a reopened tab's cancelled splits are the only other reference.
			if _, err := tx.ExecContext(ctx, `DELETE FROM payment_split_items psi USING payment_splits ps
				WHERE psi.store_id = $1 AND psi.order_item_id = $2
				  AND ps.store_id = psi.store_id AND ps.id = psi.split_id AND ps.status = 'cancelled'`, storeID, item.ID); err != nil {
				return fmt.Errorf("failed to drop cancelled split rows: %w", err)
			}
			if _, err := tx.ExecContext(ctx, `DELETE FROM order_items WHERE store_id = $1 AND id = $2`, storeID, item.ID); err != nil {
				return fmt.Errorf("failed to remove tab item: %w", err)
			}
			if _, err := tx.ExecContext(ctx, `UPDATE kds_tickets k SET status = 'cancelled', cancelled_at = $5, updated_at = $5
				WHERE k.store_id = $1 AND k.order_id = $2 AND k.round_id IS NOT DISTINCT FROM $3::uuid AND k.station = $4::text
				  AND k.status IN ('queued', 'preparing', 'ready')
				  AND NOT EXISTS (SELECT 1 FROM order_items oi WHERE oi.store_id = k.store_id AND oi.order_id = k.order_id
				      AND oi.round_id IS NOT DISTINCT FROM k.round_id AND oi.snapshot_prep_station = k.station)`,
				storeID, order.ID, item.RoundID, item.PrepStation, now); err != nil {
				return fmt.Errorf("failed to cancel empty kds ticket: %w", err)
			}
		} else if _, err := tx.ExecContext(ctx, `UPDATE order_items SET quantity = $3::int, subtotal_amount = snapshot_price * $3::int
			WHERE store_id = $1 AND id = $2`, storeID, item.ID, remaining); err != nil {
			return fmt.Errorf("failed to shrink tab item: %w", err)
		}

		store, err := s.orderRepo.GetStoreForOrder(ctx, tx, storeID)
		if err != nil {
			return err
		}
		if err := RecalculateOrderTotals(ctx, tx, store, order); err != nil {
			return fmt.Errorf("failed to recalculate tab totals: %w", err)
		}

		payload, _ := json.Marshal(map[string]any{
			"order_id": order.ID, "order_number": order.OrderNumber, "table_number": order.TableNumber,
			"order_item_id": item.ID, "voided_quantity": qty, "remaining_quantity": remaining, "order_total": order.TotalAmount,
		})
		if err := s.orderRepo.InsertOutboxEvent(ctx, tx, &domain.OutboxEvent{
			ID: uuid.New(), StoreID: storeID, EventType: "order.item_voided", AggregateType: "order",
			AggregateID: order.ID.String(), Payload: string(payload), CreatedAt: now,
		}); err != nil {
			return err
		}

		actorID := actor.Staff.UserID
		auditPayload, _ := json.Marshal(map[string]any{
			"order_id": order.ID, "order_item_id": item.ID, "item_name": item.Name, "unit_price": item.Price,
			"voided_quantity": qty, "remaining_quantity": remaining, "voided_amount": amount,
			"stock_returned": stockReturned, "owner_override_by": ownerOverrideID, "approval_id": claim.usedID(),
		})
		if err := postgres.InsertAuditLog(ctx, tx, &domain.AuditLog{
			StoreID: storeID, ActorID: &actorID, ActorName: actor.Staff.FullName, Action: "VOID_ORDER_ITEM",
			Reason: reason, ReferenceID: order.OrderNumber, Metadata: string(auditPayload), CreatedAt: now,
		}); err != nil {
			return err
		}
		// Return the tab as it now stands: the loaded order still lists the line before the void.
		fresh, _, err := s.orderRepo.GetOrderByIDReadOnly(ctx, tx, storeID, order.ID)
		if err != nil {
			return err
		}
		result = &VoidTabItemResult{Order: fresh, RemainingQuantity: remaining, StockReturned: stockReturned}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func orderHasItem(order *domain.Order, itemID uuid.UUID) bool {
	for _, it := range order.Items {
		if it.ID == itemID {
			return true
		}
	}
	return false
}
