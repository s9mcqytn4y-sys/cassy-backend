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
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/domain"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/handler"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/pkg/database"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/pkg/rbac"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/repository/postgres"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/service"
)

// TestStaffOrderSplitIntegration covers Release One PR 9: a cashier divides a pending pay-first order
// into server-priced shares, settles them one by one in cash, and the kitchen receives the order only
// after the last share is paid. Dividing twice, after a payment, or without permission is refused.
func TestStaffOrderSplitIntegration(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_APP_DATABASE_URL"))
	if dsn == "" {
		t.Skip("TEST_APP_DATABASE_URL not set; skipping staff order split integration test")
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
	ah.DefaultStoreSlug = "central-cafe"
	orderRepo := postgres.NewOrderRepository()
	paymentSvc := service.NewPaymentService(db, postgres.NewPaymentRepository(), orderRepo, nil, service.WithShiftRepository(postgres.NewShiftRepository()))
	pos := handler.NewPOSHandler(service.NewOrderService(db, orderRepo), paymentSvc, service.NewSettingsService(db))
	sh := handler.NewShiftHandler(service.NewShiftService(db))

	r := gin.New()
	r.POST("/login", ah.Login)
	r.POST("/pos/drawers", ah.RequirePermission(rbac.CashDrawerManage), sh.CreateDrawer)
	r.POST("/pos/shifts", ah.RequirePermission(rbac.ShiftOpen), sh.OpenShift)
	r.GET("/pos/shifts/current", ah.RequireStaff(), sh.GetCurrentShift)
	r.POST("/pos/orders", ah.RequirePermission(rbac.OrderPOSCreate), pos.CreateOrder)
	r.POST("/pos/orders/:id/splits", ah.RequirePermission(rbac.OrderPOSCreate), pos.DivideOrder)
	r.POST("/pos/orders/:id/payments", ah.RequireStaff(), pos.InitiatePayment)

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
	login := func(email string) string {
		t.Helper()
		w := call(http.MethodPost, "/login", "", map[string]string{"email": email, "pin": "482913"})
		if w.Code != http.StatusOK {
			t.Fatalf("login %s: %d %s", email, w.Code, w.Body.String())
		}
		var out struct {
			AccessToken string `json:"access_token"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		return out.AccessToken
	}
	owner := login("abdulaziz543333@gmail.com")
	cashier := login("cashier.central@mycaffe.local")

	// Cash needs an open shift; reuse the cashier's if another test left one open.
	drawerID := ""
	if w := call(http.MethodGet, "/pos/shifts/current", cashier, nil); w.Code == http.StatusOK {
		var current domain.Shift
		_ = json.Unmarshal(w.Body.Bytes(), &current)
		if current.DrawerID != uuid.Nil {
			drawerID = current.DrawerID.String()
		}
	}
	if drawerID == "" {
		w := call(http.MethodPost, "/pos/drawers", owner, map[string]any{"name": "Laci Split " + uuid.NewString()[:8]})
		if w.Code != http.StatusCreated {
			t.Fatalf("create drawer: %d %s", w.Code, w.Body.String())
		}
		var drawer domain.CashDrawer
		_ = json.Unmarshal(w.Body.Bytes(), &drawer)
		drawerID = drawer.ID.String()
		if w := call(http.MethodPost, "/pos/shifts", cashier, map[string]any{"drawer_id": drawerID, "opening_float": 100000}); w.Code != http.StatusCreated {
			t.Fatalf("open shift: %d %s", w.Code, w.Body.String())
		}
	}

	createOrder := func() domain.Order {
		t.Helper()
		w := call(http.MethodPost, "/pos/orders", cashier, map[string]any{
			"idempotency_key": "split-ord-" + uuid.NewString()[:8],
			"customer_name":   "Rombongan Split",
			"order_type":      "dine_in",
			"items": []map[string]any{
				{
					"product_id": "d0000001-0000-4000-8000-000000000001", "quantity": 2,
					"modifiers": []map[string]any{
						{"option_id": "20000001-0000-4000-8000-000000000001", "quantity": 1},
						{"option_id": "20000003-0000-4000-8000-000000000003", "quantity": 1},
					},
				},
				{"product_id": "d0000003-0000-4000-8000-000000000003", "quantity": 1},
			},
		})
		if w.Code != http.StatusCreated {
			t.Fatalf("create order: %d %s", w.Code, w.Body.String())
		}
		var res struct {
			Order domain.Order `json:"order"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &res)
		return res.Order
	}
	kdsTickets := func(orderID uuid.UUID) int {
		t.Helper()
		var n int
		if err := database.WithTenantTx(ctx, db, storeID, func(tx *sqlx.Tx) error {
			return tx.GetContext(ctx, &n, `SELECT COUNT(*) FROM kds_tickets WHERE store_id = $1 AND order_id = $2`, storeID, orderID)
		}); err != nil {
			t.Fatal(err)
		}
		return n
	}
	type divided struct {
		Order  domain.Order          `json:"order"`
		Splits []domain.PaymentSplit `json:"splits"`
	}
	pay := func(orderID, splitID uuid.UUID, tendered int64) *httptest.ResponseRecorder {
		return call(http.MethodPost, "/pos/orders/"+orderID.String()+"/payments", cashier, map[string]any{
			"idempotency_key":      "split-pay-" + uuid.NewString()[:8],
			"method":               "cash",
			"split_id":             splitID.String(),
			"cash_tendered_amount": tendered,
			"drawer_id":            drawerID,
		})
	}

	t.Run("EqualSplitSettlesShareByShareAndHoldsKitchenUntilPaid", func(t *testing.T) {
		order := createOrder()
		w := call(http.MethodPost, "/pos/orders/"+order.ID.String()+"/splits", cashier, map[string]any{"strategy": "equal", "split_count": 3})
		if w.Code != http.StatusOK {
			t.Fatalf("divide: %d %s", w.Code, w.Body.String())
		}
		var res divided
		_ = json.Unmarshal(w.Body.Bytes(), &res)
		if len(res.Splits) != 3 {
			t.Fatalf("expected 3 shares, got %d", len(res.Splits))
		}
		var sum int64
		for _, sp := range res.Splits {
			sum += sp.TotalAmount
		}
		if sum != order.TotalAmount {
			t.Fatalf("server shares must add up to the order total: %d vs %d", sum, order.TotalAmount)
		}
		if again := call(http.MethodPost, "/pos/orders/"+order.ID.String()+"/splits", cashier, map[string]any{"strategy": "equal", "split_count": 2}); again.Code != http.StatusConflict || !strings.Contains(again.Body.String(), "SPLIT_NOT_ALLOWED") {
			t.Fatalf("dividing twice must be 409 SPLIT_NOT_ALLOWED, got %d %s", again.Code, again.Body.String())
		}

		for i, sp := range res.Splits {
			pw := pay(order.ID, sp.ID, sp.TotalAmount+5000)
			if pw.Code != http.StatusOK {
				t.Fatalf("pay share %d: %d %s", i+1, pw.Code, pw.Body.String())
			}
			var paid struct {
				Order domain.Order `json:"order"`
			}
			_ = json.Unmarshal(pw.Body.Bytes(), &paid)
			if i < len(res.Splits)-1 {
				if paid.Order.PaymentStatus != domain.OrderPaymentStatusPartiallyPaid {
					t.Fatalf("after share %d the order is partially paid, got %s", i+1, paid.Order.PaymentStatus)
				}
				if n := kdsTickets(order.ID); n != 0 {
					t.Fatalf("kitchen must wait for every share; %d tickets after share %d", n, i+1)
				}
				continue
			}
			if paid.Order.PaymentStatus != domain.OrderPaymentStatusPaid || paid.Order.PaidAmount != order.TotalAmount {
				t.Fatalf("last share settles the order: %+v", paid.Order)
			}
		}
		if n := kdsTickets(order.ID); n == 0 {
			t.Fatal("the kitchen must receive the order once fully paid")
		}
		if again := pay(order.ID, res.Splits[0].ID, 100000); again.Code == http.StatusOK {
			t.Fatal("a settled share must not be paid twice")
		}
	})

	t.Run("ByItemSplitPricesEachShareOnServer", func(t *testing.T) {
		order := createOrder()
		if len(order.Items) < 2 {
			t.Fatalf("expected two lines, got %+v", order.Items)
		}
		body := map[string]any{"strategy": "by_item", "splits": []map[string]any{
			{"label": "Andi", "items": []map[string]any{{"order_item_id": order.Items[0].ID, "quantity": order.Items[0].Quantity}}},
			{"label": "Sari", "items": []map[string]any{{"order_item_id": order.Items[1].ID, "quantity": order.Items[1].Quantity}}},
		}}
		w := call(http.MethodPost, "/pos/orders/"+order.ID.String()+"/splits", cashier, body)
		if w.Code != http.StatusOK {
			t.Fatalf("divide by item: %d %s", w.Code, w.Body.String())
		}
		var res divided
		_ = json.Unmarshal(w.Body.Bytes(), &res)
		if len(res.Splits) != 2 || res.Splits[0].Label != "Andi" || res.Splits[0].TotalAmount+res.Splits[1].TotalAmount != order.TotalAmount {
			t.Fatalf("unexpected shares: %+v (total %d)", res.Splits, order.TotalAmount)
		}

		other := createOrder()
		missing := map[string]any{"strategy": "by_item", "splits": []map[string]any{
			{"label": "X", "items": []map[string]any{{"order_item_id": other.Items[0].ID, "quantity": 1}}},
			{"label": "Y", "items": []map[string]any{}},
		}}
		if w := call(http.MethodPost, "/pos/orders/"+other.ID.String()+"/splits", cashier, missing); w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("an allocation that leaves items out must be 422, got %d %s", w.Code, w.Body.String())
		}
	})

	t.Run("RefusedAfterPaymentStartedAndForUnknownStrategy", func(t *testing.T) {
		order := createOrder()
		var single domain.PaymentSplit
		if err := database.WithTenantTx(ctx, db, storeID, func(tx *sqlx.Tx) error {
			return tx.GetContext(ctx, &single, `SELECT id, total_amount FROM payment_splits WHERE store_id = $1 AND order_id = $2`, storeID, order.ID)
		}); err != nil {
			t.Fatal(err)
		}
		if w := call(http.MethodPost, "/pos/orders/"+order.ID.String()+"/splits", cashier, map[string]any{"strategy": "thirds"}); w.Code != http.StatusBadRequest {
			t.Fatalf("unknown strategy must be 400, got %d", w.Code)
		}
		if w := call(http.MethodPost, "/pos/orders/"+uuid.NewString()+"/splits", cashier, map[string]any{"strategy": "equal", "split_count": 2}); w.Code != http.StatusNotFound {
			t.Fatalf("unknown order must be 404, got %d", w.Code)
		}
		whole := call(http.MethodPost, "/pos/orders/"+order.ID.String()+"/payments", cashier, map[string]any{
			"idempotency_key": "split-whole-" + uuid.NewString()[:8], "method": "cash",
			"cash_tendered_amount": single.TotalAmount, "drawer_id": drawerID,
		})
		if w := whole; w.Code != http.StatusOK {
			t.Fatalf("pay whole order: %d %s", w.Code, w.Body.String())
		}
		if w := call(http.MethodPost, "/pos/orders/"+order.ID.String()+"/splits", cashier, map[string]any{"strategy": "equal", "split_count": 2}); w.Code != http.StatusConflict {
			t.Fatalf("a paid order cannot be divided, got %d %s", w.Code, w.Body.String())
		}
	})
}
