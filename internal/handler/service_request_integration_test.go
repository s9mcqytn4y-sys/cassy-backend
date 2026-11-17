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
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/domain"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/handler"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/pkg/database"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/pkg/rbac"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/repository/postgres"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/service"
)

// TestServiceRequestIntegration covers W4.3 "Panggil Pelayan": the call_waiter flag gate, owner-only
// templates, idempotent live requests per template and table, the per-session burst limit, note
// validation, own-session isolation for guests, and the staff acknowledge/done lifecycle.
func TestServiceRequestIntegration(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_APP_DATABASE_URL"))
	if dsn == "" {
		t.Skip("TEST_APP_DATABASE_URL not set; skipping service request integration test")
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
	orderRepo := postgres.NewOrderRepository()
	orderSvc := service.NewOrderService(db, orderRepo)
	paymentSvc := service.NewPaymentService(db, postgres.NewPaymentRepository(), orderRepo, qrisGateway{})
	gh := handler.NewGuestHandler(service.NewGuestService(db, orderSvc, paymentSvc))
	sh := handler.NewShiftHandler(service.NewShiftService(db))
	fh := handler.NewFeatureHandler(service.NewFeatureService(db))
	rh := handler.NewServiceRequestHandler(service.NewServiceRequestService(db))

	r := gin.New()
	r.POST("/login", ah.Login)
	r.POST("/pos/drawers", ah.RequirePermission(rbac.CashDrawerManage), sh.CreateDrawer)
	r.POST("/pos/shifts", ah.RequireStaff(), sh.OpenShift)
	r.POST("/pos/shifts/:id/force-close", ah.RequireStaff(), sh.ForceCloseShift)
	r.PUT("/store/features/:key", ah.RequirePermission(rbac.StoreSettingsManage), fh.SetFeature)
	r.POST("/store/service-request-templates", ah.RequirePermission(rbac.StoreSettingsManage), rh.CreateTemplate)
	r.PATCH("/store/service-request-templates/:id", ah.RequirePermission(rbac.StoreSettingsManage), rh.UpdateTemplate)
	r.GET("/store/service-requests", ah.RequirePermission(rbac.OrderReadStore), rh.ListForStaff)
	r.POST("/store/service-requests/:id/acknowledge", ah.RequirePermission(rbac.OrderReadStore), rh.Acknowledge)
	r.POST("/store/service-requests/:id/done", ah.RequirePermission(rbac.OrderReadStore), rh.Done)
	r.POST("/public/qr/:token/session", gh.StartSession)
	guest := r.Group("/public/guest", gh.RequireGuest())
	guest.GET("/service-request-templates", rh.GuestTemplates)
	guest.GET("/service-requests", rh.ListForGuest)
	guest.POST("/service-requests", rh.CreateForGuest)

	type req struct {
		cookie, bearer string
	}
	call := func(method, path string, o req, body any) *httptest.ResponseRecorder {
		var buf bytes.Buffer
		if body != nil {
			_ = json.NewEncoder(&buf).Encode(body)
		}
		r0 := httptest.NewRequest(method, path, &buf)
		r0.Header.Set("Content-Type", "application/json")
		r0.Header.Set(handler.CSRFHeader, "1")
		if o.cookie != "" {
			r0.AddCookie(&http.Cookie{Name: handler.GuestCookieName, Value: o.cookie})
		}
		if o.bearer != "" {
			r0.Header.Set("Authorization", "Bearer "+o.bearer)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, r0)
		return w
	}
	login := func(email string) string {
		t.Helper()
		res := call("POST", "/login", req{}, map[string]string{"store_slug": "central-cafe", "email": email, "pin": "482913"})
		var s struct {
			AccessToken string `json:"access_token"`
		}
		_ = json.Unmarshal(res.Body.Bytes(), &s)
		if res.Code != http.StatusOK {
			t.Fatalf("login %s failed: %d", email, res.Code)
		}
		return s.AccessToken
	}
	owner, cashier := login("abdulaziz543333@gmail.com"), login("cashier.central@mycaffe.local")

	// QR ordering needs an open shift (QR4) and a table.
	qrToken := "sr" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if err := database.WithTenantTx(ctx, db, storeID, func(tx *sqlx.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO tables (store_id, table_number, qr_token) VALUES ($1, $2, $3)`, storeID, "S-"+qrToken[:6], qrToken)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	var drawer domain.CashDrawer
	_ = json.Unmarshal(call("POST", "/pos/drawers", req{bearer: owner}, map[string]any{"name": "Laci Pelayan " + uuid.NewString()[:6]}).Body.Bytes(), &drawer)
	shiftRes := call("POST", "/pos/shifts", req{bearer: owner}, map[string]any{"drawer_id": drawer.ID, "opening_float": 0})
	if shiftRes.Code != http.StatusCreated {
		t.Fatalf("open shift failed: %d: %s", shiftRes.Code, shiftRes.Body.String())
	}
	var opened struct {
		Shift domain.Shift `json:"shift"`
	}
	_ = json.Unmarshal(shiftRes.Body.Bytes(), &opened)
	t.Cleanup(func() {
		call("PUT", "/store/features/call_waiter", req{bearer: owner}, map[string]any{"enabled": false})
		call("POST", fmt.Sprintf("/pos/shifts/%s/force-close", opened.Shift.ID), req{bearer: owner}, map[string]any{"reason": "Akhir test pelayan"})
	})
	session := func() string {
		t.Helper()
		res := call("POST", "/public/qr/"+qrToken+"/session", req{}, nil)
		for _, ck := range res.Result().Cookies() {
			if ck.Name == handler.GuestCookieName {
				return ck.Value
			}
		}
		t.Fatalf("guest session failed: %d: %s", res.Code, res.Body.String())
		return ""
	}
	guestA, guestB := session(), session()

	var tmpl service.ServiceRequestTemplate
	t.Run("OwnerKeepsTemplates", func(t *testing.T) {
		if res := call("POST", "/store/service-request-templates", req{bearer: cashier}, map[string]any{"label": "Minta tisu"}); res.Code != http.StatusForbidden {
			t.Fatalf("a cashier cannot add templates, got %d", res.Code)
		}
		if res := call("POST", "/store/service-request-templates", req{bearer: owner}, map[string]any{"label": "  "}); res.Code != http.StatusUnprocessableEntity {
			t.Fatalf("an empty label is 422, got %d", res.Code)
		}
		res := call("POST", "/store/service-request-templates", req{bearer: owner}, map[string]any{"label": "Minta tisu " + uuid.NewString()[:4], "sort_order": 1})
		if res.Code != http.StatusCreated {
			t.Fatalf("create template: %d: %s", res.Code, res.Body.String())
		}
		_ = json.Unmarshal(res.Body.Bytes(), &tmpl)
	})

	t.Run("FlagGatesGuests", func(t *testing.T) {
		call("PUT", "/store/features/call_waiter", req{bearer: owner}, map[string]any{"enabled": false})
		if res := call("POST", "/public/guest/service-requests", req{cookie: guestA}, map[string]any{"template_id": tmpl.ID}); res.Code != http.StatusForbidden {
			t.Fatalf("call_waiter off must refuse guests, got %d", res.Code)
		}
		call("PUT", "/store/features/call_waiter", req{bearer: owner}, map[string]any{"enabled": true})
		res := call("GET", "/public/guest/service-request-templates", req{cookie: guestA}, nil)
		if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), tmpl.ID.String()) {
			t.Fatalf("guests see active templates: %d: %s", res.Code, res.Body.String())
		}
	})

	t.Run("LifecycleIdempotencyAndIsolation", func(t *testing.T) {
		long := strings.Repeat("a", 81)
		if res := call("POST", "/public/guest/service-requests", req{cookie: guestA}, map[string]any{"template_id": tmpl.ID, "note": long}); res.Code != http.StatusUnprocessableEntity {
			t.Fatalf("a note over 80 characters is 422, got %d", res.Code)
		}
		if res := call("POST", "/public/guest/service-requests", req{cookie: guestA}, map[string]any{"template_id": uuid.New()}); res.Code != http.StatusNotFound {
			t.Fatalf("an unknown template is 404, got %d", res.Code)
		}
		res := call("POST", "/public/guest/service-requests", req{cookie: guestA}, map[string]any{"template_id": tmpl.ID, "note": "dua\x00 lembar"})
		if res.Code != http.StatusCreated {
			t.Fatalf("create request: %d: %s", res.Code, res.Body.String())
		}
		var first service.ServiceRequest
		_ = json.Unmarshal(res.Body.Bytes(), &first)
		if first.Status != "open" || first.Note == nil || *first.Note != "dua lembar" {
			t.Fatalf("a new request is open and its note is cleaned: %+v", first)
		}
		var again service.ServiceRequest
		_ = json.Unmarshal(call("POST", "/public/guest/service-requests", req{cookie: guestB}, map[string]any{"template_id": tmpl.ID}).Body.Bytes(), &again)
		if again.ID != first.ID {
			t.Fatal("a second tap at the same table returns the live request instead of a new one")
		}
		if body := call("GET", "/public/guest/service-requests", req{cookie: guestB}, nil).Body.String(); strings.Contains(body, first.ID.String()) {
			t.Fatal("a guest only lists requests from its own session")
		}
		if body := call("GET", "/store/service-requests", req{bearer: cashier}, nil).Body.String(); !strings.Contains(body, first.ID.String()) {
			t.Fatalf("staff see open requests: %s", body)
		}
		if res := call("POST", "/store/service-requests/"+first.ID.String()+"/acknowledge", req{bearer: cashier}, nil); res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"acknowledged"`) {
			t.Fatalf("acknowledge: %d: %s", res.Code, res.Body.String())
		}
		if res := call("POST", "/store/service-requests/"+first.ID.String()+"/acknowledge", req{bearer: cashier}, nil); res.Code != http.StatusConflict {
			t.Fatalf("acknowledging twice is 409, got %d", res.Code)
		}
		if res := call("POST", "/store/service-requests/"+first.ID.String()+"/done", req{bearer: cashier}, nil); res.Code != http.StatusOK {
			t.Fatalf("done: %d", res.Code)
		}
	})

	t.Run("BurstLimitPerSession", func(t *testing.T) {
		// One request is already spent in this window; four more fit, the next is refused.
		for i := 0; i < 4; i++ {
			res := call("POST", "/public/guest/service-requests", req{cookie: guestA}, map[string]any{"template_id": tmpl.ID})
			if res.Code != http.StatusCreated {
				t.Fatalf("request %d: %d: %s", i+2, res.Code, res.Body.String())
			}
			var sr service.ServiceRequest
			_ = json.Unmarshal(res.Body.Bytes(), &sr)
			call("POST", "/store/service-requests/"+sr.ID.String()+"/done", req{bearer: cashier}, nil)
		}
		if res := call("POST", "/public/guest/service-requests", req{cookie: guestA}, map[string]any{"template_id": tmpl.ID}); res.Code != http.StatusTooManyRequests {
			t.Fatalf("the sixth request in ten minutes is 429, got %d", res.Code)
		}
	})

	t.Run("InactiveTemplateHidden", func(t *testing.T) {
		if res := call("PATCH", "/store/service-request-templates/"+tmpl.ID.String(), req{bearer: owner}, map[string]any{"is_active": false}); res.Code != http.StatusOK {
			t.Fatalf("deactivate: %d", res.Code)
		}
		if body := call("GET", "/public/guest/service-request-templates", req{cookie: guestB}, nil).Body.String(); strings.Contains(body, tmpl.ID.String()) {
			t.Fatal("a deactivated template is hidden from guests")
		}
	})
}
