-- Refuses to run while managers exist: silently demoting them would change who can approve.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM users WHERE role = 'manager') THEN
        RAISE EXCEPTION 'users with role manager exist; reassign them before rolling back 000078';
    END IF;
END $$;

ALTER TABLE users DROP CONSTRAINT IF EXISTS chk_users_role;
ALTER TABLE users ADD CONSTRAINT chk_users_role
    CHECK (role IN ('owner', 'cashier', 'barista', 'kitchen'));
