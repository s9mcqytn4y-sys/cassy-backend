package midtrans

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// S4-Q8: with an override every Core API request lands on the stub, path and query intact.
func TestUseAPIBaseURLRedirectsCoreAPICalls(t *testing.T) {
	var gotPath string
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.RequestURI()
		w.WriteHeader(http.StatusOK)
	}))
	defer stub.Close()
	t.Cleanup(func() { apiBaseOverride = nil })

	if err := UseAPIBaseURL(stub.URL); err != nil {
		t.Fatal(err)
	}
	res, err := sharedMidtransHTTPClient.Get("https://api.sandbox.midtrans.com/v2/order-1/status?x=1")
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if gotPath != "/v2/order-1/status?x=1" {
		t.Fatalf("stub saw %q", gotPath)
	}
	if err := UseAPIBaseURL("not a url"); err == nil {
		t.Fatal("a base without host must be refused")
	}
}
