package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/config"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/handler"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/pkg/database"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/service"
)

// TestIntegrationAccountDeletion covers FL-Q9: the public /hapus-akun flow (no enumeration, OTP
// cooldown, wrong codes rejected, 30-day grace request, duplicate request 409, cancel) and the worker
// that anonymizes personal data once the grace period ends while keeping financial rows.
func TestIntegrationAccountDeletion(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_APP_DATABASE_URL"))
	migratorDSN := strings.TrimSpace(os.Getenv("TEST_MIGRATOR_DATABASE_URL"))
	if dsn == "" || migratorDSN == "" {
		t.Skip("TEST_APP_DATABASE_URL / TEST_MIGRATOR_DATABASE_URL not set; skipping account deletion integration test")
	}
	db, err := database.NewPostgresDB(&config.Config{DatabaseURL: dsn})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	migratorDB, err := database.NewPostgresDB(&config.Config{DatabaseURL: migratorDSN})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = migratorDB.Close() })
	ctx := context.Background()

	suffix := uuid.NewString()[:8]
	slug := "hapus-" + suffix
	ownerEmail := "owner.hapus." + suffix + "@cassy.local"
	storeID, ownerID, err := service.BootstrapStore(ctx, migratorDB, service.BootstrapStoreInput{
		Slug: slug, StoreName: "Kafe Hapus", OwnerName: "Pemilik Hapus", OwnerEmail: ownerEmail, PIN: "482913",
	})
	if err != nil {
		t.Fatalf("bootstrap store: %v", err)
	}
	cashierEmail := "cashier.hapus." + suffix + "@cassy.local"
	mustExec := func(q string, args ...any) {
		t.Helper()
		if _, err := migratorDB.ExecContext(ctx, q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	mustExec(`INSERT INTO users (store_id, email, full_name, role, pin_hash, is_active, pin_set_at, phone)
		SELECT store_id, $2, 'Kasir Hapus', 'cashier', pin_hash, true, NOW(), '+6281234500011' FROM users WHERE id = $1`, ownerID, cashierEmail)
	mustExec(`INSERT INTO customers (store_id, phone, name, phone_verified_at) VALUES ($1, '+6281234500022', 'Pelanggan Setia', NOW())`, storeID)
	mustExec(`INSERT INTO devices (store_id, name, prefix, credential_hash, registered_by) VALUES ($1, 'Kasir 1', 'K', $2, $3)`,
		storeID, strings.Repeat("a", 64), ownerID)

	mail := &captureMailSender{}
	gov := service.NewGovernanceService(db, &config.Config{AppEnv: "test"}, service.WithGovernanceMailer(mail, ""))
	gh := handler.NewGovernanceHandler(gov, "")
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/otp", gh.RequestAccountDeletionOTP)
	r.POST("/delete", gh.ConfirmAccountDeletion)
	r.POST("/cancel", gh.CancelAccountDeletion)
	call := func(path string, body map[string]string) (int, map[string]any) {
		t.Helper()
		raw, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		var out map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		return w.Code, out
	}
	// newCode skips the 1-minute resend cooldown by aging earlier challenges, then asks for a code.
	newCode := func() string {
		t.Helper()
		mustExec(`UPDATE account_deletion_challenges SET created_at = created_at - INTERVAL '2 minutes', expires_at = expires_at - INTERVAL '2 minutes'
			WHERE store_id = $1`, storeID)
		code, out := call("/otp", map[string]string{"store_slug": slug, "email": ownerEmail})
		preview, _ := out["dev_otp_preview"].(string)
		if code != http.StatusAccepted || len(preview) != 6 {
			t.Fatalf("otp: %d %v", code, out)
		}
		return preview
	}

	t.Run("NoEnumeration", func(t *testing.T) {
		for _, body := range []map[string]string{
			{"store_slug": "tidak-ada-" + suffix, "email": ownerEmail},
			{"store_slug": slug, "email": cashierEmail},
			{"store_slug": slug, "email": "bukan-email"},
		} {
			code, out := call("/otp", body)
			if code != http.StatusAccepted || out["dev_otp_preview"] != nil {
				t.Fatalf("%v: want 202 without code, got %d %v", body, code, out)
			}
		}
		if len(mail.sent) != 0 {
			t.Fatalf("no email expected, got %d", len(mail.sent))
		}
	})

	var first string
	t.Run("CooldownAndWrongCode", func(t *testing.T) {
		first = newCode()
		if len(mail.sent) != 1 || mail.sent[0].To != ownerEmail || !strings.Contains(mail.sent[0].Text, first) {
			t.Fatalf("owner email with code expected, got %+v", mail.sent)
		}
		if code, out := call("/otp", map[string]string{"store_slug": slug, "email": ownerEmail}); code != http.StatusAccepted || out["dev_otp_preview"] != nil {
			t.Fatalf("second request inside cooldown must not issue a code: %d %v", code, out)
		}
		wrong := "000000"
		if first == wrong {
			wrong = "111111"
		}
		if code, out := call("/delete", map[string]string{"store_slug": slug, "email": ownerEmail, "otp_code": wrong}); code != http.StatusUnprocessableEntity {
			t.Fatalf("wrong code: %d %v", code, out)
		}
	})

	t.Run("RequestDuplicateCancel", func(t *testing.T) {
		code, out := call("/delete", map[string]string{"store_slug": slug, "email": ownerEmail, "otp_code": first})
		if code != http.StatusCreated {
			t.Fatalf("confirm: %d %v", code, out)
		}
		req := out["request"].(map[string]any)
		ends, _ := time.Parse(time.RFC3339Nano, req["cooling_off_ends_at"].(string))
		if req["channel"] != "web" || req["status"] != "cooling_off" || time.Until(ends) < 29*24*time.Hour {
			t.Fatalf("expected web request with 30-day grace, got %v", req)
		}
		if code, out := call("/delete", map[string]string{"store_slug": slug, "email": ownerEmail, "otp_code": first}); code != http.StatusUnprocessableEntity {
			t.Fatalf("used code must fail: %d %v", code, out)
		}
		if code, out := call("/delete", map[string]string{"store_slug": slug, "email": ownerEmail, "otp_code": newCode()}); code != http.StatusConflict || out["code"] != "ACCOUNT_DELETION_PENDING" {
			t.Fatalf("duplicate: %d %v", code, out)
		}
		if code, out := call("/cancel", map[string]string{"store_slug": slug, "email": ownerEmail, "otp_code": newCode()}); code != http.StatusOK || out["request"].(map[string]any)["status"] != "cancelled" {
			t.Fatalf("cancel: %d %v", code, out)
		}
		if code, out := call("/cancel", map[string]string{"store_slug": slug, "email": ownerEmail, "otp_code": newCode()}); code != http.StatusNotFound {
			t.Fatalf("cancel without pending request: %d %v", code, out)
		}
	})

	t.Run("WorkerAnonymizesAfterGrace", func(t *testing.T) {
		code, out := call("/delete", map[string]string{"store_slug": slug, "email": ownerEmail, "otp_code": newCode(), "reason": "Gerai tutup permanen"})
		if code != http.StatusCreated {
			t.Fatalf("confirm: %d %v", code, out)
		}
		if _, err := gov.ExecuteDueClearances(ctx); err != nil {
			t.Fatal(err)
		}
		var status string
		if err := migratorDB.GetContext(ctx, &status, `SELECT status FROM tenant_clearance_requests WHERE store_id = $1 ORDER BY created_at DESC LIMIT 1`, storeID); err != nil || status != "cooling_off" {
			t.Fatalf("request must wait for its grace period: status=%q err=%v", status, err)
		}
		mustExec(`UPDATE tenant_clearance_requests SET cooling_off_ends_at = NOW() - INTERVAL '1 minute' WHERE store_id = $1 AND status = 'cooling_off'`, storeID)
		if n, err := gov.ExecuteDueClearances(ctx); err != nil || n < 1 {
			t.Fatalf("worker run: n=%d err=%v", n, err)
		}
		var check struct {
			ActiveUsers   int    `db:"active_users"`
			PlainEmails   int    `db:"plain_emails"`
			Phones        int    `db:"phones"`
			CustomerPhone string `db:"customer_phone"`
			OpenDevices   int    `db:"open_devices"`
			StoreActive   bool   `db:"store_active"`
			Status        string `db:"status"`
			Audit         int    `db:"audit"`
		}
		if err := migratorDB.GetContext(ctx, &check, `SELECT
			(SELECT COUNT(*) FROM users WHERE store_id = $1 AND is_active) AS active_users,
			(SELECT COUNT(*) FROM users WHERE store_id = $1 AND email NOT LIKE 'deleted-%@cleared.invalid') AS plain_emails,
			(SELECT COUNT(*) FROM users WHERE store_id = $1 AND phone IS NOT NULL) AS phones,
			(SELECT phone FROM customers WHERE store_id = $1 LIMIT 1) AS customer_phone,
			(SELECT COUNT(*) FROM devices WHERE store_id = $1 AND revoked_at IS NULL) AS open_devices,
			(SELECT is_active FROM stores WHERE id = $1) AS store_active,
			(SELECT status FROM tenant_clearance_requests WHERE store_id = $1 ORDER BY created_at DESC LIMIT 1) AS status,
			(SELECT COUNT(*) FROM governance_audit_logs WHERE store_id = $1 AND action = 'CLEARANCE_PII_ANONYMIZED') AS audit`, storeID); err != nil {
			t.Fatal(err)
		}
		if check.ActiveUsers != 0 || check.PlainEmails != 0 || check.Phones != 0 || !strings.HasPrefix(check.CustomerPhone, "+999") ||
			check.OpenDevices != 0 || check.StoreActive || check.Status != "pii_anonymized_ledger_retained" || check.Audit != 1 {
			t.Fatalf("anonymization incomplete: %+v", check)
		}
		if n, err := gov.ExecuteDueClearances(ctx); err != nil || n != 0 {
			t.Fatalf("second run must be a no-op: n=%d err=%v", n, err)
		}
		if code, out := call("/otp", map[string]string{"store_slug": slug, "email": ownerEmail}); code != http.StatusAccepted || out["dev_otp_preview"] != nil {
			t.Fatalf("cleared store must not get codes: %d %v", code, out)
		}
	})
}
