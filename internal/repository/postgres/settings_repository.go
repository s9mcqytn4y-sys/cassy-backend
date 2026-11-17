package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/domain"
)

type SettingsRepository struct{}

func NewSettingsRepository() *SettingsRepository {
	return &SettingsRepository{}
}

const storeSettingsColumns = `store_id, void_refund_owner_threshold, cashier_max_discount_bps, cash_variance_tolerance,
	cashier_handles_bar, table_idle_close_minutes, qr_ordering_paused, static_qris_fallback_mode,
	refund_window_cashier_days, refund_window_owner_days, cash_rounding_step,
	outlet_daily_expense_limit, default_opening_float, cashier_sees_expected_cash, gateway_partial_refunds,
	table_time_limit_minutes, table_timer_warning_minutes, qr_service_mode,
	offline_max_hours, offline_max_orders, stock_approval_limit`

// GetStoreSettings returns the store's settings, or the defaults when no row exists yet.
func (r *SettingsRepository) GetStoreSettings(ctx context.Context, tx *sqlx.Tx, storeID uuid.UUID) (domain.StoreSettings, error) {
	if tx == nil {
		return domain.StoreSettings{}, ErrNilTransaction
	}
	var s domain.StoreSettings
	err := tx.GetContext(ctx, &s, `SELECT `+storeSettingsColumns+` FROM store_settings WHERE store_id = $1`, storeID)
	if errors.Is(err, sql.ErrNoRows) {
		s = domain.DefaultStoreSettings(storeID)
	} else if err != nil {
		return domain.StoreSettings{}, fmt.Errorf("failed to load store settings: %w", err)
	}
	var storeMeta struct {
		MethodsJSON      string `db:"enabled_payment_methods"`
		ValidationStatus string `db:"validation_status"`
	}
	metaErr := tx.GetContext(ctx, &storeMeta, `
		SELECT st.enabled_payment_methods::text AS enabled_payment_methods,
			COALESCE((SELECT spc.validation_status FROM store_payment_credentials spc WHERE spc.store_id = st.id), 'unverified') AS validation_status
		FROM stores st
		WHERE st.id = $1
	`, storeID)
	if metaErr == nil {
		_ = json.Unmarshal([]byte(storeMeta.MethodsJSON), &s.EnabledPaymentMethods)
		s.MidtransValidationStatus = storeMeta.ValidationStatus
	} else if !errors.Is(metaErr, sql.ErrNoRows) {
		return domain.StoreSettings{}, fmt.Errorf("failed to load store payment methods: %w", metaErr)
	}
	return s, nil
}

// UpsertStoreSettings writes the full settings row (the caller merges a patch onto the current values).
func (r *SettingsRepository) UpsertStoreSettings(ctx context.Context, tx *sqlx.Tx, s domain.StoreSettings, updatedBy uuid.UUID) error {
	if tx == nil {
		return ErrNilTransaction
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO store_settings (`+storeSettingsColumns+`, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22)
		ON CONFLICT (store_id) DO UPDATE SET
			void_refund_owner_threshold = EXCLUDED.void_refund_owner_threshold,
			cashier_max_discount_bps = EXCLUDED.cashier_max_discount_bps,
			cash_variance_tolerance = EXCLUDED.cash_variance_tolerance,
			cashier_handles_bar = EXCLUDED.cashier_handles_bar,
			table_idle_close_minutes = EXCLUDED.table_idle_close_minutes,
			qr_ordering_paused = EXCLUDED.qr_ordering_paused,
			static_qris_fallback_mode = EXCLUDED.static_qris_fallback_mode,
			refund_window_cashier_days = EXCLUDED.refund_window_cashier_days,
			refund_window_owner_days = EXCLUDED.refund_window_owner_days,
			cash_rounding_step = EXCLUDED.cash_rounding_step,
			outlet_daily_expense_limit = EXCLUDED.outlet_daily_expense_limit,
			default_opening_float = EXCLUDED.default_opening_float,
			cashier_sees_expected_cash = EXCLUDED.cashier_sees_expected_cash,
			gateway_partial_refunds = EXCLUDED.gateway_partial_refunds,
			table_time_limit_minutes = EXCLUDED.table_time_limit_minutes,
			table_timer_warning_minutes = EXCLUDED.table_timer_warning_minutes,
			qr_service_mode = EXCLUDED.qr_service_mode,
			offline_max_hours = EXCLUDED.offline_max_hours,
			offline_max_orders = EXCLUDED.offline_max_orders,
			stock_approval_limit = EXCLUDED.stock_approval_limit,
			updated_by = EXCLUDED.updated_by
	`, s.StoreID, s.VoidRefundOwnerThreshold, s.CashierMaxDiscountBPS, s.CashVarianceTolerance,
		s.CashierHandlesBar, s.TableIdleCloseMinutes, s.QROrderingPaused, s.StaticQRISFallbackMode,
		s.RefundWindowCashierDays, s.RefundWindowOwnerDays, s.CashRoundingStep,
		s.OutletDailyExpenseLimit, s.DefaultOpeningFloat, s.CashierSeesExpectedCash, s.GatewayPartialRefunds,
		s.TableTimeLimitMinutes, s.TableTimerWarningMinutes, s.QRServiceMode,
		s.OfflineMaxHours, s.OfflineMaxOrders, s.StockApprovalLimit, updatedBy)
	if err != nil {
		return fmt.Errorf("failed to save store settings: %w", err)
	}
	return nil
}

// UpdateStorePaymentMethods persists allowed store payment methods and clears stale store caches.
func (r *SettingsRepository) UpdateStorePaymentMethods(ctx context.Context, tx *sqlx.Tx, storeID uuid.UUID, methods []string) error {
	if tx == nil {
		return ErrNilTransaction
	}
	encoded, err := json.Marshal(methods)
	if err != nil {
		return fmt.Errorf("failed to encode enabled payment methods: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE stores SET enabled_payment_methods = $2::jsonb, updated_at = NOW()
		WHERE id = $1
	`, storeID, string(encoded)); err != nil {
		return fmt.Errorf("failed to update store payment methods: %w", err)
	}
	return nil
}

// HasVerifiedMidtransCredential reports whether a credential row exists and its validation status is "valid".
func (r *SettingsRepository) MidtransCredentialStatus(ctx context.Context, tx *sqlx.Tx, storeID uuid.UUID) (exists bool, valid bool, err error) {
	var row struct {
		ValidationStatus string `db:"validation_status"`
		IsEnabled        bool   `db:"is_enabled"`
	}
	qErr := tx.GetContext(ctx, &row, `
		SELECT validation_status, is_enabled
		FROM store_payment_credentials
		WHERE store_id = $1
	`, storeID)
	if errors.Is(qErr, sql.ErrNoRows) {
		return false, false, nil
	}
	if qErr != nil {
		return false, false, fmt.Errorf("failed to inspect midtrans credential status: %w", qErr)
	}
	return true, row.ValidationStatus == "valid" && row.IsEnabled, nil
}
