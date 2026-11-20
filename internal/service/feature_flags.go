package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/domain"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/pkg/database"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/pkg/rbac"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/repository/postgres"
)

// Per-tenant feature flags (W4.2). Defaults keep today's behaviour: existing features stay on, new ones
// (call_waiter) and ones needing setup (whatsapp) start off. Keys mirror chk_store_feature_flags_key.
var featureDefaults = map[string]bool{
	"call_waiter":   false,
	"kds":           true,
	"queue_display": true,
	"dual_screen":   true,
	"open_bill":     true,
	"split_bill":    true,
	"stock":         true,
	"whatsapp":      false,
}

// FeatureCallWaiter gates guest service requests (W4.3).
const (
	FeatureCallWaiter = "call_waiter"
	FeatureSplitBill  = "split_bill"
)

// ErrSplitBillDisabled means the owner turned split_bill off; no new shares may be created.
var ErrSplitBillDisabled = errors.New("split bill is disabled for this store")

// requireSplitBill fails closed when the store has turned split_bill off.
func requireSplitBill(ctx context.Context, tx *sqlx.Tx, storeID uuid.UUID) error {
	flags, err := loadFeatureFlagsTx(ctx, tx, storeID)
	if err != nil {
		return err
	}
	if !flags[FeatureSplitBill] {
		return ErrSplitBillDisabled
	}
	return nil
}

// FeatureService reads and changes the flags of the caller's store.
type FeatureService struct {
	db      *sqlx.DB
	nowFunc func() time.Time
}

func NewFeatureService(db *sqlx.DB) *FeatureService {
	return &FeatureService{db: db, nowFunc: time.Now}
}

// loadFeatureFlagsTx returns every known flag with the store override applied; call it inside the
// tenant transaction of storeID.
func loadFeatureFlagsTx(ctx context.Context, tx *sqlx.Tx, storeID uuid.UUID) (map[string]bool, error) {
	out := make(map[string]bool, len(featureDefaults))
	for k, v := range featureDefaults {
		out[k] = v
	}
	var rows []struct {
		Key     string `db:"flag_key"`
		Enabled bool   `db:"enabled"`
	}
	if err := tx.SelectContext(ctx, &rows, `SELECT flag_key, enabled FROM store_feature_flags WHERE store_id = $1`, storeID); err != nil {
		return nil, fmt.Errorf("failed to load feature flags: %w", err)
	}
	for _, r := range rows {
		out[r.Key] = r.Enabled
	}
	return out, nil
}

// List returns the flags of the caller's store; any signed-in staff may read them.
func (s *FeatureService) List(ctx context.Context, actor *AuthenticatedStaff) (map[string]bool, error) {
	if actor == nil {
		return nil, ErrInvalidSession
	}
	var out map[string]bool
	err := database.WithTenantTx(ctx, s.db, actor.Staff.Store.ID, func(tx *sqlx.Tx) error {
		var err error
		out, err = loadFeatureFlagsTx(ctx, tx, actor.Staff.Store.ID)
		return err
	})
	return out, err
}

// Set turns one flag on or off (store.settings.manage, audited) and returns all flags.
func (s *FeatureService) Set(ctx context.Context, actor *AuthenticatedStaff, key string, enabled bool) (map[string]bool, error) {
	if actor == nil {
		return nil, ErrInvalidSession
	}
	if !actor.Can(rbac.StoreSettingsManage) {
		return nil, ErrForbidden
	}
	if _, ok := featureDefaults[key]; !ok {
		f := fieldErrors{}
		f.add("key", "fitur tidak dikenal")
		return nil, f.err()
	}
	storeID := actor.Staff.Store.ID
	var out map[string]bool
	err := database.WithTenantTx(ctx, s.db, storeID, func(tx *sqlx.Tx) error {
		before, err := loadFeatureFlagsTx(ctx, tx, storeID)
		if err != nil {
			return err
		}
		now := s.nowFunc().UTC()
		if _, err := tx.ExecContext(ctx, `INSERT INTO store_feature_flags (store_id, flag_key, enabled, updated_by, updated_at)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (store_id, flag_key) DO UPDATE SET enabled = EXCLUDED.enabled, updated_by = EXCLUDED.updated_by, updated_at = EXCLUDED.updated_at`,
			storeID, key, enabled, actor.Staff.UserID, now); err != nil {
			return fmt.Errorf("failed to set feature flag: %w", err)
		}
		if before[key] != enabled {
			payload, _ := json.Marshal(map[string]any{"key": key, "before": before[key], "after": enabled})
			actorID := actor.Staff.UserID
			if err := postgres.InsertAuditLog(ctx, tx, &domain.AuditLog{
				StoreID: storeID, ActorID: &actorID, ActorName: actor.Staff.FullName, Action: "FEATURE_FLAG_CHANGED",
				Reason: "Fitur toko diubah", ReferenceID: storeID.String(), Metadata: string(payload), CreatedAt: now,
			}); err != nil {
				return err
			}
		}
		before[key] = enabled
		out = before
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
