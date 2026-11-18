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
	"github.com/jmoiron/sqlx"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/config"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/handler"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/pkg/database"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/pkg/rbac"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/repository/postgres"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/service"
)

// TestFeatureFlagIntegration covers W4.2: staff read flags with defaults, only store.settings.manage
// changes them, unknown keys are rejected, and a real change is audited once.
func TestFeatureFlagIntegration(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_APP_DATABASE_URL"))
	if dsn == "" {
		t.Skip("TEST_APP_DATABASE_URL not set; skipping feature flag integration test")
	}
	db, err := database.NewPostgresDB(&config.Config{DatabaseURL: dsn})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	storeID := uuid.MustParse("11111111-1111-4111-8111-111111111111")

	gin.SetMode(gin.TestMode)
	authSvc := service.NewAuthService(db, postgres.NewAuthRepository(), "integration-test-jwt-secret-at-least-32-chars", service.AuthPolicy{
		AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: 12 * time.Hour, SessionIdleTimeout: 2 * time.Hour,
		MaxSessions: 3, PINMaxFailures: 5, PINLockout: 15 * time.Minute, MaxConcurrentHashes: 8,
	})
	ah := handler.NewAuthHandler(authSvc)
	fh := handler.NewFeatureHandler(service.NewFeatureService(db))
	r := gin.New()
	r.POST("/login", ah.Login)
	r.GET("/store/features", ah.RequireStaff(), fh.ListFeatures)
	r.PUT("/store/features/:key", ah.RequirePermission(rbac.StoreSettingsManage), fh.SetFeature)

	call := func(method, path, authz string, body any) *httptest.ResponseRecorder {
		var buf bytes.Buffer
		if body != nil {
			_ = json.NewEncoder(&buf).Encode(body)
		}
		req := httptest.NewRequest(method, path, &buf)
		req.Header.Set("Content-Type", "application/json")
		if authz != "" {
			req.Header.Set("Authorization", authz)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	bearer := func(email string) string {
		t.Helper()
		res := call("POST", "/login", "", map[string]string{"store_slug": "central-cafe", "email": email, "pin": "482913"})
		var s struct {
			AccessToken string `json:"access_token"`
		}
		_ = json.Unmarshal(res.Body.Bytes(), &s)
		if res.Code != http.StatusOK {
			t.Fatalf("login %s failed: %d", email, res.Code)
		}
		return "Bearer " + s.AccessToken
	}
	type flags struct {
		Features map[string]bool `json:"features"`
	}
	decode := func(res *httptest.ResponseRecorder) map[string]bool {
		t.Helper()
		var f flags
		if err := json.Unmarshal(res.Body.Bytes(), &f); err != nil {
			t.Fatalf("decode: %v: %s", err, res.Body.String())
		}
		return f.Features
	}
	audits := func() int {
		t.Helper()
		var n int
		if err := database.WithTenantTx(ctx, db, storeID, func(tx *sqlx.Tx) error {
			return tx.GetContext(ctx, &n, `SELECT COUNT(*) FROM audit_logs WHERE store_id = $1 AND action = 'FEATURE_FLAG_CHANGED'`, storeID)
		}); err != nil {
			t.Fatal(err)
		}
		return n
	}
	owner, cashier := bearer("abdulaziz543333@gmail.com"), bearer("cashier.central@mycaffe.local")
	t.Cleanup(func() { call("PUT", "/store/features/call_waiter", owner, map[string]any{"enabled": false}) })

	res := call("GET", "/store/features", cashier, nil)
	if res.Code != http.StatusOK {
		t.Fatalf("staff can read flags, got %d", res.Code)
	}
	if got := decode(res); len(got) != 8 || !got["kds"] {
		t.Fatalf("all 8 flags with defaults are returned: %v", got)
	}
	if res := call("PUT", "/store/features/call_waiter", cashier, map[string]any{"enabled": true}); res.Code != http.StatusForbidden {
		t.Fatalf("a cashier cannot change flags, got %d", res.Code)
	}
	if res := call("PUT", "/store/features/printer", owner, map[string]any{"enabled": true}); res.Code != http.StatusUnprocessableEntity {
		t.Fatalf("an unknown key is 422, got %d", res.Code)
	}
	if res := call("PUT", "/store/features/call_waiter", owner, map[string]any{}); res.Code != http.StatusBadRequest && res.Code != http.StatusUnprocessableEntity {
		t.Fatalf("enabled is required, got %d", res.Code)
	}

	call("PUT", "/store/features/call_waiter", owner, map[string]any{"enabled": false})
	before := audits()
	res = call("PUT", "/store/features/call_waiter", owner, map[string]any{"enabled": true})
	if res.Code != http.StatusOK || !decode(res)["call_waiter"] {
		t.Fatalf("owner turns call_waiter on: %d: %s", res.Code, res.Body.String())
	}
	call("PUT", "/store/features/call_waiter", owner, map[string]any{"enabled": true})
	if got := audits() - before; got != 1 {
		t.Fatalf("one real change is audited once, a repeat is not; got %d audits", got)
	}
}
