ALTER TABLE printers
    ADD COLUMN IF NOT EXISTS transport_type TEXT NOT NULL DEFAULT 'lan'
        CHECK (transport_type IN ('lan', 'bluetooth')),
    ADD COLUMN IF NOT EXISTS bluetooth_mac TEXT;
