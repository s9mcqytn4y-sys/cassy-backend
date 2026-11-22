package service

import (
	"testing"

	"github.com/google/uuid"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/domain"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/repository/postgres"
)

func TestBuildCatalogViewSmartPairingAndBadges(t *testing.T) {
	storeID := uuid.New()
	catID := uuid.New()
	coffeeID := uuid.New()
	croissantID := uuid.New()
	cheesecakeID := uuid.New()
	badge := "Favorit"
	prep := 4

	store := &domain.Store{ID: storeID, Slug: "senopati", Name: "Senopati", Currency: "IDR"}
	rows := &postgres.CatalogRows{
		Categories: []domain.Category{{ID: catID, StoreID: storeID, Name: "Kopi", IsActive: true}},
		Products: []domain.Product{
			{ID: coffeeID, StoreID: storeID, CategoryID: &catID, Name: "Iced Latte", ProductType: "prepared", PrepStation: "bar", BadgeLabel: &badge, PrepMinutes: &prep, IsAvailable: true},
			{ID: croissantID, StoreID: storeID, CategoryID: &catID, Name: "Croissant", ProductType: "ready_to_serve", PrepStation: "kitchen", IsAvailable: true},
			{ID: cheesecakeID, StoreID: storeID, CategoryID: &catID, Name: "Cheesecake", ProductType: "ready_to_serve", PrepStation: "kitchen", IsAvailable: true},
		},
		Variants: []domain.ProductVariant{
			{ID: uuid.New(), StoreID: storeID, ProductID: coffeeID, Name: "Reguler", Price: 24000, IsDefault: true, IsAvailable: true},
			{ID: uuid.New(), StoreID: storeID, ProductID: croissantID, Name: "Reguler", Price: 24000, IsDefault: true, IsAvailable: true},
			{ID: uuid.New(), StoreID: storeID, ProductID: cheesecakeID, Name: "Reguler", Price: 42000, IsDefault: true, IsAvailable: true},
		},
		Recommendations: []domain.ProductRecommendation{
			{StoreID: storeID, ProductID: coffeeID, RecommendedProductID: croissantID, SortOrder: 1},
			{StoreID: storeID, ProductID: coffeeID, RecommendedProductID: cheesecakeID, SortOrder: 2},
		},
	}

	view := buildCatalogView(store, domain.StoreSettings{}, rows, false)
	if len(view.Products) != 3 {
		t.Fatalf("expected 3 products, got %d", len(view.Products))
	}
	coffee := view.Products[0]
	if coffee.BadgeLabel == nil || *coffee.BadgeLabel != "Favorit" {
		t.Fatalf("expected badge_label Favorit, got %v", coffee.BadgeLabel)
	}
	if coffee.PrepMinutes == nil || *coffee.PrepMinutes != 4 {
		t.Fatalf("expected prep_minutes 4, got %v", coffee.PrepMinutes)
	}
	if len(coffee.RecommendedProductIDs) != 2 || coffee.RecommendedProductIDs[0] != croissantID || coffee.RecommendedProductIDs[1] != cheesecakeID {
		t.Fatalf("unexpected recommended_product_ids: %+v", coffee.RecommendedProductIDs)
	}
}
