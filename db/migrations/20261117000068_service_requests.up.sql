-- Guest service requests ("Panggil Pelayan", W4.3, 2026-11-17). Owners keep the templates; a guest with
-- an active table session sends one; staff acknowledge and finish it. Gated by feature flag call_waiter.
CREATE TABLE IF NOT EXISTS service_request_templates (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    store_id   UUID NOT NULL REFERENCES stores(id) ON DELETE RESTRICT,
    label      VARCHAR(40) NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    is_active  BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT uq_service_request_templates_store_id UNIQUE (store_id, id),
    CONSTRAINT chk_service_request_templates_label CHECK (length(btrim(label)) BETWEEN 1 AND 40),
    CONSTRAINT chk_service_request_templates_sort CHECK (sort_order BETWEEN 0 AND 999)
);

CREATE TABLE IF NOT EXISTS service_requests (
    id                 UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    store_id           UUID NOT NULL REFERENCES stores(id) ON DELETE RESTRICT,
    table_id           UUID NOT NULL,
    table_session_id   UUID NOT NULL,
    guest_session_id   UUID NOT NULL,
    template_id        UUID NOT NULL,
    label_snapshot     VARCHAR(40) NOT NULL,
    note               VARCHAR(80),
    status             VARCHAR(12) NOT NULL DEFAULT 'open',
    created_at         TIMESTAMPTZ NOT NULL,
    acknowledged_by    UUID,
    acknowledged_at    TIMESTAMPTZ,
    done_by            UUID,
    done_at            TIMESTAMPTZ,
    CONSTRAINT uq_service_requests_store_id UNIQUE (store_id, id),
    CONSTRAINT chk_service_requests_status CHECK (status IN ('open', 'acknowledged', 'done', 'cancelled')),
    CONSTRAINT fk_service_requests_table FOREIGN KEY (store_id, table_id) REFERENCES tables(store_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_service_requests_table_session FOREIGN KEY (store_id, table_session_id)
        REFERENCES table_sessions(store_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_service_requests_guest_session FOREIGN KEY (store_id, guest_session_id)
        REFERENCES guest_sessions(store_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_service_requests_template FOREIGN KEY (store_id, template_id)
        REFERENCES service_request_templates(store_id, id) ON DELETE RESTRICT
);
-- One live request per template per table: a repeat tap returns the live one instead of spamming staff.
CREATE UNIQUE INDEX IF NOT EXISTS uq_service_requests_live
    ON service_requests (store_id, table_id, template_id) WHERE status IN ('open', 'acknowledged');
CREATE INDEX IF NOT EXISTS idx_service_requests_status ON service_requests (store_id, status, created_at);
CREATE INDEX IF NOT EXISTS idx_service_requests_guest ON service_requests (store_id, guest_session_id, created_at);

ALTER TABLE service_request_templates ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS p_service_request_templates_tenant_isolation ON service_request_templates;
CREATE POLICY p_service_request_templates_tenant_isolation ON service_request_templates FOR ALL TO cassy_app
    USING (store_id = current_store_id()) WITH CHECK (store_id = current_store_id());
ALTER TABLE service_requests ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS p_service_requests_tenant_isolation ON service_requests;
CREATE POLICY p_service_requests_tenant_isolation ON service_requests FOR ALL TO cassy_app
    USING (store_id = current_store_id()) WITH CHECK (store_id = current_store_id());

GRANT SELECT, INSERT, UPDATE ON service_request_templates TO cassy_app;
GRANT SELECT, INSERT, UPDATE ON service_requests TO cassy_app;
