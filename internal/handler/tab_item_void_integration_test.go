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

// TestTabItemVoidIntegration covers POST /pos/tabs/:id/items/:itemId/void:
// a small void on the cashier's own PIN shrinks the line, returns stock and recalculates the tab;
// a void above the owner threshold needs an approved void_item request bound to the line and quantity;
// voiding the whole line removes it; a closed tab refuses item voids.
func TestTabItemVoidIntegration(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_APP_DATABASE_URL"))
	migratorDSN := strings.TrimSpace(os.Getenv("TEST_MIGRATOR_DATABASE_URL"))
	if dsn == "" || migratorDSN == "" {
		t.Skip("TEST_APP_DATABASE_URL / TEST_MIGRATOR_DATABASE_URL not set; skipping tab item void integration test")
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
	slug := "tabvoid-" + suffix
	email := func(name string) string { return name + ".tabvoid." + suffix + "@mycaffe.local" }
	storeID, ownerID, err := service.BootstrapStore(ctx, migratorDB, service.BootstrapStoreInput{
		Slug: slug, StoreName: "Kafe Void Item", OwnerName: "Pemilik Void", OwnerEmail: email("owner"), PIN: "482913",
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
	orderRepo := postgres.NewOrderRepository()
	paymentRepo := postgres.NewPaymentRepository()
	ops := service.NewOperationalService(db, orderRepo, paymentRepo, nil, nil, nil)
	pos := handler.NewPOSHandler(service.NewOrderService(db, orderRepo), service.NewPaymentService(db, paymentRepo, orderRepo, nil), service.NewSettingsService(db))
	pos.SetOperational(ops)
	th := handler.NewTabHandler(service.NewTabService(db, postgres.NewTabRepository(), orderRepo))
	aph := handler.NewApprovalHandler(service.NewApprovalService(db, ops))

	r := gin.New()
	r.POST("/login", ah.Login)
	r.POST("/pos/tabs", ah.RequirePermission(rbac.TabManage), th.OpenTab)
	r.POST("/pos/tabs/:id/rounds", ah.RequirePermission(rbac.TabManage), th.SubmitRound)
	r.POST("/pos/tabs/:id/close", ah.RequirePermission(rbac.TabManage), th.CloseTab)
	r.POST("/pos/tabs/:id/items/:itemId/void", ah.RequirePermission(rbac.OrderVoid), pos.VoidTabItem)
	r.POST("/pos/approvals", ah.RequireStaff(), aph.Create)
	r.POST("/approvals/:id/approve", ah.RequirePermission(rbac.ApprovalDecide), aph.Approve)

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
	login := func(who string) string {
		t.Helper()
		res := call("POST", "/login", "", map[string]string{"store_slug": slug, "email": email(who), "pin": "482913"})
		if res.Code != http.StatusOK {
			t.Fatalf("login %s failed: %d: %s", who, res.Code, res.Body.String())
		}
		var body struct {
			AccessToken string `json:"access_token"`
		}
		_ = json.Unmarshal(res.Body.Bytes(), &body)
		return body.AccessToken
	}
	code := func(res *httptest.ResponseRecorder) string {
		var b struct {
			Code string `json:"code"`
		}
		_ = json.Unmarshal(res.Body.Bytes(), &b)
		return b.Code
	}

	priceyID, croissantID := uuid.New(), uuid.New()
	if err := database.WithTenantTx(ctx, db, storeID, func(tx *sqlx.Tx) error {
		if _, err := tx.ExecContext(ctx, `INSERT INTO users (store_id, email, full_name, role, pin_hash, is_active, pin_set_at)
			SELECT store_id, $2, 'cashier', 'cashier', pin_hash, true, NOW() FROM users WHERE store_id = $1 AND id = $3`,
			storeID, email("cashier"), ownerID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `WITH p AS (INSERT INTO products (id, store_id, name, base_price, is_available) VALUES ($1, $2, 'Paket Besar', 0, true) RETURNING store_id, id)
			INSERT INTO product_variants (store_id, product_id, name, price, is_default) SELECT store_id, id, 'Reguler', 60000, true FROM p`, priceyID, storeID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `WITH p AS (INSERT INTO products (id, store_id, name, base_price, is_available, track_stock, stock_quantity) VALUES ($1, $2, 'Croissant', 0, true, true, 10) RETURNING store_id, id)
			INSERT INTO product_variants (store_id, product_id, name, price, is_default) SELECT store_id, id, 'Reguler', 15000, true FROM p`, croissantID, storeID)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	owner, cashier := login("owner"), login("cashier")

	stockOf := func(productID uuid.UUID) int {
		t.Helper()
		var qty int
		if err := database.WithTenantTx(ctx, db, storeID, func(tx *sqlx.Tx) error {
			return tx.GetContext(ctx, &qty, `SELECT stock_quantity FROM products WHERE store_id = $1 AND id = $2`, storeID, productID)
		}); err != nil {
			t.Fatal(err)
		}
		return qty
	}

	openRes := call("POST", "/pos/tabs", cashier, map[string]any{"idempotency_key": "tv-open-" + suffix, "table_number": "M-07"})
	if openRes.Code != http.StatusCreated {
		t.Fatalf("open tab: %d: %s", openRes.Code, openRes.Body.String())
	}
	var opened service.OpenTabResult
	_ = json.Unmarshal(openRes.Body.Bytes(), &opened)
	tabID := opened.Order.ID
	roundRes := call("POST", fmt.Sprintf("/pos/tabs/%s/rounds", tabID), cashier, map[string]any{
		"idempotency_key": "tv-round-" + suffix,
		"items":           []map[string]any{{"product_id": croissantID, "quantity": 3}, {"product_id": priceyID, "quantity": 3}},
	})
	if roundRes.Code != http.StatusCreated {
		t.Fatalf("submit round: %d: %s", roundRes.Code, roundRes.Body.String())
	}
	var round service.SubmitRoundResult
	_ = json.Unmarshal(roundRes.Body.Bytes(), &round)
	itemOf := func(productID uuid.UUID) domain.OrderItem {
		for _, it := range round.Round.Items {
			if it.ProductID != nil && *it.ProductID == productID {
				return it
			}
		}
		t.Fatalf("round has no item for %s", productID)
		return domain.OrderItem{}
	}
	croissantLine, priceyLine := itemOf(croissantID), itemOf(priceyID)
	if got := stockOf(croissantID); got != 7 {
		t.Fatalf("round must take 3 croissants from stock 10, left %d", got)
	}

	voidPath := func(itemID uuid.UUID) string { return fmt.Sprintf("/pos/tabs/%s/items/%s/void", tabID, itemID) }

	t.Run("SmallVoidOnCashierPinShrinksLineAndReturnsStock", func(t *testing.T) {
		res := call("POST", voidPath(croissantLine.ID), cashier, map[string]any{"quantity": 1, "reason": "Salah input", "pin": "482913"})
		if res.Code != http.StatusOK {
			t.Fatalf("void 1 croissant: %d: %s", res.Code, res.Body.String())
		}
		var out service.VoidTabItemResult
		_ = json.Unmarshal(res.Body.Bytes(), &out)
		if out.RemainingQuantity != 2 || !out.StockReturned {
			t.Fatalf("expected 2 left and stock returned, got %+v", out)
		}
		if out.Order.SubtotalAmount != 2*15000+3*60000 {
			t.Fatalf("subtotal after void = %d", out.Order.SubtotalAmount)
		}
		if got := stockOf(croissantID); got != 8 {
			t.Fatalf("voided unseen croissant must return to stock, have %d", got)
		}
	})

	t.Run("TooManyUnitsIsRejected", func(t *testing.T) {
		res := call("POST", voidPath(croissantLine.ID), cashier, map[string]any{"quantity": 5, "reason": "Salah input", "pin": "482913"})
		if res.Code != http.StatusUnprocessableEntity && res.Code != http.StatusBadRequest {
			t.Fatalf("voiding more than the line holds must fail validation: %d: %s", res.Code, res.Body.String())
		}
	})

	t.Run("VoidAboveThresholdNeedsBoundOwnerApproval", func(t *testing.T) {
		res := call("POST", voidPath(priceyLine.ID), cashier, map[string]any{"quantity": 2, "reason": "Tamu batal", "pin": "482913"})
		if code(res) != "OWNER_PIN_REQUIRED" {
			t.Fatalf("Rp120.000 void must need the owner: %d: %s", res.Code, res.Body.String())
		}
		reqRes := call("POST", "/pos/approvals", cashier, map[string]any{
			"action": "void_item", "reference_id": priceyLine.ID, "reason": "Tamu batal", "params": map[string]any{"quantity": 2},
		})
		if reqRes.Code != http.StatusCreated {
			t.Fatalf("request approval: %d: %s", reqRes.Code, reqRes.Body.String())
		}
		var a service.Approval
		_ = json.Unmarshal(reqRes.Body.Bytes(), &a)
		if a.Amount != 120000 || !strings.Contains(a.ReferenceLabel, "Paket Besar") {
			t.Fatalf("owner must see the line and its value: amount=%d label=%q", a.Amount, a.ReferenceLabel)
		}
		if res := call("POST", fmt.Sprintf("/approvals/%s/approve", a.ID), owner, map[string]any{"pin": "482913"}); res.Code != http.StatusOK {
			t.Fatalf("approve: %d: %s", res.Code, res.Body.String())
		}
		// The approval is bound to quantity 2: a void of all 3 units (Rp180.000) cannot use it.
		if res := call("POST", voidPath(priceyLine.ID), cashier, map[string]any{"quantity": 3, "reason": "Tamu batal", "pin": "482913", "approval_id": a.ID}); code(res) != "APPROVAL_INVALID" {
			t.Fatalf("approval for 2 units must not cover 3: %d: %s", res.Code, res.Body.String())
		}
		res = call("POST", voidPath(priceyLine.ID), cashier, map[string]any{"quantity": 2, "reason": "Tamu batal", "pin": "482913", "approval_id": a.ID})
		if res.Code != http.StatusOK {
			t.Fatalf("void with approval: %d: %s", res.Code, res.Body.String())
		}
		var out service.VoidTabItemResult
		_ = json.Unmarshal(res.Body.Bytes(), &out)
		if out.RemainingQuantity != 1 || out.Order.SubtotalAmount != 2*15000+60000 {
			t.Fatalf("2 of 3 units must be gone: %+v subtotal=%d", out, out.Order.SubtotalAmount)
		}
		var audits int
		if err := database.WithTenantTx(ctx, db, storeID, func(tx *sqlx.Tx) error {
			return tx.GetContext(ctx, &audits, `SELECT COUNT(*) FROM audit_logs WHERE store_id = $1 AND action = 'VOID_ORDER_ITEM'`, storeID)
		}); err != nil {
			t.Fatal(err)
		}
		if audits != 2 {
			t.Fatalf("expected 2 VOID_ORDER_ITEM audit rows so far, got %d", audits)
		}
	})

	t.Run("VoidingTheLastUnitsRemovesTheLine", func(t *testing.T) {
		res := call("POST", voidPath(priceyLine.ID), cashier, map[string]any{"quantity": 1, "reason": "Tamu batal", "pin": "482913"})
		if res.Code != http.StatusOK {
			t.Fatalf("void last unit: %d: %s", res.Code, res.Body.String())
		}
		var out service.VoidTabItemResult
		_ = json.Unmarshal(res.Body.Bytes(), &out)
		if out.RemainingQuantity != 0 || orderHasProduct(out.Order, priceyID) {
			t.Fatalf("line must be removed: %+v", out)
		}
	})

	t.Run("ClosedTabRefusesItemVoid", func(t *testing.T) {
		if res := call("POST", fmt.Sprintf("/pos/tabs/%s/close", tabID), cashier, map[string]any{}); res.Code != http.StatusOK {
			t.Fatalf("close tab: %d: %s", res.Code, res.Body.String())
		}
		res := call("POST", voidPath(croissantLine.ID), cashier, map[string]any{"quantity": 1, "reason": "Telat", "pin": "482913"})
		if code(res) != "TAB_NOT_OPEN" {
			t.Fatalf("closed tab must refuse item void: %d: %s", res.Code, res.Body.String())
		}
	})

	t.Run("AppRoleCannotDeleteLinesOfAClosedTab", func(t *testing.T) {
		var deleted int64
		if err := database.WithTenantTx(ctx, db, storeID, func(tx *sqlx.Tx) error {
			res, err := tx.ExecContext(ctx, `DELETE FROM order_items WHERE store_id = $1 AND id = $2`, storeID, croissantLine.ID)
			if err != nil {
				return err
			}
			deleted, err = res.RowsAffected()
			return err
		}); err != nil {
			t.Fatal(err)
		}
		if deleted != 0 {
			t.Fatalf("restrictive policy must keep lines of a closed tab, deleted %d", deleted)
		}
	})
}

func orderHasProduct(order *domain.Order, productID uuid.UUID) bool {
	for _, it := range order.Items {
		if it.ProductID != nil && *it.ProductID == productID {
			return true
		}
	}
	return false
}
