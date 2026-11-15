-- Rollback cassy_app direct schema/table/sequence grants and policy targets added in 000065.
DO $$
DECLARE
    pol RECORD;
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'mycaffe_app') THEN
        FOR pol IN
            SELECT schemaname, tablename, policyname
            FROM pg_policies
            WHERE schemaname = 'public'
              AND 'cassy_app' = ANY(roles)
              AND 'mycaffe_app' = ANY(roles)
        LOOP
            EXECUTE format('ALTER POLICY %I ON %I.%I TO mycaffe_app', pol.policyname, pol.schemaname, pol.tablename);
        END LOOP;
    END IF;

    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'cassy_app') THEN
        REVOKE ALL PRIVILEGES ON ALL TABLES IN SCHEMA public FROM cassy_app;
        REVOKE ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public FROM cassy_app;
        REVOKE USAGE ON SCHEMA public FROM cassy_app;
    END IF;
END $$;
