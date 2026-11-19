package service

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/domain"
)

// A promo or bundle line whose subtotal does not divide by its quantity must still be split to the rupiah.
func TestAllocateSplitsByItemSharesAnUnevenLineExactly(t *testing.T) {
	line := domain.OrderItem{ID: uuid.New(), Quantity: 3, SubtotalAmount: 10_000, SnapshotName: "Kopi promo"}
	order := &domain.Order{ID: uuid.New(), StoreID: uuid.New(), SubtotalAmount: 10_000, TaxAmount: 1_100, TotalAmount: 11_100}
	in := CloseTabInput{Splits: []SplitInput{
		{Items: []SplitItemAllocationInput{{OrderItemID: line.ID, Quantity: 1}}},
		{Items: []SplitItemAllocationInput{{OrderItemID: line.ID, Quantity: 2}}},
	}}

	splits, items, err := allocateSplits(order, []domain.OrderItem{line}, in, "by_item", time.Now())
	if err != nil {
		t.Fatalf("an uneven line must still split: %v", err)
	}
	if splits[0].SubtotalAmount != 3_333 || splits[1].SubtotalAmount != 6_667 {
		t.Fatalf("subtotals must be 3333 and 6667, got %d and %d", splits[0].SubtotalAmount, splits[1].SubtotalAmount)
	}
	if splits[0].TotalAmount+splits[1].TotalAmount != order.TotalAmount {
		t.Fatalf("shares must add up to the bill: %d + %d != %d", splits[0].TotalAmount, splits[1].TotalAmount, order.TotalAmount)
	}
	if items[0].AllocatedAmount+items[1].AllocatedAmount != line.SubtotalAmount {
		t.Fatal("allocated item amounts must add up to the line subtotal")
	}
}
