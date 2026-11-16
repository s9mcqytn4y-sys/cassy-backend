package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestPlanQuotasCoverEveryResource(t *testing.T) {
	for _, r := range []PlanResource{PlanResourceProducts, PlanResourceStaff, PlanResourceDevices, PlanResourceTables, PlanResourcePrinters} {
		q, ok := planQuotas[r]
		if !ok || !strings.HasPrefix(q.limitColumn, "max_") || !strings.Contains(q.countSQL, "store_id = $1") {
			t.Fatalf("%s: quota must name a max_ column and count per store: %+v", r, q)
		}
	}
}

func TestEnsurePlanQuotaRejectsUnknownResource(t *testing.T) {
	// The resource is checked before the transaction is touched.
	if err := ensurePlanQuota(context.Background(), nil, uuid.New(), "tablets"); err == nil || !strings.Contains(err.Error(), "unknown plan resource") {
		t.Fatalf("got %v", err)
	}
}

func TestPlanLimitErrorMessage(t *testing.T) {
	err := error(&PlanLimitError{Resource: PlanResourceDevices, Limit: 2, PlanCode: "trial_14d"})
	var pl *PlanLimitError
	if !errors.As(err, &pl) || err.Error() != "plan trial_14d allows at most 2 devices" {
		t.Fatalf("got %v", err)
	}
}
