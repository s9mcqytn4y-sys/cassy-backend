-- Store manager role (K-04, Docs/PRD/PrdOpsAndPwaClose.md): runs the outlet day to day and
-- approves void/refund/discount with their own PIN; store settings, staff, B2B invoices and
-- cost data stay owner-only (enforced in internal/pkg/rbac and the service layer).
ALTER TABLE users DROP CONSTRAINT IF EXISTS chk_users_role;
ALTER TABLE users ADD CONSTRAINT chk_users_role
    CHECK (role IN ('owner', 'manager', 'cashier', 'barista', 'kitchen'));
