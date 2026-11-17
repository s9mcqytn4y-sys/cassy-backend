package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/pkg/database"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/pkg/rbac"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/repository/postgres"
)

// DivideOrder lets a cashier split a pending pay-first POS order into server-priced shares
// (equal or by item) before anyone pays (Release One PR 9). Each share is then settled through
// InitiatePayment with its split_id; the order reaches the kitchen only when the last share is paid,
// because settlement creates KDS tickets on full payment only. Same rules as the guest split:
// refused once money or a payment attempt is attached, after expiry, or when already divided.
// Requires order.pos.create; the order must belong to the caller's store (tenant transaction).
func (s *PaymentService) DivideOrder(ctx context.Context, actor *AuthenticatedStaff, orderID uuid.UUID, in CloseTabInput) (*CloseTabResult, error) {
	if actor == nil {
		return nil, ErrInvalidSession
	}
	if !actor.Can(rbac.OrderPOSCreate) {
		return nil, ErrForbidden
	}
	if err := validateSplitInput(&in); err != nil {
		return nil, err
	}
	var out *CloseTabResult
	err := database.WithTenantTx(ctx, s.db, actor.Staff.Store.ID, func(tx *sqlx.Tx) error {
		order, splits, err := dividePayFirstOrderTx(ctx, tx, s.orderRepo, s.paymentRepo, postgres.NewTabRepository(),
			actor.Staff.Store.ID, orderID, in, s.nowFunc().UTC())
		if err != nil {
			return err
		}
		out = &CloseTabResult{Order: order, Splits: splits}
		return nil
	})
	return out, err
}
