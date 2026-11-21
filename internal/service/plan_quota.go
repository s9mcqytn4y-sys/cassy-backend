package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// Subscription plan quotas (PM-E1). Every create path that adds a billable resource calls
// ensurePlanQuota inside its tenant transaction, before the INSERT. The plan comes from
// platform_tenant_registry (kept in sync with stores by trigger); a limit of 0 means unlimited.
// Multi-outlet quotas are not enforced: one tenant is one store in the current data model.

// PlanResource names a resource counted against the tenant's subscription plan.
type PlanResource string

const (
	PlanResourceProducts PlanResource = "products"
	PlanResourceStaff    PlanResource = "staff"
	PlanResourceDevices  PlanResource = "devices"
	PlanResourceTables   PlanResource = "tables"
	PlanResourcePrinters PlanResource = "printers"
)

// PlanLimitError reports that the store's plan does not allow one more resource of this kind.
// Clients show an upgrade prompt; the request itself is not retryable until the plan changes
// or an existing resource is archived, revoked, or deactivated.
type PlanLimitError struct {
	Resource PlanResource
	Limit    int
	PlanCode string
}

func (e *PlanLimitError) Error() string {
	return fmt.Sprintf("plan %s allows at most %d %s", e.PlanCode, e.Limit, e.Resource)
}

// planQuota holds the trusted SQL for one resource. Column and query text never come from input.
type planQuota struct {
	limitColumn string
	countSQL    string
}

var planQuotas = map[PlanResource]planQuota{
	PlanResourceProducts: {"max_products", `SELECT COUNT(*) FROM products WHERE store_id = $1 AND archived_at IS NULL`},
	PlanResourceStaff:    {"max_staff", `SELECT COUNT(*) FROM users WHERE store_id = $1 AND is_active`},
	PlanResourceDevices:  {"max_devices", `SELECT COUNT(*) FROM devices WHERE store_id = $1 AND revoked_at IS NULL`},
	PlanResourceTables:   {"max_tables", `SELECT COUNT(*) FROM tables WHERE store_id = $1 AND archived_at IS NULL`},
	PlanResourcePrinters: {"max_printers", `SELECT COUNT(*) FROM printers WHERE store_id = $1 AND is_active`},
}

// errPlanNotResolved means the store has no registry row; creation fails closed (503).
var errPlanNotResolved = errors.New("subscription plan for store could not be resolved")

// ensurePlanQuota returns *PlanLimitError when adding one more resource would exceed the plan.
// It must run inside database.WithTenantTx before the INSERT: a transaction-scoped advisory lock
// per (store, resource) serialises concurrent creates, so two requests cannot both pass a count
// of limit-1. The lock is released at commit or rollback.
func ensurePlanQuota(ctx context.Context, tx *sqlx.Tx, storeID uuid.UUID, resource PlanResource) error {
	q, ok := planQuotas[resource]
	if !ok {
		return fmt.Errorf("unknown plan resource %q", resource)
	}
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('plan_quota:' || $1::text || ':' || $2, 0))`,
		storeID, string(resource)); err != nil {
		return fmt.Errorf("failed to lock plan quota: %w", err)
	}
	var plan struct {
		Code  string `db:"code"`
		Limit int    `db:"quota_limit"`
	}
	err := tx.GetContext(ctx, &plan, `SELECT p.code, p.`+q.limitColumn+` AS quota_limit
		FROM platform_tenant_registry r JOIN subscription_plans p ON p.code = r.plan_code
		WHERE r.store_id = $1`, storeID)
	if errors.Is(err, sql.ErrNoRows) {
		return errPlanNotResolved
	}
	if err != nil {
		return fmt.Errorf("failed to load plan quota: %w", err)
	}
	if plan.Limit == 0 {
		return nil
	}
	var used int
	if err := tx.GetContext(ctx, &used, q.countSQL, storeID); err != nil {
		return fmt.Errorf("failed to count %s: %w", resource, err)
	}
	if used >= plan.Limit {
		return &PlanLimitError{Resource: resource, Limit: plan.Limit, PlanCode: plan.Code}
	}
	return nil
}
