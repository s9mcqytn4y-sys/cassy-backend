-- E2E cashier lifecycle store (Sprint S2, Docs/PRD/PrdOpsAndPwaClose.md section 10, decision S2-Q2).
-- Loaded only by the disposable test stack (cassy-docker Core/DockerComposeTest.yml) after development.sql.
-- Deterministic: two drawers and no open shift, so tests open, use and close shifts themselves.
-- PINs are test values for a throwaway database: owner 482913, manager 739164, cashier 528307, kitchen 615842.
-- Printers point at the virtual ESC/POS printer as seen from the Android emulator (10.0.2.2:9100).

INSERT INTO stores (
    id, slug, name, address, phone, timezone, is_active,
    order_prefix, queue_prefix, default_billing_mode, shift_mode,
    enabled_payment_methods, tax_bps, service_bps, qris_mdr_bps, currency
)
VALUES (
    'e2e00000-0000-4000-8000-000000000001', 'e2e-kasir', 'Kafe Uji E2E', 'Jl. Uji No. 1, Jakarta', '021-0000000',
    'Asia/Jakarta', true, 'E2E', 'E', 'pay_first', 'single_auto',
    '["qris","cash","static_qris","edc"]'::jsonb, 1000, 500, 70, 'IDR'
)
ON CONFLICT (id) DO NOTHING;

-- FL1 W4: demo and e2e stores run on Pro so plan quotas and the expired-trial read-only guard
-- never block the live Android and web suites.
UPDATE platform_tenant_registry SET plan_code = 'pro_cafe', subscription_status = 'active', updated_at = NOW()
WHERE store_id IN ('e2e00000-0000-4000-8000-000000000001') AND (plan_code, subscription_status) <> ('pro_cafe', 'active');

INSERT INTO users (id, store_id, email, full_name, role, phone, pin_hash, is_active, pin_set_at)
VALUES
    ('e2e00001-0000-4000-8000-000000000001', 'e2e00000-0000-4000-8000-000000000001', 'owner.e2e@cassy.local',
     'Pemilik Uji', 'owner', '+628000000001',
     '$argon2id$v=19$m=19456,t=2,p=1$asaQjWwUQ7vf35jdVNPtlw$giPi5ZwfklHxgmJ2rcCuvkNw3zn5mEfDTdbaGvy+sFY', true, NOW()),
    ('e2e00002-0000-4000-8000-000000000002', 'e2e00000-0000-4000-8000-000000000001', 'manager.e2e@cassy.local',
     'Manajer Uji', 'manager', '+628000000002',
     '$argon2id$v=19$m=19456,t=2,p=1$opm8iFnn7ciL8qhwzae3KA$cM8kww65lhjTHe2xfYaBvIk113r19NKC+picnak5P1g', true, NOW()),
    ('e2e00003-0000-4000-8000-000000000003', 'e2e00000-0000-4000-8000-000000000001', 'cashier.e2e@cassy.local',
     'Kasir Uji', 'cashier', '+628000000003',
     '$argon2id$v=19$m=19456,t=2,p=1$PjUwaGwK2KEcAkdOXuFDSw$m2N+IMv1R7huTSukVy9xm0YRti/DRU95UVLsBLoXweY', true, NOW()),
    -- Kitchen tablet for the S3 KDS live tests (PrdOpsAndPwaClose.md section 11, S3-Q3).
    ('e2e00004-0000-4000-8000-000000000004', 'e2e00000-0000-4000-8000-000000000001', 'kitchen.e2e@cassy.local',
     'Dapur Uji', 'kitchen', '+628000000004',
     '$argon2id$v=19$m=19456,t=2,p=1$PwJMkaEEkAxY5AoWmFfIDg$EUmF/C8oEIuxxrVOKUIQU/EAMEVN27aSQEdGSQI4k+s', true, NOW())
ON CONFLICT (id) DO NOTHING;

INSERT INTO store_settings (
    store_id, void_refund_owner_threshold, cashier_max_discount_bps, cash_variance_tolerance,
    cashier_handles_bar, table_idle_close_minutes, qr_ordering_paused, static_qris_fallback_mode,
    table_time_limit_minutes, table_timer_warning_minutes, qr_service_mode, updated_by
)
VALUES (
    'e2e00000-0000-4000-8000-000000000001', 100000, 1000, 5000, true, 30, false, 'auto', 90, 15, 'table',
    'e2e00001-0000-4000-8000-000000000001'
)
ON CONFLICT (store_id) DO NOTHING;

INSERT INTO categories (id, store_id, name, display_order, is_active)
VALUES
    ('e2e00010-0000-4000-8000-000000000001', 'e2e00000-0000-4000-8000-000000000001', 'Minuman', 1, true),
    ('e2e00010-0000-4000-8000-000000000002', 'e2e00000-0000-4000-8000-000000000001', 'Makanan', 2, true)
ON CONFLICT (id) DO NOTHING;

-- Prices chosen so one item stays under the Rp 100.000 void threshold and one is above it.
INSERT INTO products (
    id, store_id, category_id, name, description, base_price, image_url,
    product_type, prep_station, display_order, is_available, track_stock, stock_quantity
)
VALUES
    ('e2e00020-0000-4000-8000-000000000001', 'e2e00000-0000-4000-8000-000000000001', 'e2e00010-0000-4000-8000-000000000001',
     'Kopi Uji', 'Produk uji bar', 25000, '', 'prepared', 'bar', 1, true, false, 0),
    ('e2e00020-0000-4000-8000-000000000002', 'e2e00000-0000-4000-8000-000000000001', 'e2e00010-0000-4000-8000-000000000002',
     'Nasi Uji', 'Produk uji dapur', 32000, '', 'prepared', 'kitchen', 1, true, false, 0),
    ('e2e00020-0000-4000-8000-000000000003', 'e2e00000-0000-4000-8000-000000000001', 'e2e00010-0000-4000-8000-000000000002',
     'Tumpeng Uji', 'Produk uji di atas batas void kasir', 150000, '', 'prepared', 'kitchen', 2, true, false, 0),
    -- Guest PWA-01 live test (S4-Q7): a drink with a required option the guest must pick.
    ('e2e00020-0000-4000-8000-000000000004', 'e2e00000-0000-4000-8000-000000000001', 'e2e00010-0000-4000-8000-000000000001',
     'Latte Uji', 'Produk uji dengan pilihan wajib', 28000, '', 'prepared', 'bar', 2, true, false, 0)
ON CONFLICT (id) DO NOTHING;

INSERT INTO product_variants (store_id, product_id, name, sku, price, is_default, is_available, display_order)
SELECT p.store_id, p.id, 'Reguler', '', p.base_price, true, true, 0
FROM products p
WHERE p.store_id = 'e2e00000-0000-4000-8000-000000000001'
  AND NOT EXISTS (SELECT 1 FROM product_variants v WHERE v.store_id = p.store_id AND v.product_id = p.id);

INSERT INTO modifier_groups (id, store_id, name, selection_type, is_required, min_select, max_select, display_order, is_active)
VALUES ('e2e00060-0000-4000-8000-000000000001', 'e2e00000-0000-4000-8000-000000000001', 'Susu Uji', 'single', true, 1, 1, 1, true)
ON CONFLICT (id) DO NOTHING;

INSERT INTO modifier_options (id, store_id, group_id, name, price_delta, is_available, display_order)
VALUES
    ('e2e00061-0000-4000-8000-000000000001', 'e2e00000-0000-4000-8000-000000000001', 'e2e00060-0000-4000-8000-000000000001', 'Susu Sapi', 0, true, 1),
    ('e2e00061-0000-4000-8000-000000000002', 'e2e00000-0000-4000-8000-000000000001', 'e2e00060-0000-4000-8000-000000000001', 'Susu Oat', 5000, true, 2)
ON CONFLICT (id) DO NOTHING;

INSERT INTO product_modifier_groups (store_id, product_id, group_id, display_order)
VALUES ('e2e00000-0000-4000-8000-000000000001', 'e2e00020-0000-4000-8000-000000000004', 'e2e00060-0000-4000-8000-000000000001', 1)
ON CONFLICT DO NOTHING;

INSERT INTO cash_drawers (id, store_id, name, is_active)
VALUES
    ('e2e00030-0000-4000-8000-000000000001', 'e2e00000-0000-4000-8000-000000000001', 'Laci Uji 1', true),
    ('e2e00030-0000-4000-8000-000000000002', 'e2e00000-0000-4000-8000-000000000001', 'Laci Uji 2', true)
ON CONFLICT (id) DO NOTHING;

INSERT INTO tables (id, store_id, table_number, label, section, capacity, qr_token, is_active)
VALUES
    ('e2e00040-0000-4000-8000-000000000001', 'e2e00000-0000-4000-8000-000000000001', 'U1', 'Meja Uji 1', 'Dalam', 4,
     'e2e-kasir-meja-u1-qrcode', true),
    ('e2e00040-0000-4000-8000-000000000002', 'e2e00000-0000-4000-8000-000000000001', 'U2', 'Meja Uji 2', 'Dalam', 4,
     'e2e-kasir-meja-u2-qrcode', true)
ON CONFLICT (id) DO NOTHING;

-- OPS-11: copies and footer come from the server; the receipt test asserts both on the printed text.
INSERT INTO printers (
    id, store_id, name, station, format, paper_width_mm, copies, footer, agent_token_hash,
    is_active, created_by, transport_type, lan_address
)
VALUES
    ('e2e00050-0000-4000-8000-000000000001', 'e2e00000-0000-4000-8000-000000000001', 'Struk Uji', 'receipt', 'escpos', 80,
     2, 'Terima kasih - E2E', repeat('0', 64), true, 'e2e00001-0000-4000-8000-000000000001', 'lan', '10.0.2.2:9100'),
    ('e2e00050-0000-4000-8000-000000000002', 'e2e00000-0000-4000-8000-000000000001', 'Dapur Uji', 'kitchen', 'escpos', 80,
     1, '', repeat('0', 64), true, 'e2e00001-0000-4000-8000-000000000001', 'lan', '10.0.2.2:9100')
ON CONFLICT (id) DO NOTHING;

-- S4-Q3: cup label printer behind the print agent (no lan_address, so the server queues TSPL jobs).
-- Agent token for the test stack only: "<store_id>.<printer_id>.e2e-label-agent-secret-for-the-test-stack-x" (43 chars, as the parser requires).
INSERT INTO printers (
    id, store_id, name, station, format, paper_width_mm, label_height_mm, copies, footer, agent_token_hash,
    is_active, created_by, transport_type
)
VALUES
    ('e2e00050-0000-4000-8000-000000000003', 'e2e00000-0000-4000-8000-000000000001', 'Label Cup Uji', 'label', 'tspl', 50, 30,
     1, '', '9795adfd1c2d0904ff25026d688df2aa44cb68b93f7d450a9a4543b6c9fddbc0', true, 'e2e00001-0000-4000-8000-000000000001', 'lan')
ON CONFLICT (id) DO NOTHING;

-- Guest call-waiter live test (PWA-05, S4-Q7): flag on and two request templates.
INSERT INTO store_feature_flags (store_id, flag_key, enabled, updated_by, updated_at)
VALUES ('e2e00000-0000-4000-8000-000000000001', 'call_waiter', true, 'e2e00001-0000-4000-8000-000000000001', NOW())
ON CONFLICT (store_id, flag_key) DO UPDATE SET enabled = true;

INSERT INTO service_request_templates (id, store_id, label, sort_order, is_active, created_at, updated_at)
VALUES
    ('e2e00070-0000-4000-8000-000000000001', 'e2e00000-0000-4000-8000-000000000001', 'Minta Tisu', 1, true, NOW(), NOW()),
    ('e2e00070-0000-4000-8000-000000000002', 'e2e00000-0000-4000-8000-000000000001', 'Minta Air Putih', 2, true, NOW(), NOW())
ON CONFLICT (id) DO NOTHING;
