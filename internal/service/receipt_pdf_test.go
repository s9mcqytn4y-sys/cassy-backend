package service

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestReceiptPDF(t *testing.T) {
	r := &Receipt{
		Kind:        ReceiptKindSale,
		Store:       ReceiptStore{Name: "Cassy Roastery Senopati", Address: "Jl. Senopati No. 10, Jakarta Selatan", Phone: "+6281234567890", Timezone: "Asia/Jakarta"},
		OrderID:     uuid.New(),
		OrderNumber: "ORD-20261115-001",
		QueueNumber: "A-001",
		TableNumber: "M-04",
		OrderType:   "dine_in",
		Status:      "completed",
		PaymentStat: "paid",
		IssuedAt:    time.Date(2026, 11, 15, 10, 15, 0, 0, time.UTC),
		LocalTime:   "15/11/2026 17:15",
		Lines: []ReceiptLine{
			{Name: "Es Kopi Susu Pandan", Variant: "Regular", Options: []string{"Less Sugar", "Oat Milk"}, Quantity: 2, UnitPrice: 28000, Amount: 56000},
			{Name: "Croissant Butter", Quantity: 1, UnitPrice: 24000, Amount: 24000},
		},
		Subtotal: 80000,
		Service:  4000,
		Tax:      8400,
		Total:    92400,
		Paid:     92400,
		Payments: []ReceiptPayment{
			{Method: "qris", Amount: 92400, Status: "paid"},
		},
	}
	pdf := RenderReceiptPDF(r, "https://demo.abbdullaziz.online/receipts/tok")
	if !bytes.HasPrefix(pdf, []byte("%PDF-1.4\n")) {
		t.Fatalf("expected PDF-1.4 header, got %q", pdf[:16])
	}
	body := string(pdf)
	for _, want := range []string{"Cassy Roastery Senopati", "ORD-20261115-001", "Es Kopi Susu Pandan", "Rp92.400", "STRUK DIGITAL RESMI"} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected PDF to contain %q", want)
		}
	}
}
