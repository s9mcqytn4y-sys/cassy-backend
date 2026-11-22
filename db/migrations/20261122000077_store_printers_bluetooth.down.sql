ALTER TABLE printers
    DROP COLUMN IF EXISTS bluetooth_mac,
    DROP COLUMN IF EXISTS transport_type;
