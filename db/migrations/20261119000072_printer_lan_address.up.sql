-- 20261119000072_printer_lan_address.up.sql
-- A1 printer config from the server: a printer with a LAN address ("host:port") is printed by the POS
-- tablets directly on port 9100, so the print queue skips it. The new receipt station is the cashier
-- printer; the server renders no receipt jobs, so a receipt printer always needs an address.

ALTER TABLE printers ADD COLUMN IF NOT EXISTS lan_address VARCHAR(64);

ALTER TABLE printers DROP CONSTRAINT IF EXISTS chk_printers_station;
ALTER TABLE printers ADD CONSTRAINT chk_printers_station CHECK (station IN ('bar', 'kitchen', 'checker', 'label', 'receipt'));

ALTER TABLE printers DROP CONSTRAINT IF EXISTS chk_printers_lan_address;
ALTER TABLE printers ADD CONSTRAINT chk_printers_lan_address CHECK (
    (lan_address IS NULL OR length(btrim(lan_address)) BETWEEN 3 AND 64)
    AND (station <> 'receipt' OR lan_address IS NOT NULL)
    AND (station <> 'label' OR lan_address IS NULL));
