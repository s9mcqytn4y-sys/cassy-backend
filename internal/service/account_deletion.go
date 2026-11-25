package service

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/domain"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/pkg/database"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/pkg/mailer"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/pkg/metrics"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/repository/postgres"
)

// Public account deletion for /hapus-akun (FL-Q9). The store owner proves control of the owner
// email with a 6-digit code, then a tenant_clearance_requests row (channel web) starts the 30-day
// grace period. The worker anonymizes personal data once the grace period ends; orders, payments,
// shifts, reports and invoices stay for the legal retention period.

const (
	deletionOTPTTL         = 10 * time.Minute
	deletionOTPCooldown    = time.Minute
	deletionOTPHourlyCap   = 5
	deletionOTPMaxAttempts = 5
	deletionDefaultReason  = "Permintaan hapus akun lewat halaman web /hapus-akun"
	clearedName            = "Akun dihapus"
)

var (
	// ErrDeletionOTPInvalid covers a wrong, used or expired code and an unknown store or email, so
	// the public endpoints never reveal which stores or emails exist.
	ErrDeletionOTPInvalid = errors.New("kode OTP salah atau kedaluwarsa")
	// ErrDeletionNotPending means there is no request in its grace period to cancel.
	ErrDeletionNotPending = errors.New("tidak ada permintaan hapus akun yang sedang berjalan")
)

// DeletionPendingError is returned when a request already runs its grace period.
type DeletionPendingError struct {
	Request *domain.TenantClearanceRequest
}

func (e *DeletionPendingError) Error() string {
	return "permintaan hapus akun sudah berjalan pada masa tenggang"
}

// RequestAccountDeletionOTP emails a code to the store owner. It answers nil for unknown stores,
// unknown or non-owner emails and rate-limited requests alike; the caller always responds 202.
// devPreview holds the code only in development and test.
func (s *GovernanceService) RequestAccountDeletionOTP(ctx context.Context, storeSlug, email string) (devPreview string, err error) {
	email, err = NormalizeStaffEmail(email)
	if err != nil {
		return "", nil
	}
	storeID, ok, err := s.resolveDeletionStore(ctx, storeSlug)
	if err != nil || !ok {
		return "", err
	}
	now := s.nowFunc().UTC()
	code, err := generateSixDigitOTP()
	if err != nil {
		return "", err
	}
	var owner *domain.User
	var storeName string
	err = database.WithTenantTx(ctx, s.db, storeID, func(tx *sqlx.Tx) error {
		u, err := deletionOwnerTx(ctx, tx, s.authRepo, storeID, email)
		if err != nil || u == nil {
			return err
		}
		var recent struct {
			Hour int        `db:"hour"`
			Last *time.Time `db:"last"`
		}
		if err := tx.GetContext(ctx, &recent, `SELECT COUNT(*) AS hour, MAX(created_at) AS last FROM account_deletion_challenges
			WHERE store_id = $1 AND user_id = $2 AND created_at > $3`, storeID, u.ID, now.Add(-time.Hour)); err != nil {
			return err
		}
		if recent.Hour >= deletionOTPHourlyCap || (recent.Last != nil && now.Sub(*recent.Last) < deletionOTPCooldown) {
			return nil
		}
		id := uuid.New()
		if _, err := tx.ExecContext(ctx, `INSERT INTO account_deletion_challenges (id, store_id, user_id, code_hash, expires_at, created_at)
			VALUES ($1, $2, $3, $4, $5, $6)`, id, storeID, u.ID, hashOTP(id, code), now.Add(deletionOTPTTL), now); err != nil {
			return fmt.Errorf("failed to store deletion otp: %w", err)
		}
		if err := tx.GetContext(ctx, &storeName, `SELECT name FROM stores WHERE id = $1`, storeID); err != nil {
			return err
		}
		owner = u
		return s.writeGovernanceAuditTx(ctx, tx, storeID, &u.ID, "tenant_owner", "CLEARANCE_OTP_ISSUED",
			"Kode OTP hapus akun dikirim ke email pemilik", map[string]any{"challenge_id": id})
	})
	if err != nil || owner == nil {
		return "", err
	}
	s.sendDeletionOTPEmail(ctx, owner, storeName, strings.ToLower(strings.TrimSpace(storeSlug)), code)
	if s.appEnv == "development" || s.appEnv == "test" {
		return code, nil
	}
	return "", nil
}

// ConfirmAccountDeletion verifies the code and opens the 30-day grace period. A request already in
// its grace period comes back as *DeletionPendingError; open shifts or orders as *ConflictError.
func (s *GovernanceService) ConfirmAccountDeletion(ctx context.Context, storeSlug, email, code, reason string) (*domain.TenantClearanceRequest, error) {
	reason = strings.TrimSpace(reason)
	if len([]rune(reason)) > 500 {
		f := fieldErrors{}
		f.add("reason", "maksimal 500 karakter")
		return nil, f.err()
	}
	if len([]rune(reason)) < 5 {
		reason = deletionDefaultReason
	}
	var out *domain.TenantClearanceRequest
	var pending *domain.TenantClearanceRequest
	err := s.withVerifiedDeletionOwner(ctx, storeSlug, email, code, func(tx *sqlx.Tx, storeID uuid.UUID, owner *domain.User, now time.Time) error {
		existing, err := pendingClearanceTx(ctx, tx, storeID)
		if err != nil {
			return err
		}
		if existing != nil {
			pending = existing
			return nil
		}
		out, err = s.createClearanceTx(ctx, tx, storeID, owner.ID, reason, "web", now)
		return err
	})
	if err != nil {
		return nil, err
	}
	if pending != nil {
		return nil, &DeletionPendingError{Request: pending}
	}
	return out, nil
}

// CancelAccountDeletion verifies the code and cancels the request in its grace period.
func (s *GovernanceService) CancelAccountDeletion(ctx context.Context, storeSlug, email, code string) (*domain.TenantClearanceRequest, error) {
	var out *domain.TenantClearanceRequest
	err := s.withVerifiedDeletionOwner(ctx, storeSlug, email, code, func(tx *sqlx.Tx, storeID uuid.UUID, owner *domain.User, now time.Time) error {
		existing, err := pendingClearanceTx(ctx, tx, storeID)
		if err != nil || existing == nil {
			return err
		}
		out = &domain.TenantClearanceRequest{}
		if err := tx.GetContext(ctx, out, `
			UPDATE tenant_clearance_requests SET status = 'cancelled', updated_at = $3
			WHERE store_id = $1 AND id = $2 AND status = 'cooling_off'
			RETURNING id, store_id, requested_by_user_id, reason, status, cooling_off_ends_at, ledger_retention_until, executed_at, channel, created_at, updated_at
		`, storeID, existing.ID, now); err != nil {
			return err
		}
		return s.writeGovernanceAuditTx(ctx, tx, storeID, &owner.ID, "tenant_owner", "CLEARANCE_CANCELLED",
			"Permintaan hapus akun dibatalkan lewat halaman web", map[string]any{"clearance_id": out.ID, "channel": "web"})
	})
	if err != nil {
		return nil, err
	}
	if out == nil {
		return nil, ErrDeletionNotPending
	}
	return out, nil
}

// withVerifiedDeletionOwner spends one attempt on the newest code of the owner and runs fn in the
// same tenant transaction when it matches. A wrong code still commits so the attempt counts.
func (s *GovernanceService) withVerifiedDeletionOwner(ctx context.Context, storeSlug, email, code string,
	fn func(tx *sqlx.Tx, storeID uuid.UUID, owner *domain.User, now time.Time) error) error {
	code = strings.TrimSpace(code)
	email, err := NormalizeStaffEmail(email)
	if err != nil || !otpSixDigitsPattern.MatchString(code) {
		return ErrDeletionOTPInvalid
	}
	storeID, ok, err := s.resolveDeletionStore(ctx, storeSlug)
	if err != nil {
		return err
	}
	if !ok {
		return ErrDeletionOTPInvalid
	}
	now := s.nowFunc().UTC()
	verified := false
	err = database.WithTenantTx(ctx, s.db, storeID, func(tx *sqlx.Tx) error {
		owner, err := deletionOwnerTx(ctx, tx, s.authRepo, storeID, email)
		if err != nil || owner == nil {
			return err
		}
		var c otpChallenge
		err = tx.GetContext(ctx, &c, `SELECT id, code_hash, attempts, expires_at, consumed_at FROM account_deletion_challenges
			WHERE store_id = $1 AND user_id = $2 ORDER BY created_at DESC LIMIT 1 FOR UPDATE`, storeID, owner.ID)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if c.ConsumedAt != nil || !now.Before(c.ExpiresAt) || c.Attempts >= deletionOTPMaxAttempts {
			return nil
		}
		match := subtle.ConstantTimeCompare([]byte(hashOTP(c.ID, code)), []byte(c.CodeHash)) == 1
		if !match {
			_, err := tx.ExecContext(ctx, `UPDATE account_deletion_challenges SET attempts = attempts + 1 WHERE store_id = $1 AND id = $2`, storeID, c.ID)
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE account_deletion_challenges SET attempts = attempts + 1, consumed_at = $3
			WHERE store_id = $1 AND id = $2`, storeID, c.ID, now); err != nil {
			return err
		}
		verified = true
		return fn(tx, storeID, owner, now)
	})
	if err != nil {
		return err
	}
	if !verified {
		return ErrDeletionOTPInvalid
	}
	return nil
}

// resolveDeletionStore maps a slug to an active store; ok is false for unknown or inactive stores.
func (s *GovernanceService) resolveDeletionStore(ctx context.Context, storeSlug string) (uuid.UUID, bool, error) {
	storeID, active, err := s.authRepo.ResolveStoreForLogin(ctx, s.db, strings.ToLower(strings.TrimSpace(storeSlug)))
	if errors.Is(err, postgres.ErrStoreNotFound) {
		return uuid.Nil, false, nil
	}
	if err != nil {
		return uuid.Nil, false, err
	}
	return storeID, active, nil
}

// deletionOwnerTx returns the active owner with this email, or nil for anyone else.
func deletionOwnerTx(ctx context.Context, tx *sqlx.Tx, repo *postgres.AuthRepository, storeID uuid.UUID, email string) (*domain.User, error) {
	u, err := repo.GetUserByEmailForUpdate(ctx, tx, storeID, email)
	if errors.Is(err, postgres.ErrUserNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !u.IsActive || u.Role != domain.RoleOwner {
		return nil, nil
	}
	return u, nil
}

func pendingClearanceTx(ctx context.Context, tx *sqlx.Tx, storeID uuid.UUID) (*domain.TenantClearanceRequest, error) {
	var r domain.TenantClearanceRequest
	err := tx.GetContext(ctx, &r, `
		SELECT id, store_id, requested_by_user_id, reason, status, cooling_off_ends_at, ledger_retention_until, executed_at, channel, created_at, updated_at
		FROM tenant_clearance_requests WHERE store_id = $1 AND status = 'cooling_off' FOR UPDATE`, storeID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *GovernanceService) sendDeletionOTPEmail(ctx context.Context, owner *domain.User, storeName, storeSlug, code string) {
	if s.mailSender == nil {
		return
	}
	rendered, err := mailer.Render(mailer.TemplateAccountDeletionOTP, mailer.Data{
		StoreName:     storeName,
		StoreSlug:     storeSlug,
		RecipientName: owner.FullName,
		OTPCode:       code,
		ExpiresIn:     "10 menit",
		Year:          s.nowFunc().UTC().Year(),
	})
	if err != nil {
		log.Printf("[ACCOUNT DELETION] template render error slug=%s: %v", storeSlug, err)
		return
	}
	sendCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := s.mailSender.Send(sendCtx, mailer.Message{
		To: owner.Email, ToName: owner.FullName, Subject: rendered.Subject, HTML: rendered.HTML, Text: rendered.Text,
	}); err != nil {
		log.Printf("[ACCOUNT DELETION] send warning slug=%s: %v", storeSlug, err)
	}
}

// anonymizeStoreTx overwrites personal data of staff, customers, guests and the owner, and
// deactivates the store, its devices and display tokens. Orders, payments, shifts, reports and
// invoices keep their financial content. Every statement is safe to run twice.
func anonymizeStoreTx(ctx context.Context, tx *sqlx.Tx, storeID, actorUserID uuid.UUID, now time.Time) (map[string]int64, error) {
	steps := []struct {
		name  string
		query string
	}{
		{"users", `UPDATE users SET full_name = '` + clearedName + `', email = 'deleted-' || id::text || '@cleared.invalid',
			phone = NULL, pin_hash = '!cleared', is_active = false, locked_until = NULL, updated_at = $2
			WHERE store_id = $1 AND email NOT LIKE 'deleted-%@cleared.invalid'`},
		{"auth_sessions", `UPDATE auth_sessions SET revoked_at = COALESCE(revoked_at, $2),
			revoked_reason = COALESCE(revoked_reason, 'user_deactivated'), user_agent = '', client_ip = ''
			WHERE store_id = $1 AND (revoked_at IS NULL OR user_agent <> '' OR client_ip <> '')`},
		{"account_tokens", `UPDATE account_tokens SET used_at = $2 WHERE store_id = $1 AND used_at IS NULL`},
		{"customers", `WITH n AS (
				SELECT id, row_number() OVER (ORDER BY created_at, id) AS rn FROM customers
				WHERE store_id = $1 AND phone NOT LIKE '+999%')
			UPDATE customers c SET name = NULL, phone = '+999' || lpad(n.rn::text, 10, '0'), phone_verified_at = NULL, updated_at = $2
			FROM n WHERE c.store_id = $1 AND c.id = n.id`},
		{"business_customers", `UPDATE business_customers SET email = '', phone = '' WHERE store_id = $1 AND (email <> '' OR phone <> '')`},
		{"orders", `UPDATE orders SET customer_name = '', customer_phone = NULL
			WHERE store_id = $1 AND (customer_name <> '' OR customer_phone IS NOT NULL)`},
		{"otp_challenges", `UPDATE otp_challenges SET phone = '+99900000000' WHERE store_id = $1 AND phone <> '+99900000000'`},
		{"account_deletion_challenges", `DELETE FROM account_deletion_challenges WHERE store_id = $1`},
		{"email_outbox", `UPDATE email_outbox SET to_email = 'deleted@cleared.invalid', to_name = '', reply_to = '',
			subject = 'cleared', html_body = '', text_body = '',
			status = CASE WHEN status IN ('pending', 'sending') THEN 'failed' ELSE status END,
			last_error = CASE WHEN status IN ('pending', 'sending') THEN 'account_cleared' ELSE last_error END
			WHERE store_id = $1 AND to_email <> 'deleted@cleared.invalid'`},
		{"message_outbox", `UPDATE message_outbox SET to_phone = '+99900000000', body = '',
			status = CASE WHEN status IN ('pending', 'sending') THEN 'failed' ELSE status END,
			last_error = CASE WHEN status IN ('pending', 'sending') THEN 'account_cleared' ELSE last_error END
			WHERE store_id = $1 AND to_phone <> '+99900000000'`},
		{"store_cms_configs", `UPDATE store_cms_configs SET public_contact_email = '', public_contact_phone = ''
			WHERE store_id = $1 AND (public_contact_email <> '' OR public_contact_phone <> '')`},
		{"store_notification_configs", `UPDATE store_notification_configs SET smtp_username = '', smtp_from_email = '', smtp_from_name = ''
			WHERE store_id = $1 AND (smtp_username <> '' OR smtp_from_email <> '' OR smtp_from_name <> '')`},
		{"tenant_onboarding_applications", `UPDATE tenant_onboarding_applications SET owner_name = '` + clearedName + `',
			owner_email = 'deleted-' || id::text || '@cleared.invalid', owner_phone = '', updated_at = $2
			WHERE provisioned_store_id = $1 AND owner_email NOT LIKE 'deleted-%@cleared.invalid'`},
		{"devices", `UPDATE devices SET revoked_at = $2, revoked_by = $3, updated_at = $2 WHERE store_id = $1 AND revoked_at IS NULL`},
		{"display_tokens", `UPDATE display_tokens SET revoked_at = $2 WHERE store_id = $1 AND revoked_at IS NULL`},
		{"stores", `UPDATE stores SET phone = NULL, is_active = false WHERE id = $1 AND (phone IS NOT NULL OR is_active)`},
	}
	counts := make(map[string]int64, len(steps))
	for _, st := range steps {
		args := []any{storeID}
		if strings.Contains(st.query, "$2") {
			args = append(args, now)
		}
		if strings.Contains(st.query, "$3") {
			args = append(args, actorUserID)
		}
		res, err := tx.ExecContext(ctx, st.query, args...)
		if err != nil {
			return nil, fmt.Errorf("anonymize %s: %w", st.name, err)
		}
		n, _ := res.RowsAffected()
		counts[st.name] = n
	}
	return counts, nil
}

// ExecuteDueClearances anonymizes every store whose grace period ended. A request cancelled or
// executed in the meantime is skipped, so overlapping runs are harmless.
func (s *GovernanceService) ExecuteDueClearances(ctx context.Context) (int, error) {
	var due []struct {
		StoreID     uuid.UUID `db:"store_id"`
		ClearanceID uuid.UUID `db:"clearance_id"`
	}
	if err := s.db.SelectContext(ctx, &due, `SELECT store_id, clearance_id FROM list_due_account_clearances()`); err != nil {
		return 0, fmt.Errorf("failed to list due clearances: %w", err)
	}
	done := 0
	for _, d := range due {
		if ctx.Err() != nil {
			return done, ctx.Err()
		}
		now := s.nowFunc().UTC()
		err := database.WithTenantTx(ctx, s.db, d.StoreID, func(tx *sqlx.Tx) error {
			_, err := s.executeClearanceTx(ctx, tx, d.StoreID, d.ClearanceID, "system", now)
			return err
		})
		switch {
		case err == nil:
			done++
		case errors.Is(err, ErrGovernanceNotFound):
		default:
			log.Printf("[ACCOUNT DELETION] store_id=%s clearance_id=%s error: %v", d.StoreID, d.ClearanceID, err)
		}
	}
	return done, nil
}

// StartClearanceWorker runs ExecuteDueClearances until ctx is cancelled.
func (s *GovernanceService) StartClearanceWorker(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := s.ExecuteDueClearances(ctx); err != nil {
				if !errors.Is(err, context.Canceled) {
					log.Printf("[ACCOUNT DELETION] cycle error: %v", err)
				}
			} else {
				metrics.WorkerSucceeded("account_clearance")
			}
		}
	}
}
