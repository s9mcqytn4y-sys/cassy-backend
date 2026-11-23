package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/config"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/handler"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/pkg/auth"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/pkg/database"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/pkg/rbac"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/repository/postgres"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/service"
)

// TestIntegrationManagerRole covers K-04: a store manager logs in, may authorise a cashier's
// override with their own PIN and act on owner-threshold actions directly, while owner-only
// endpoints stay closed to them and a cashier's PIN never counts as an approver PIN.
func TestIntegrationManagerRole(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_APP_DATABASE_URL"))
	migratorDSN := strings.TrimSpace(os.Getenv("TEST_MIGRATOR_DATABASE_URL"))
	if dsn == "" || migratorDSN == "" {
		t.Skip("TEST_APP_DATABASE_URL / TEST_MIGRATOR_DATABASE_URL not set; skipping manager role integration test")
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

	gin.SetMode(gin.TestMode)
	policy := service.AuthPolicy{
		AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: 12 * time.Hour, SessionIdleTimeout: 2 * time.Hour,
		MaxSessions: 3, PINMaxFailures: 5, PINLockout: 10 * time.Minute, MaxConcurrentHashes: 8,
	}
	authSvc := service.NewAuthService(db, postgres.NewAuthRepository(), "integration-test-jwt-secret-at-least-32-chars", policy)
	ah := handler.NewAuthHandler(authSvc)
	orderRepo := postgres.NewOrderRepository()
	paymentRepo := postgres.NewPaymentRepository()
	opSvc := service.NewOperationalService(db, orderRepo, paymentRepo, nil, nil, nil)
	opSvc.SetPINPolicy(policy, nil)
	pos := handler.NewPOSHandler(service.NewOrderService(db, orderRepo), service.NewPaymentService(db, paymentRepo, orderRepo, nil), service.NewSettingsService(db))
	pos.SetOperational(opSvc)

	r := gin.New()
	r.POST("/login", ah.Login)
	r.POST("/pos/products/:id/stock-override", ah.RequirePermission(rbac.StockOverride), pos.OverrideStock)
	r.GET("/owner-only", ah.RequirePermission(rbac.StoreSettingsManage), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	r.GET("/force-close-gate", ah.RequirePermission(rbac.ShiftForceClose), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	call := func(method, path, token string, body any) *httptest.ResponseRecorder {
		var buf bytes.Buffer
		if body != nil {
			_ = json.NewEncoder(&buf).Encode(body)
		}
		req := httptest.NewRequest(method, path, &buf)
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	suffix := uuid.NewString()[:8]
	slug := "manager-" + suffix
	email := func(name string) string { return name + ".manager." + suffix + "@cassy.local" }
	const ownerPIN, managerPIN, cashierPIN = "482913", "739164", "528307"
	storeID, _, err := service.BootstrapStore(ctx, migratorDB, service.BootstrapStoreInput{
		Slug: slug, StoreName: "Kafe Manajer", OwnerName: "Manager test owner", OwnerEmail: email("owner"), PIN: ownerPIN,
	})
	if err != nil {
		t.Fatalf("bootstrap store: %v", err)
	}
	productID := uuid.New()
	if err := database.WithTenantTx(ctx, db, storeID, func(tx *sqlx.Tx) error {
		for _, u := range []struct{ name, role, pin string }{{"manager", "manager", managerPIN}, {"cashier", "cashier", cashierPIN}} {
			hash, err := auth.HashPIN(u.pin)
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO users (id, store_id, email, full_name, role, pin_hash, is_active, pin_set_at)
				VALUES ($1, $2, $3, $4, $5, $6, true, NOW())`, uuid.New(), storeID, email(u.name), "Uji "+u.name, u.role, hash); err != nil {
				return err
			}
		}
		_, err := tx.ExecContext(ctx, `WITH p AS (INSERT INTO products (id, store_id, name, base_price, is_available, track_stock, stock_quantity)
			VALUES ($1, $2, $3, 10000, true, true, 5)
			RETURNING store_id, id, base_price)
			INSERT INTO product_variants (store_id, product_id, name, price, is_default) SELECT store_id, id, 'Reguler', base_price, true FROM p`, productID, storeID, "Manager "+suffix)
		return err
	}); err != nil {
		t.Fatalf("seed store: %v", err)
	}

	login := func(name, pin string) string {
		t.Helper()
		res := call(http.MethodPost, "/login", "", map[string]string{"store_slug": slug, "email": email(name), "pin": pin})
		if res.Code != http.StatusOK {
			t.Fatalf("login %s failed: %d: %s", name, res.Code, res.Body.String())
		}
		var body struct {
			AccessToken string `json:"access_token"`
		}
		_ = json.Unmarshal(res.Body.Bytes(), &body)
		return body.AccessToken
	}
	managerToken, cashierToken := login("manager", managerPIN), login("cashier", cashierPIN)
	override := func(token string, body map[string]any) *httptest.ResponseRecorder {
		body["reason"] = "Opname"
		return call(http.MethodPost, fmt.Sprintf("/pos/products/%s/stock-override", productID), token, body)
	}

	t.Run("ManagerPINAuthorisesCashierOverride", func(t *testing.T) {
		if res := override(cashierToken, map[string]any{"new_quantity": 4, "owner_pin": managerPIN}); res.Code != http.StatusOK {
			t.Fatalf("manager PIN must authorise a cashier override: %d: %s", res.Code, res.Body.String())
		}
	})
	t.Run("CashierPINIsNotAnApproverPIN", func(t *testing.T) {
		res := override(cashierToken, map[string]any{"new_quantity": 3, "owner_pin": cashierPIN})
		if res.Code != http.StatusUnauthorized || !strings.Contains(res.Body.String(), "INVALID_OWNER_PIN") {
			t.Fatalf("cashier PIN must not approve: %d: %s", res.Code, res.Body.String())
		}
	})
	t.Run("ManagerActsWithOwnPIN", func(t *testing.T) {
		if res := override(managerToken, map[string]any{"new_quantity": 2, "owner_pin": managerPIN}); res.Code != http.StatusOK {
			t.Fatalf("manager must act on an override with their own PIN: %d: %s", res.Code, res.Body.String())
		}
	})
	t.Run("PermissionBoundary", func(t *testing.T) {
		if res := call(http.MethodGet, "/force-close-gate", managerToken, nil); res.Code != http.StatusNoContent {
			t.Fatalf("manager must pass shift.force_close: %d", res.Code)
		}
		if res := call(http.MethodGet, "/owner-only", managerToken, nil); res.Code != http.StatusForbidden {
			t.Fatalf("manager must not pass store.settings.manage: %d", res.Code)
		}
	})
}
