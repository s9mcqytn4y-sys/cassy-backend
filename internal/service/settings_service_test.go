package service

import (
	"slices"
	"testing"
)

func TestNormalizeEnabledPaymentMethods(t *testing.T) {
	got, err := normalizeEnabledPaymentMethods([]string{" QRIS ", "gopay", "qris", "bank_transfer", "cash"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"qris", "gopay", "bank_transfer", "cash"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}

	if _, err := normalizeEnabledPaymentMethods(nil); err == nil {
		t.Fatal("expected empty payment method list to be rejected")
	}
	if _, err := normalizeEnabledPaymentMethods([]string{"qris", "bitcoin"}); err == nil {
		t.Fatal("expected unknown payment method to be rejected")
	}
}
