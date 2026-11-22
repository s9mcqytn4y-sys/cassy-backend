package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestFeatureDefaultsIncludeGuestV4Flags(t *testing.T) {
	v4Flags := []string{
		FeatureGuestSelfCancel,
		FeatureGuestSplitEqual,
		FeatureGuestSplitByItem,
		FeatureGuestSplitMultiPayment,
		FeatureGuestBellSound,
		FeatureCartPairing,
		FeatureGuestEmailReceipt,
	}
	for _, key := range v4Flags {
		enabled, ok := featureDefaults[key]
		if !ok {
			t.Fatalf("missing default for feature flag %q", key)
		}
		if !enabled {
			t.Fatalf("expected feature flag %q to default to true", key)
		}
	}
}

func TestPaySplitShareValidation(t *testing.T) {
	svc := &GuestService{}
	tests := []struct {
		name      string
		key       string
		method    string
		bank      string
		wantField string
	}{
		{name: "empty idempotency key", key: "", method: "qris", wantField: "idempotency_key"},
		{name: "unknown payment method", key: "k-1", method: "crypto", wantField: "method"},
		{name: "va missing bank", key: "k-2", method: "va", bank: "", wantField: "bank"},
		{name: "va unsupported bank", key: "k-3", method: "va", bank: "unknown", wantField: "bank"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.PaySplitShare(context.Background(), "token", tc.key, tc.method, tc.bank)
			var ve *ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("expected ValidationError, got %v", err)
			}
			if _, ok := ve.Fields[tc.wantField]; !ok {
				t.Fatalf("expected field error on %q, got %+v", tc.wantField, ve.Fields)
			}
		})
	}
}

func TestGuestCancelOrderReasonValidation(t *testing.T) {
	svc := &GuestService{}
	err := svc.CancelOrder(context.Background(), &GuestIdentity{StoreID: uuid.New(), SessionID: uuid.New()}, uuid.New(), strings.Repeat("a", 141))
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError for oversized reason, got %v", err)
	}
	if _, ok := ve.Fields["reason"]; !ok {
		t.Fatalf("expected field error on reason, got %+v", ve.Fields)
	}
}
