package main

import (
	"fmt"
	"strings"
)

func receiptPDFURL(receiptURL string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(receiptURL), "/")
	if strings.Contains(trimmed, "/r/") {
		return strings.Replace(trimmed, "/r/", "/api/v1/public/receipts/short/", 1) + "/pdf"
	}
	if strings.Contains(trimmed, "/receipts/") {
		return strings.Replace(trimmed, "/receipts/", "/api/v1/public/receipts/", 1) + "/pdf"
	}
	return ""
}

// render turns a template name and its parameters into the text the demo sends. Template
// "struk_digital" mirrors the approved WhatsApp Cloud API template (decision W14) with clean
// multi-line breaks and direct PDF download link; "text" sends the first parameter as is.
func render(template string, params []string) (string, error) {
	switch template {
	case "struk_digital", "cassy_order_receipt":
		if len(params) == 3 {
			pdfURL := receiptPDFURL(params[2])
			if pdfURL != "" {
				return fmt.Sprintf(
					"Terima kasih sudah memesan di %s via Cassy.\n\n"+
						"Nomor Pesanan: %s\n"+
						"Struk Digital: %s\n"+
						"Unduh PDF Struk: %s\n\n"+
						"Tautan berlaku 90 hari.",
					params[0], params[1], params[2], pdfURL,
				), nil
			}
			return fmt.Sprintf(
				"Terima kasih sudah memesan di %s via Cassy.\n\n"+
					"Nomor Pesanan: %s\n"+
					"Struk Digital: %s\n\n"+
					"Tautan berlaku 90 hari.",
				params[0], params[1], params[2],
			), nil
		}
		if len(params) == 6 {
			pdfURL := receiptPDFURL(params[5])
			if pdfURL != "" {
				return fmt.Sprintf(
					"Halo %s, terima kasih sudah memesan di %s.\n\n"+
						"Nomor Pesanan: %s\n"+
						"Total Pembayaran: %s\n"+
						"Metode / Waktu: %s\n\n"+
						"Lihat Struk Digital:\n%s\n\n"+
						"Unduh PDF Struk Resmi:\n%s",
					params[0], params[2], params[1], params[3], params[4], params[5], pdfURL,
				), nil
			}
			return fmt.Sprintf(
				"Halo %s, terima kasih sudah memesan di %s.\n\n"+
					"Nomor Pesanan: %s\n"+
					"Total Pembayaran: %s\n"+
					"Metode / Waktu: %s\n\n"+
					"Lihat Struk Digital:\n%s",
				params[0], params[2], params[1], params[3], params[4], params[5],
			), nil
		}
		return "", fmt.Errorf("%s needs 3 or 6 parameters, got %d", template, len(params))
	case "tagihan_invoice":
		if len(params) != 5 {
			return "", fmt.Errorf("tagihan_invoice needs 5 parameters, got %d", len(params))
		}
		return fmt.Sprintf("[Cassy Billing] %s\nTagihan: %s\nNominal: %s\nJatuh Tempo: %s\nStatus: %s\n\nMohon abaikan bila sudah dibayar.",
			params[0], params[1], params[2], params[3], params[4]), nil
	case "text":
		if len(params) != 1 || strings.TrimSpace(params[0]) == "" {
			return "", fmt.Errorf("text needs 1 non-empty parameter")
		}
		return params[0], nil
	}
	return "", fmt.Errorf("unknown template %q", template)
}
