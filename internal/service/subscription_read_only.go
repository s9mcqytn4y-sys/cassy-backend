package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/pkg/database"
)

// trialLengthSQL is the free trial every new store starts on (FL-Q3: the only free tier).
const trialLengthSQL = `INTERVAL '14 days'`

// SubscriptionReadOnly reports whether the store's trial has ended without a paid plan. Such a store
// is read-only: staff can still sign in, read data, and pay the subscription, but every other write
// is refused. Nothing is deleted. The trial end is the KYB period end when a profile exists, else the
// store creation time plus 14 days. A store without a registry row returns false; quota checks fail
// closed for it on their own.
func (s *AuthService) SubscriptionReadOnly(ctx context.Context, storeID uuid.UUID) (bool, error) {
	var readOnly bool
	err := database.WithTenantTx(ctx, s.db, storeID, func(tx *sqlx.Tx) error {
		var err error
		readOnly, err = subscriptionReadOnlyTx(ctx, tx, storeID, s.nowFunc().UTC())
		return err
	})
	return readOnly, err
}

// subscriptionReadOnlyTx is SubscriptionReadOnly inside an existing tenant transaction, so guest QR
// ordering can refuse new orders from a read-only store in the same transaction that places them.
func subscriptionReadOnlyTx(ctx context.Context, tx *sqlx.Tx, storeID uuid.UUID, nowUTC time.Time) (bool, error) {
	var readOnly bool
	err := tx.GetContext(ctx, &readOnly, `SELECT r.subscription_status = 'trialing'
			AND COALESCE(k.current_period_ends_at, st.created_at + `+trialLengthSQL+`) < $2
		FROM platform_tenant_registry r
		JOIN stores st ON st.id = r.store_id
		LEFT JOIN store_kyb_profiles k ON k.store_id = r.store_id
		WHERE r.store_id = $1`, storeID, nowUTC)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to resolve subscription state: %w", err)
	}
	return readOnly, nil
}
