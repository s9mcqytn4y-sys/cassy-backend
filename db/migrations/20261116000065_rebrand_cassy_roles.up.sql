-- Mirror exact least-privilege table, sequence, and function grants from legacy NOLOGIN role
-- mycaffe_app to cassy_app (NOSUPERUSER, NOBYPASSRLS) so all ledger REVOKE rules remain intact.
DO $$
DECLARE
    g   RECORD;
    sq  RECORD;
    fn  RECORD;
    pol RECORD;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'cassy_app') THEN
        RAISE NOTICE 'Role cassy_app does not exist in this cluster; skipping cassy_app privilege sync.';
        RETURN;
    END IF;

    GRANT USAGE ON SCHEMA public TO cassy_app;

    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'mycaffe_app') THEN
        FOR g IN
            SELECT table_schema, table_name, privilege_type
            FROM information_schema.role_table_grants
            WHERE grantee = 'mycaffe_app' AND table_schema = 'public'
        LOOP
            EXECUTE format('GRANT %s ON TABLE %I.%I TO cassy_app', g.privilege_type, g.table_schema, g.table_name);
        END LOOP;

        FOR sq IN
            SELECT sequence_schema, sequence_name
            FROM information_schema.sequences
            WHERE sequence_schema = 'public'
        LOOP
            IF has_sequence_privilege('mycaffe_app', format('%I.%I', sq.sequence_schema, sq.sequence_name), 'USAGE') THEN
                EXECUTE format('GRANT USAGE, SELECT ON SEQUENCE %I.%I TO cassy_app', sq.sequence_schema, sq.sequence_name);
            END IF;
        END LOOP;

        FOR fn IN
            SELECT p.oid::regprocedure AS sig
            FROM pg_proc p
            JOIN pg_namespace n ON n.oid = p.pronamespace
            WHERE n.nspname = 'public'
              AND p.prokind IN ('f', 'p')
              AND has_function_privilege('mycaffe_app', p.oid, 'EXECUTE')
        LOOP
            EXECUTE format('GRANT EXECUTE ON FUNCTION %s TO cassy_app', fn.sig);
        END LOOP;

        FOR pol IN
            SELECT schemaname, tablename, policyname
            FROM pg_policies
            WHERE schemaname = 'public'
              AND 'mycaffe_app' = ANY(roles)
        LOOP
            EXECUTE format('ALTER POLICY %I ON %I.%I TO cassy_app, mycaffe_app', pol.policyname, pol.schemaname, pol.tablename);
        END LOOP;
    END IF;
END $$;
