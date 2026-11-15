package service

import (
	"context"
	"fmt"
	"slices"
	"strings"
)

// RenderReceiptPDF renders a structured A4 digital receipt PDF (%PDF-1.4) from an immutable order receipt snapshot.
func RenderReceiptPDF(r *Receipt, receiptURL string) []byte {
	row := func(left, right string) string { return fmt.Sprintf("%-50.50s %27.27s", left, right) }
	sep := strings.Repeat("-", 78)
	doubleSep := strings.Repeat("=", 78)

	lines := []string{
		doubleSep,
		"CASSY POS & QR ORDERING - STRUK DIGITAL RESMI",
		r.Store.Name,
	}
	if r.Store.Address != "" {
		lines = append(lines, r.Store.Address)
	}
	if r.Store.Phone != "" {
		lines = append(lines, "Telp: "+r.Store.Phone)
	}
	lines = append(lines,
		doubleSep,
		fmt.Sprintf("Nomor Pesanan  : %s", r.OrderNumber),
		fmt.Sprintf("Nomor Antrean  : %s", r.QueueNumber),
	)
	if r.TableNumber != "" {
		lines = append(lines, fmt.Sprintf("Nomor Meja     : %s", r.TableNumber))
	}
	lines = append(lines,
		fmt.Sprintf("Waktu Transaksi: %s", r.LocalTime),
		fmt.Sprintf("Status Bayar   : %s", strings.ToUpper(r.PaymentStat)),
		sep,
		row("Item Pesanan", "Subtotal"),
		sep,
	)

	for _, l := range r.Lines {
		name := fmt.Sprintf("%dx %s", l.Quantity, l.Name)
		if l.Variant != "" {
			name += " (" + l.Variant + ")"
		}
		lines = append(lines, row(name, rupiah(l.Amount)))
		if len(l.Options) > 0 {
			lines = append(lines, "   + "+strings.Join(l.Options, ", "))
		}
		if l.Notes != "" {
			lines = append(lines, "   Catatan: "+l.Notes)
		}
		if l.PromoDiscount > 0 {
			lines = append(lines, row("   Promo "+l.PromoName, "-"+rupiah(l.PromoDiscount)))
		}
	}

	lines = append(lines,
		sep,
		row("Subtotal Produk", rupiah(r.Subtotal)),
	)
	if r.Discount > 0 {
		lines = append(lines, row("Potongan / Diskon", "-"+rupiah(r.Discount)))
	}
	if r.Service > 0 {
		lines = append(lines, row("Biaya Layanan (Service)", rupiah(r.Service)))
	}
	if r.Tax > 0 {
		lines = append(lines, row("Pajak Restoran (PB1)", rupiah(r.Tax)))
	}
	lines = append(lines,
		doubleSep,
		row("TOTAL PEMBAYARAN", rupiah(r.Total)),
		doubleSep,
	)

	methods := []string{}
	for _, p := range r.Payments {
		if label := methodLabel(p.Method); !slices.Contains(methods, label) {
			methods = append(methods, label)
		}
	}
	if len(methods) > 0 {
		lines = append(lines, fmt.Sprintf("Metode Pembayaran : %s", strings.Join(methods, ", ")))
	}
	if r.Refunded > 0 {
		lines = append(lines, fmt.Sprintf("Total Dikembalikan: %s", rupiah(r.Refunded)))
	}
	if receiptURL != "" {
		lines = append(lines, "", "Tautan Struk Digital (berlaku 90 hari):", receiptURL)
	}
	lines = append(lines,
		"",
		"Struk digital ini adalah bukti pembayaran sah yang diterbitkan secara",
		"elektronik melalui sistem Cassy by AzureDevID.",
	)
	return renderTextPDF(lines)
}

// PublicPDF resolves a public receipt token and returns both the receipt metadata and its rendered PDF bytes.
func (s *ReceiptService) PublicPDF(ctx context.Context, token string) (*Receipt, []byte, error) {
	r, err := s.Public(ctx, token)
	if err != nil {
		return nil, nil, err
	}
	return r, RenderReceiptPDF(r, s.LinkURL(token)), nil
}

// PDFURL returns the public PDF download endpoint for a receipt token.
func (s *ReceiptService) PDFURL(token string) string {
	return s.publicURL + "/api/v1/public/receipts/" + token + "/pdf"
}
