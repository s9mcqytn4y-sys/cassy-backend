package main

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"regexp"
	"strings"
)

// ErrNotOnWhatsApp marks a recipient that cannot receive WhatsApp messages (a permanent failure).
var ErrNotOnWhatsApp = errors.New("recipient is not on WhatsApp")

// ErrNotReady means the bridge is not paired or not connected yet (retry later).
var ErrNotReady = errors.New("bridge is not connected to WhatsApp")

// Sender sends text or PDF document messages to an E.164 phone number.
type Sender interface {
	Send(ctx context.Context, phone, text string) error
	SendDocument(ctx context.Context, phone, filename string, pdfBytes []byte, caption string) error
	Ready() bool
}

var e164 = regexp.MustCompile(`^\+[1-9][0-9]{7,14}$`)

type sendRequest struct {
	To          string   `json:"to"`
	Template    string   `json:"template"`
	Params      []string `json:"params"`
	PDFBase64   string   `json:"pdf_base64,omitempty"`
	PDFFilename string   `json:"pdf_filename,omitempty"`
}

// newHandler serves POST /send (bearer token) and GET /healthz. Response codes follow the API's
// driver contract: 4xx is permanent, 503 is retried.
func newHandler(token string, sender Sender) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		if !sender.Ready() {
			http.Error(w, "not connected", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("POST /send", func(w http.ResponseWriter, r *http.Request) {
		got, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var req sendRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10)).Decode(&req); err != nil || !e164.MatchString(req.To) {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		text, err := render(req.Template, req.Params)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
		var sendErr error
		if strings.TrimSpace(req.PDFBase64) != "" {
			pdfBytes, decErr := base64.StdEncoding.DecodeString(strings.TrimSpace(req.PDFBase64))
			if decErr != nil || len(pdfBytes) == 0 {
				http.Error(w, "invalid pdf_base64", http.StatusBadRequest)
				return
			}
			filename := strings.TrimSpace(req.PDFFilename)
			if filename == "" {
				filename = "Struk-Cassy.pdf"
			}
			sendErr = sender.SendDocument(r.Context(), req.To, filename, pdfBytes, text)
		} else {
			sendErr = sender.Send(r.Context(), req.To, text)
		}
		switch {
		case sendErr == nil:
			w.WriteHeader(http.StatusNoContent)
		case errors.Is(sendErr, ErrNotOnWhatsApp):
			http.Error(w, "recipient is not on WhatsApp", http.StatusUnprocessableEntity)
		case errors.Is(sendErr, ErrNotReady):
			http.Error(w, "bridge not connected", http.StatusServiceUnavailable)
		default:
			// The phone number never reaches the log (personal data).
			log.Printf("[WA BRIDGE] send failed: %v", sendErr)
			http.Error(w, "send failed", http.StatusServiceUnavailable)
		}
	})
	return mux
}
