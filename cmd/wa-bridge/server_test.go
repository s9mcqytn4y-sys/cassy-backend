package main

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeSender struct {
	ready bool
	err   error
	sent  []string
	docs  []string
}

func (f *fakeSender) Ready() bool { return f.ready }
func (f *fakeSender) Send(_ context.Context, phone, text string) error {
	if f.err != nil {
		return f.err
	}
	f.sent = append(f.sent, phone+"|"+text)
	return nil
}
func (f *fakeSender) SendDocument(_ context.Context, phone, filename string, pdfBytes []byte, caption string) error {
	if f.err != nil {
		return f.err
	}
	f.docs = append(f.docs, phone+"|"+filename+"|"+string(pdfBytes[:8])+"|"+caption)
	return nil
}

var testToken = strings.Repeat("t", 32)

func post(h http.Handler, auth, body string) int {
	req := httptest.NewRequest("POST", "/send", strings.NewReader(body))
	req.Header.Set("Authorization", auth)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w.Code
}

func TestSendHandler(t *testing.T) {
	f := &fakeSender{ready: true}
	h := newHandler(testToken, f)
	ok := `{"to":"+6281234567890","template":"struk_digital","params":["Kafe Uji","ORD-1","https://x/receipts/a"]}`

	if code := post(h, "Bearer wrong", ok); code != http.StatusUnauthorized {
		t.Fatalf("wrong token must be 401, got %d", code)
	}
	if code := post(h, "Bearer "+testToken, `{"to":"0812","template":"text","params":["hi"]}`); code != http.StatusBadRequest {
		t.Fatalf("non-E.164 number must be 400, got %d", code)
	}
	if code := post(h, "Bearer "+testToken, `{"to":"+6281234567890","template":"struk_digital","params":["a"]}`); code != http.StatusUnprocessableEntity {
		t.Fatalf("wrong parameter count must be 422, got %d", code)
	}
	if code := post(h, "Bearer "+testToken, ok); code != http.StatusNoContent {
		t.Fatalf("send must be 204, got %d", code)
	}
	want := "+6281234567890|Terima kasih sudah memesan di Kafe Uji via Cassy.\n\nNomor Pesanan: ORD-1\nStruk Digital: https://x/receipts/a\nUnduh PDF Struk: https://x/api/v1/public/receipts/a/pdf\n\nTautan berlaku 90 hari."
	if len(f.sent) != 1 || f.sent[0] != want {
		t.Fatalf("unexpected message %v, want %q", f.sent, want)
	}

	if code := post(h, "Bearer "+testToken, `{"to":"+6281234567890","template":"cassy_order_receipt","params":["Aziz","ORD-2","Cassy Senopati","Rp50.000","QRIS","https://x/receipts/b"]}`); code != http.StatusNoContent {
		t.Fatalf("6-param cassy_order_receipt must be 204, got %d", code)
	}
	if code := post(h, "Bearer "+testToken, `{"to":"+6281234567890","template":"tagihan_invoice","params":["Cassy Senopati","INV-1","Rp299.000","2026-12-01","Jatuh tempo"]}`); code != http.StatusNoContent {
		t.Fatalf("tagihan_invoice must be 204, got %d", code)
	}

	if code := post(h, "Bearer "+testToken, `{"to":"+6281234567890","template":"struk_digital","params":["Kafe Uji","ORD-3","https://x/r/Ab12Cd34"]}`); code != http.StatusNoContent {
		t.Fatalf("short URL struk_digital must be 204, got %d", code)
	}
	wantShort := "+6281234567890|Terima kasih sudah memesan di Kafe Uji via Cassy.\n\nNomor Pesanan: ORD-3\nStruk Digital: https://x/r/Ab12Cd34\nUnduh PDF Struk: https://x/api/v1/public/receipts/short/Ab12Cd34/pdf\n\nTautan berlaku 90 hari."
	if len(f.sent) < 2 || f.sent[len(f.sent)-1] != wantShort {
		t.Fatalf("unexpected short message %v, want %q", f.sent, wantShort)
	}

	pdfB64 := base64.StdEncoding.EncodeToString([]byte("%PDF-1.4 sample receipt"))
	if code := post(h, "Bearer "+testToken, `{"to":"+6281234567890","template":"struk_digital","params":["Kafe Uji","ORD-1","https://x/receipts/a"],"pdf_base64":"`+pdfB64+`","pdf_filename":"Struk-ORD-1.pdf"}`); code != http.StatusNoContent {
		t.Fatalf("document send must be 204, got %d", code)
	}
	if len(f.docs) != 1 || !strings.HasPrefix(f.docs[0], "+6281234567890|Struk-ORD-1.pdf|%PDF-1.4|") {
		t.Fatalf("unexpected doc %v", f.docs)
	}

	f.err = ErrNotOnWhatsApp
	if code := post(h, "Bearer "+testToken, ok); code != http.StatusUnprocessableEntity {
		t.Fatalf("a number without WhatsApp must be 422, got %d", code)
	}
	f.err = ErrNotReady
	if code := post(h, "Bearer "+testToken, ok); code != http.StatusServiceUnavailable {
		t.Fatalf("an unpaired bridge must be 503, got %d", code)
	}

	f.ready = false
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/healthz", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("healthz must be 503 until paired, got %d", w.Code)
	}
}
