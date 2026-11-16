package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/service"
)

func TestPlanLimitErrorMapsTo402(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limit := fmt.Errorf("tx: %w", &service.PlanLimitError{Resource: service.PlanResourceProducts, Limit: 100, PlanCode: "starter_umkm"})
	for name, write := range map[string]func(*gin.Context, error){"writeServiceError": writeServiceError, "lifecycleError": lifecycleError} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		write(c, limit)
		var body struct {
			Code     string `json:"code"`
			Error    string `json:"error"`
			Resource string `json:"resource"`
			Limit    int    `json:"limit"`
			PlanCode string `json:"plan_code"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if w.Code != http.StatusPaymentRequired || body.Code != "PLAN_LIMIT_REACHED" || body.Resource != "products" ||
			body.Limit != 100 || body.PlanCode != "starter_umkm" || !strings.Contains(body.Error, "100 menu aktif") {
			t.Fatalf("%s: %d %+v", name, w.Code, body)
		}
	}
}

func TestPlanResourceLabelsCoverEveryResource(t *testing.T) {
	for _, r := range []service.PlanResource{service.PlanResourceProducts, service.PlanResourceStaff,
		service.PlanResourceDevices, service.PlanResourceTables, service.PlanResourcePrinters} {
		if planResourceLabels[r] == "" {
			t.Fatalf("no owner-facing label for %s", r)
		}
	}
}

// TestWriteCatalogLeavesSharedViewIntact: the public view is shared by concurrent requests, so the
// writer must never touch it (run with -race).
func TestWriteCatalogLeavesSharedViewIntact(t *testing.T) {
	gin.SetMode(gin.TestMode)
	generated := time.Date(2026, 11, 16, 9, 0, 0, 0, time.UTC)
	shared := &service.CatalogView{GeneratedAt: generated}
	var wg sync.WaitGroup
	etags := make([]string, 16)
	for i := range etags {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
			writeCatalog(c, shared, "public, max-age=30")
			etags[i] = w.Header().Get("ETag")
			if !strings.Contains(w.Body.String(), "2026-11-16T09:00:00Z") {
				t.Errorf("body keeps generated_at: %s", w.Body.String())
			}
		}(i)
	}
	wg.Wait()
	if !shared.GeneratedAt.Equal(generated) {
		t.Fatal("writeCatalog mutated the shared view")
	}
	for _, e := range etags {
		if e == "" || e != etags[0] {
			t.Fatalf("one content, one ETag: %v", etags)
		}
	}
}

func TestQuoteResponsesAreNeverCached(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewPOSHandler(nil, nil, nil)
	r := gin.New()
	r.POST("/pos/orders/quote", h.QuoteOrder)
	r.POST("/public/stores/:slug/orders/quote", h.PublicQuoteOrder)
	for _, path := range []string{"/pos/orders/quote", "/public/stores/kopi/orders/quote"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, path, strings.NewReader("{")))
		if w.Code != http.StatusBadRequest || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("%s: even errors are no-store, got %d %q", path, w.Code, w.Header().Get("Cache-Control"))
		}
	}
}

func TestPairingTokenInvalidMapsTo401(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	writeDeviceError(c, fmt.Errorf("wrap: %w", service.ErrPairingTokenInvalid))
	if w.Code != http.StatusUnauthorized || !strings.Contains(w.Body.String(), "PAIRING_TOKEN_INVALID") {
		t.Fatalf("got %d %s", w.Code, w.Body.String())
	}
	if errors.Is(service.ErrPairingTokenInvalid, service.ErrDeviceInvalid) {
		t.Fatal("pairing and device credential errors stay distinct")
	}
}
