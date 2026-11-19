-- 20261119000072_printer_lan_address.down.sql
-- Refuses while receipt printers exist: the old station check has no place for them.

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM printers WHERE station = 'receipt') THEN
        RAISE EXCEPTION 'Receipt printers exist; cannot roll back 000072.';
    END IF;
END $$;

ALTER TABLE printers DROP CONSTRAINT IF EXISTS chk_printers_lan_address;
ALTER TABLE printers DROP CONSTRAINT IF EXISTS chk_printers_station;
ALTER TABLE printers ADD CONSTRAINT chk_printers_station CHECK (station IN ('bar', 'kitchen', 'checker', 'label'));
ALTER TABLE printers DROP COLUMN IF EXISTS lan_address;
