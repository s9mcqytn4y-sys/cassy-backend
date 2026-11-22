package service

import (
	"strings"
	"testing"
	"time"

	"github.com/s9mcqytn4y-sys/cassy-backend/internal/pkg/mailer"
)

func TestReceiptEmailDataCarriesLinesTotalsAndLink(t *testing.T) {
	r := &Receipt{
		OrderNumber: "A-014",
		TableNumber: "M04",
		IssuedAt:    time.Date(2026, 11, 11, 3, 0, 0, 0, time.UTC),
		Lines: []ReceiptLine{
			{Name: "Classic Iced Latte", Variant: "Besar", Options: []string{"Oat Milk Barista Blend"}, Quantity: 2, Amount: 76000},
			{Name: "Croissant", Quantity: 1, Amount: 25000},
		},
		Subtotal: 101000, Service: 5050, Tax: 10100, Total: 116150,
		Payments: []ReceiptPayment{{Method: "gopay", Status: "paid"}, {Method: "gopay", Status: "paid"}},
	}
	link := &ReceiptLink{URL: "https://demo.test/r/Ab12Cd34", ShortCode: "Ab12Cd34", PDFURL: "https://demo.test/api/v1/public/receipts/short/Ab12Cd34/pdf", ExpiresAt: time.Date(2027, 2, 9, 0, 0, 0, 0, time.UTC)}

	d := receiptEmailData(r, link)
	if d.PaymentMethod != "GoPay" || d.DiscountLabel != "" || d.ServiceFeeLabel == "" || len(d.Items) != 2 || d.TableNumber != "M04" {
		t.Fatalf("unexpected data: %+v", d)
	}
	if d.Items[0].Details != "Besar, Oat Milk Barista Blend" || d.Items[1].Details != "" {
		t.Fatalf("details must list the size then the add-ons: %+v", d.Items)
	}

	d.StoreName = "Kopi Senja"
	d.StoreSlug = "kopi-senja"
	out, err := mailer.Render(mailer.TemplateReceipt, d)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Classic Iced Latte", "Oat Milk Barista Blend", rupiah(116150), "GoPay", "Meja M04",
		"https://demo.test/r/Ab12Cd34", "https://demo.test/api/v1/public/receipts/short/Ab12Cd34/pdf",
		"https://demo.test/legal", "tabular-nums", "Lihat Struk &amp; Unduh PDF", "Cek Status Pesanan",
		"Ditenagai Cassy oleh AzureDevID - Patuh UU PDP No. 27/2022", "#F7F5F0", "#FFFFFF", "#1F5F52",
	} {
		if !strings.Contains(out.HTML, want) {
			t.Fatalf("receipt html is missing %q", want)
		}
	}
	for _, forbidden := range []string{"prefers-color-scheme: dark", "email-slug-pill", "kopi-senja"} {
		if strings.Contains(out.HTML, forbidden) {
			t.Fatalf("receipt html must not contain %q", forbidden)
		}
	}
	for _, wantText := range []string{"Meja M04", "Classic Iced Latte", rupiah(116150), "https://demo.test/r/Ab12Cd34", "Ditenagai Cassy oleh AzureDevID - Patuh UU PDP No. 27/2022"} {
		if !strings.Contains(out.Text, wantText) {
			t.Fatalf("receipt text is missing %q", wantText)
		}
	}
}

func TestShortCodeGenerationAndValidation(t *testing.T) {
	code, err := randomShortCode(8)
	if err != nil {
		t.Fatal(err)
	}
	if len(code) != 8 || !isValidShortCode(code) {
		t.Fatalf("invalid generated short code %q", code)
	}
	if isValidShortCode("../etc") || isValidShortCode("abc") {
		t.Fatal("expected invalid short code rejection")
	}
}
