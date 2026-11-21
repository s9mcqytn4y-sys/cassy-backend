package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestPairDeviceRejectsMalformedTokenBeforeDatabase(t *testing.T) {
	s := &DeviceService{} // no database: a malformed token must fail during parsing
	for _, tok := range []string{"", "abc", "a.b.c", uuid.NewString() + "." + uuid.NewString()} {
		if _, err := s.PairDevice(context.Background(), tok); !errors.Is(err, ErrPairingTokenInvalid) {
			t.Fatalf("%q: got %v", tok, err)
		}
	}
}

func TestPairingTokenTTLMatchesDatabaseCap(t *testing.T) {
	if PairingTokenTTL.Minutes() != 5 {
		t.Fatalf("SecurityQrPairingAdr fixes the TTL at 5 minutes, got %s", PairingTokenTTL)
	}
}
