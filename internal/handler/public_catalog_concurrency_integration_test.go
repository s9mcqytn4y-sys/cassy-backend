package handler_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/config"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/handler"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/pkg/database"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/service"
)

// TestIntegrationPublicCatalogConcurrent hits the guest menu from many goroutines at once (PM-E6):
// every caller gets the same content and ETag, and the shared view is not mutated under -race.
func TestIntegrationPublicCatalogConcurrent(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_APP_DATABASE_URL"))
	if dsn == "" {
		t.Skip("TEST_APP_DATABASE_URL not set; skipping public catalog concurrency integration test")
	}
	db, err := database.NewPostgresDB(&config.Config{DatabaseURL: dsn})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	gin.SetMode(gin.TestMode)
	ch := handler.NewCatalogHandler(service.NewCatalogService(db))
	r := gin.New()
	r.GET("/public/stores/:slug/catalog", ch.PublicCatalog)

	const n = 32
	var wg sync.WaitGroup
	etags, codes := make([]string, n), make([]int, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/public/stores/central-cafe/catalog", nil))
			codes[i], etags[i] = w.Code, w.Header().Get("ETag")
			if cc := w.Header().Get("Cache-Control"); cc != "public, max-age=30, stale-while-revalidate=30" {
				t.Errorf("cache-control: %q", cc)
			}
		}(i)
	}
	wg.Wait()
	for i := range codes {
		if codes[i] != http.StatusOK || etags[i] == "" || etags[i] != etags[0] {
			t.Fatalf("all concurrent callers get 200 and one ETag: codes=%v etags=%v", codes, etags)
		}
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/public/stores/central-cafe/catalog", nil)
	req.Header.Set("If-None-Match", etags[0])
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotModified {
		t.Fatalf("revalidation with the shared ETag is 304, got %d", w.Code)
	}
}
