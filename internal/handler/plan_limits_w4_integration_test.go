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
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/domain"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/handler"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/pkg/database"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/pkg/rbac"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/repository/postgres"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/service"
)

// TestIntegrationPlanLimitsW4 covers FL1 W4 on an isolated trial store: the public price list
// (annual = 10 x monthly, Starter with 1 KDS station), the KDS station quota, the supports_kds_queue_tv
// gate on flags and device roles, and the read-only state of a store whose trial ended.
func TestIntegrationPlanLimitsW4(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_APP_DATABASE_URL"))
	migratorDSN := strings.TrimSpace(os.Getenv("TEST_MIGRATOR_DATABASE_URL"))
	if dsn == "" || migratorDSN == "" {
		t.Skip("TEST_APP_DATABASE_URL / TEST_MIGRATOR_DATABASE_URL not set; skipping W4 plan limit integration test")
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
	slug := "w4-" + suffix
	ownerEmail := "owner.w4." + suffix + "@cassy.local"
	storeID, _, err := service.BootstrapStore(ctx, migratorDB, service.BootstrapStoreInput{
		Slug: slug, StoreName: "Kafe Batas", OwnerName: "Pemilik Batas", OwnerEmail: ownerEmail, PIN: "482913",
	})
	if err != nil {
		t.Fatalf("bootstrap store: %v", err)
	}

	gin.SetMode(gin.TestMode)
	authSvc := service.NewAuthService(db, postgres.NewAuthRepository(), "integration-test-jwt-secret-at-least-32-chars", service.AuthPolicy{
		AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: 12 * time.Hour, SessionIdleTimeout: 2 * time.Hour,
		MaxSessions: 3, PINMaxFailures: 5, PINLockout: 15 * time.Minute, MaxConcurrentHashes: 8,
	})
	ah := handler.NewAuthHandler(authSvc)
	dh := handler.NewDeviceHandler(service.NewDeviceService(db, service.NewCatalogService(db)))
	fh := handler.NewFeatureHandler(service.NewFeatureService(db))
	gh := handler.NewGovernanceHandler(service.NewGovernanceService(db, &config.Config{AppEnv: "test"}), "")

	r := gin.New()
	r.POST("/api/v1/auth/staff/login", ah.Login)
	r.GET("/api/v1/public/plans", gh.ListPublicSubscriptionPlans)
	r.GET("/api/v1/store/devices", ah.RequirePermission(rbac.DeviceManage), dh.ListDevices)
	r.POST("/api/v1/store/devices", ah.RequirePermission(rbac.DeviceManage), dh.RegisterDevice)
	r.PATCH("/api/v1/store/devices/:id", ah.RequirePermission(rbac.DeviceManage), dh.UpdateDevice)
	r.PUT("/api/v1/store/features/:key", ah.RequirePermission(rbac.StoreSettingsManage), fh.SetFeature)

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
	errorCode := func(res *httptest.ResponseRecorder) string {
		var body struct {
			Code string `json:"code"`
		}
		_ = json.Unmarshal(res.Body.Bytes(), &body)
		return body.Code
	}
	res := call("POST", "/api/v1/auth/staff/login", "", map[string]string{"store_slug": slug, "email": ownerEmail, "pin": "482913"})
	if res.Code != http.StatusOK {
		t.Fatalf("owner login: %d: %s", res.Code, res.Body.String())
	}
	var sess struct {
		AccessToken string `json:"access_token"`
	}
	_ = json.Unmarshal(res.Body.Bytes(), &sess)
	owner := "Bearer " + sess.AccessToken
	register := func(prefix, role string) *httptest.ResponseRecorder {
		return call("POST", "/api/v1/store/devices", owner, map[string]any{"name": "Layar " + prefix, "prefix": prefix, "role": role})
	}

	t.Run("PublicPlansCarryAnnualTenMonthsAndQuotas", func(t *testing.T) {
		res := call("GET", "/api/v1/public/plans", "", nil)
		if res.Code != http.StatusOK || !strings.Contains(res.Header().Get("Cache-Control"), "public") {
			t.Fatalf("plans: %d %q", res.Code, res.Header().Get("Cache-Control"))
		}
		var body struct {
			Plans []domain.SubscriptionPlan `json:"plans"`
		}
		if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		byCode := map[string]domain.SubscriptionPlan{}
		for _, p := range body.Plans {
			if p.AnnualPriceIDR != p.MonthlyPriceIDR*10 {
				t.Fatalf("%s: annual %d is not 10 x monthly %d", p.Code, p.AnnualPriceIDR, p.MonthlyPriceIDR)
			}
			byCode[p.Code] = p
		}
		starter := byCode["starter_umkm"]
		if starter.AnnualPriceIDR != 1490000 || !starter.SupportsKDSQueueTV || starter.MaxKDSStations != 1 ||
			starter.MaxTables != 15 || starter.MaxProducts != 100 || starter.MaxStaff != 5 || starter.MaxDevices != 2 {
			t.Fatalf("starter row: %+v", starter)
		}
		if pro := byCode["pro_cafe"]; pro.AnnualPriceIDR != 2990000 || pro.MaxKDSStations != 0 {
			t.Fatalf("pro row: %+v", pro)
		}
	})

	t.Run("TrialAllowsOneKDSStation", func(t *testing.T) {
		if res := register("K1", service.DeviceRoleKDS); res.Code != http.StatusCreated {
			t.Fatalf("first KDS station: %d: %s", res.Code, res.Body.String())
		}
		res := register("K2", service.DeviceRoleKDS)
		if res.Code != http.StatusPaymentRequired || errorCode(res) != "PLAN_LIMIT_REACHED" || !strings.Contains(res.Body.String(), "kds_stations") {
			t.Fatalf("second KDS station must be 402 kds_stations, got %d: %s", res.Code, res.Body.String())
		}
		pos := register("P1", service.DeviceRolePOS)
		if pos.Code != http.StatusCreated {
			t.Fatalf("a POS device still fits the 2-device quota: %d: %s", pos.Code, pos.Body.String())
		}
		var d service.Device
		_ = json.Unmarshal(pos.Body.Bytes(), &d)
		if res := call("PATCH", "/api/v1/store/devices/"+d.ID.String(), owner, map[string]any{"role": service.DeviceRoleKDS}); res.Code != http.StatusPaymentRequired {
			t.Fatalf("switching a POS to a second KDS role must be 402, got %d: %s", res.Code, res.Body.String())
		}
	})

	t.Run("PlanWithoutKDSQueueTVBlocksFlagAndRoles", func(t *testing.T) {
		planCode := "it_nokds_" + suffix
		if _, err := migratorDB.ExecContext(ctx, `INSERT INTO subscription_plans (code, name_id, name_en, monthly_price_idr, annual_price_idr,
			supports_kds_queue_tv, is_active) VALUES ($1, 'Uji', 'Test', 0, 0, false, false)`, planCode); err != nil {
			t.Fatal(err)
		}
		if _, err := migratorDB.ExecContext(ctx, `UPDATE platform_tenant_registry SET plan_code = $2 WHERE store_id = $1`, storeID, planCode); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_, _ = migratorDB.ExecContext(ctx, `UPDATE platform_tenant_registry SET plan_code = 'trial_14d' WHERE store_id = $1`, storeID)
			_, _ = migratorDB.ExecContext(ctx, `DELETE FROM subscription_plans WHERE code = $1`, planCode)
		})
		if res := call("PUT", "/api/v1/store/features/kds", owner, map[string]any{"enabled": false}); res.Code != http.StatusOK {
			t.Fatalf("turning kds off is always allowed: %d: %s", res.Code, res.Body.String())
		}
		res := call("PUT", "/api/v1/store/features/kds", owner, map[string]any{"enabled": true})
		if res.Code != http.StatusPaymentRequired || errorCode(res) != "PLAN_FEATURE_UNAVAILABLE" {
			t.Fatalf("turning kds on without the capability must be 402, got %d: %s", res.Code, res.Body.String())
		}
		if res := call("PUT", "/api/v1/store/features/call_waiter", owner, map[string]any{"enabled": true}); res.Code != http.StatusOK {
			t.Fatalf("other flags stay free: %d: %s", res.Code, res.Body.String())
		}
		if res := register("Q1", service.DeviceRoleQueueDisplay); res.Code != http.StatusPaymentRequired || errorCode(res) != "PLAN_FEATURE_UNAVAILABLE" {
			t.Fatalf("a queue TV without the capability must be 402, got %d: %s", res.Code, res.Body.String())
		}
		var kds int
		if err := migratorDB.GetContext(ctx, &kds, `SELECT COUNT(*) FROM devices WHERE store_id = $1 AND role = 'kds' AND revoked_at IS NULL`, storeID); err != nil || kds != 1 {
			t.Fatalf("the existing KDS device is left untouched: %d (%v)", kds, err)
		}
	})

	t.Run("ExpiredTrialIsReadOnly", func(t *testing.T) {
		if _, err := migratorDB.ExecContext(ctx, `UPDATE stores SET created_at = NOW() - INTERVAL '15 days' WHERE id = $1`, storeID); err != nil {
			t.Fatal(err)
		}
		if _, err := migratorDB.ExecContext(ctx, `UPDATE store_kyb_profiles SET current_period_ends_at = NOW() - INTERVAL '1 day' WHERE store_id = $1`, storeID); err != nil {
			t.Fatal(err)
		}
		res := call("PUT", "/api/v1/store/features/call_waiter", owner, map[string]any{"enabled": false})
		if res.Code != http.StatusPaymentRequired || errorCode(res) != "SUBSCRIPTION_READ_ONLY" {
			t.Fatalf("writes after the trial must be 402 SUBSCRIPTION_READ_ONLY, got %d: %s", res.Code, res.Body.String())
		}
		if res := call("GET", "/api/v1/store/devices", owner, nil); res.Code != http.StatusOK {
			t.Fatalf("reads stay open: %d", res.Code)
		}
		if res := call("POST", "/api/v1/auth/staff/login", "", map[string]string{"store_slug": slug, "email": ownerEmail, "pin": "482913"}); res.Code != http.StatusOK {
			t.Fatalf("sign-in stays open: %d", res.Code)
		}
		if _, err := migratorDB.ExecContext(ctx, `UPDATE platform_tenant_registry SET plan_code = 'starter_umkm', subscription_status = 'active' WHERE store_id = $1`, storeID); err != nil {
			t.Fatal(err)
		}
		if res := call("PUT", "/api/v1/store/features/call_waiter", owner, map[string]any{"enabled": false}); res.Code != http.StatusOK {
			t.Fatalf("a paid plan lifts read-only: %d: %s", res.Code, res.Body.String())
		}
	})
}
