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
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/config"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/handler"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/pkg/database"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/pkg/rbac"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/repository/postgres"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/service"
)

// TestIntegrationPlanQuotaAndDevicePairing covers PM-E1 and PM-E2 in an isolated trial store
// (trial_14d: 2 devices, 5 staff): the single-use 5-minute pairing token, its rejection paths and
// audit trail, concurrent exchange of one token, plan quotas returning 402 PLAN_LIMIT_REACHED,
// concurrent creates that cannot overshoot the quota, and unlimited quotas on Pro.
func TestIntegrationPlanQuotaAndDevicePairing(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_APP_DATABASE_URL"))
	migratorDSN := strings.TrimSpace(os.Getenv("TEST_MIGRATOR_DATABASE_URL"))
	if dsn == "" || migratorDSN == "" {
		t.Skip("TEST_APP_DATABASE_URL / TEST_MIGRATOR_DATABASE_URL not set; skipping plan quota integration test")
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
	slug := "quota-" + suffix
	email := func(name string) string { return name + ".quota." + suffix + "@cassy.local" }
	storeID, _, err := service.BootstrapStore(ctx, migratorDB, service.BootstrapStoreInput{
		Slug: slug, StoreName: "Kafe Kuota", OwnerName: "Pemilik Kuota", OwnerEmail: email("owner"), PIN: "482913",
	})
	if err != nil {
		t.Fatalf("bootstrap store: %v", err)
	}
	var plan string
	if err := migratorDB.GetContext(ctx, &plan, `SELECT plan_code FROM platform_tenant_registry WHERE store_id = $1`, storeID); err != nil || plan != "trial_14d" {
		t.Fatalf("a new store starts on trial_14d, got %q (%v)", plan, err)
	}

	gin.SetMode(gin.TestMode)
	authSvc := service.NewAuthService(db, postgres.NewAuthRepository(), "integration-test-jwt-secret-at-least-32-chars", service.AuthPolicy{
		AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: 12 * time.Hour, SessionIdleTimeout: 2 * time.Hour,
		MaxSessions: 3, PINMaxFailures: 5, PINLockout: 15 * time.Minute, MaxConcurrentHashes: 8,
	}, service.WithAuthEmail(service.NewEmailService(db, nil), "https://pos.kafe.test", "owner@kafe.test")) // invites only enqueue
	ah := handler.NewAuthHandler(authSvc)
	dh := handler.NewDeviceHandler(service.NewDeviceService(db, service.NewCatalogService(db)))

	r := gin.New()
	r.POST("/login", ah.Login)
	r.POST("/staff", ah.RequirePermission(rbac.StaffManage), ah.InviteStaff)
	r.POST("/store/devices", ah.RequirePermission(rbac.DeviceManage), dh.RegisterDevice)
	r.POST("/store/devices/:id/revoke", ah.RequirePermission(rbac.DeviceManage), dh.RevokeDevice)
	r.POST("/pos/devices/pairing-token", ah.RequirePermission(rbac.DeviceManage), dh.IssuePairingToken)
	r.POST("/pos/devices/pair", dh.PairDevice)
	r.GET("/device/catalog", dh.Catalog)

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
	res := call("POST", "/login", "", map[string]string{"store_slug": slug, "email": email("owner"), "pin": "482913"})
	if res.Code != http.StatusOK {
		t.Fatalf("owner login: %d: %s", res.Code, res.Body.String())
	}
	var sess struct {
		AccessToken string `json:"access_token"`
	}
	_ = json.Unmarshal(res.Body.Bytes(), &sess)
	owner := "Bearer " + sess.AccessToken

	type pairingToken struct {
		Token     string    `json:"pairing_token"`
		StoreSlug string    `json:"store_slug"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	issue := func(prefix string) pairingToken {
		t.Helper()
		res := call("POST", "/pos/devices/pairing-token", owner, map[string]any{"name": "Tablet " + prefix, "prefix": prefix})
		if res.Code != http.StatusCreated || res.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("issue pairing token: %d %q: %s", res.Code, res.Header().Get("Cache-Control"), res.Body.String())
		}
		var tok pairingToken
		_ = json.Unmarshal(res.Body.Bytes(), &tok)
		return tok
	}
	pair := func(token string) *httptest.ResponseRecorder {
		return call("POST", "/pos/devices/pair", "", map[string]string{"pairing_token": token})
	}

	var paired service.Device
	t.Run("PairingTokenIsSingleUseAndShortLived", func(t *testing.T) {
		if res := call("POST", "/pos/devices/pairing-token", "", map[string]any{"name": "X", "prefix": "A1"}); res.Code != http.StatusUnauthorized {
			t.Fatalf("issuing needs a staff session, got %d", res.Code)
		}
		if res := call("POST", "/pos/devices/pairing-token", owner, map[string]any{"name": "X", "prefix": "1A"}); res.Code != http.StatusUnprocessableEntity {
			t.Fatalf("a bad prefix is 422, got %d", res.Code)
		}
		tok := issue("a1")
		if tok.StoreSlug != slug || !strings.HasPrefix(tok.Token, storeID.String()+".") {
			t.Fatalf("token carries the store id and the QR gets the slug: %+v", tok)
		}
		if ttl := time.Until(tok.ExpiresAt); ttl <= 4*time.Minute || ttl > 5*time.Minute {
			t.Fatalf("pairing tokens live 5 minutes, got %s", ttl)
		}
		var storedHash string
		if err := migratorDB.GetContext(ctx, &storedHash, `SELECT token_hash FROM device_pairing_tokens WHERE store_id = $1`, storeID); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(tok.Token, storedHash) || strings.HasSuffix(tok.Token, storedHash) {
			t.Fatal("only a hash of the secret is stored")
		}
		parts := strings.Split(tok.Token, ".")
		for _, bad := range []string{"", "not-a-token", parts[0] + "." + parts[1] + ".wrong-secret", uuid.NewString() + "." + parts[1] + "." + parts[2]} {
			if res := pair(bad); res.Code != http.StatusUnauthorized && res.Code != http.StatusBadRequest {
				t.Fatalf("pair with %q must fail, got %d: %s", bad, res.Code, res.Body.String())
			}
		}
		res := pair(tok.Token)
		if res.Code != http.StatusCreated || res.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("pair: %d: %s", res.Code, res.Body.String())
		}
		_ = json.Unmarshal(res.Body.Bytes(), &paired)
		if paired.Prefix != "A1" || !strings.HasPrefix(paired.Credential, storeID.String()+"."+paired.ID.String()+".") {
			t.Fatalf("paired device gets its own credential: %+v", paired)
		}
		if res := call("GET", "/device/catalog", "Device "+paired.Credential, nil); res.Code != http.StatusOK {
			t.Fatalf("the paired credential works: %d", res.Code)
		}
		if res := pair(tok.Token); res.Code != http.StatusUnauthorized || !strings.Contains(res.Body.String(), "PAIRING_TOKEN_INVALID") {
			t.Fatalf("a used token is rejected, got %d: %s", res.Code, res.Body.String())
		}
		if res := call("POST", "/pos/devices/pairing-token", owner, map[string]any{"name": "Dobel", "prefix": "A1"}); res.Code != http.StatusConflict {
			t.Fatalf("a prefix already paired is 409 at issue time, got %d", res.Code)
		}

		expired := issue("e1")
		if _, err := migratorDB.ExecContext(ctx, `UPDATE device_pairing_tokens
			SET created_at = NOW() - INTERVAL '10 minutes', expires_at = NOW() - INTERVAL '6 minutes'
			WHERE store_id = $1 AND device_prefix = 'E1'`, storeID); err != nil {
			t.Fatal(err)
		}
		if res := pair(expired.Token); res.Code != http.StatusUnauthorized {
			t.Fatalf("an expired token is rejected, got %d", res.Code)
		}

		var actions []string
		if err := migratorDB.SelectContext(ctx, &actions, `SELECT action FROM audit_logs WHERE store_id = $1
			AND action IN ('DEVICE_PAIRING_TOKEN_ISSUED', 'DEVICE_PAIRED') ORDER BY created_at, action`, storeID); err != nil {
			t.Fatal(err)
		}
		if strings.Join(actions, ",") != "DEVICE_PAIRING_TOKEN_ISSUED,DEVICE_PAIRED,DEVICE_PAIRING_TOKEN_ISSUED" {
			t.Fatalf("issue and pair are audited: %v", actions)
		}
		var leaked bool
		if err := migratorDB.GetContext(ctx, &leaked, `SELECT EXISTS (SELECT 1 FROM audit_logs WHERE store_id = $1
			AND (metadata::text LIKE '%' || $2 || '%' OR metadata::text LIKE '%' || $3 || '%'))`,
			storeID, parts[2], strings.Split(paired.Credential, ".")[2]); err != nil || leaked {
			t.Fatalf("audit metadata never holds a secret (leaked=%v, err=%v)", leaked, err)
		}
	})

	t.Run("ConcurrentExchangeOfOneTokenYieldsOneDevice", func(t *testing.T) {
		tok := issue("b1")
		var wg sync.WaitGroup
		codes := make([]int, 8)
		for i := range codes {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				codes[i] = pair(tok.Token).Code
			}(i)
		}
		wg.Wait()
		created := 0
		for _, c := range codes {
			if c == http.StatusCreated {
				created++
			} else if c != http.StatusUnauthorized {
				t.Fatalf("losers see PAIRING_TOKEN_INVALID, got %v", codes)
			}
		}
		if created != 1 {
			t.Fatalf("exactly one exchange wins, got %v", codes)
		}
	})

	t.Run("DeviceQuotaReturns402AndCannotBeRaced", func(t *testing.T) {
		// A1 and B1 are active: trial_14d allows 2.
		res := call("POST", "/store/devices", owner, map[string]any{"name": "Ketiga", "prefix": "C1"})
		if res.Code != http.StatusPaymentRequired {
			t.Fatalf("a third device is 402, got %d: %s", res.Code, res.Body.String())
		}
		var body struct {
			Code     string `json:"code"`
			Resource string `json:"resource"`
			Limit    int    `json:"limit"`
			PlanCode string `json:"plan_code"`
		}
		_ = json.Unmarshal(res.Body.Bytes(), &body)
		if body.Code != "PLAN_LIMIT_REACHED" || body.Resource != "devices" || body.Limit != 2 || body.PlanCode != "trial_14d" {
			t.Fatalf("402 body names resource, limit and plan: %+v", body)
		}
		if res := call("POST", "/pos/devices/pairing-token", owner, map[string]any{"name": "Ketiga", "prefix": "C2"}); res.Code != http.StatusPaymentRequired {
			t.Fatalf("issuing a token at the quota is 402, got %d", res.Code)
		}
		if res := call("POST", "/store/devices/"+paired.ID.String()+"/revoke", owner, nil); res.Code != http.StatusOK {
			t.Fatalf("revoke: %d", res.Code)
		}
		var wg sync.WaitGroup
		codes := make([]int, 6)
		for i := range codes {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				codes[i] = call("POST", "/store/devices", owner, map[string]any{"name": "Balapan", "prefix": fmt.Sprintf("R%d", i)}).Code
			}(i)
		}
		wg.Wait()
		created := 0
		for _, c := range codes {
			if c == http.StatusCreated {
				created++
			} else if c != http.StatusPaymentRequired {
				t.Fatalf("concurrent creates are 201 or 402, got %v", codes)
			}
		}
		if created != 1 {
			t.Fatalf("one free slot admits exactly one concurrent create, got %v", codes)
		}
	})

	t.Run("StaffQuotaAndUnlimitedPro", func(t *testing.T) {
		invite := func(name string) *httptest.ResponseRecorder {
			return call("POST", "/staff", owner, map[string]string{"full_name": "Staf " + name, "email": email(name), "role": "cashier"})
		}
		for i := 1; i <= 4; i++ { // owner + 4 = trial_14d's 5 staff
			if res := invite(fmt.Sprintf("s%d", i)); res.Code != http.StatusCreated {
				t.Fatalf("invite %d: %d: %s", i, res.Code, res.Body.String())
			}
		}
		if res := invite("s5"); res.Code != http.StatusPaymentRequired || !strings.Contains(res.Body.String(), `"resource":"staff"`) {
			t.Fatalf("a sixth staff member is 402, got %d: %s", res.Code, res.Body.String())
		}
		if _, err := migratorDB.ExecContext(ctx, `UPDATE platform_tenant_registry SET plan_code = 'pro_cafe' WHERE store_id = $1`, storeID); err != nil {
			t.Fatal(err)
		}
		if res := invite("s5"); res.Code != http.StatusCreated {
			t.Fatalf("pro_cafe has no staff limit, got %d: %s", res.Code, res.Body.String())
		}
		if res := call("POST", "/store/devices", owner, map[string]any{"name": "Pro", "prefix": "P1"}); res.Code != http.StatusCreated {
			t.Fatalf("pro_cafe has no device limit, got %d: %s", res.Code, res.Body.String())
		}
	})
}
