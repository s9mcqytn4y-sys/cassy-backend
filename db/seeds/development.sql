-- db/seeds/development.sql
-- Consolidated Active Single-Tenant Seed (1 Source of Truth for Local Dev & Staging Demo)
-- STRICTLY BLOCKED IN PRODUCTION AND EMPTY APP_ENV by Migrator.Seed.
--
-- Scope:
--   - Platform reference rows (subscription_plans, legal_documents)
--   - 1 Tenant / 1 Flagship Store: Cassy Roastery Senopati (slug: central-cafe)
--   - 1 Owner RBAC lengkap (abdulaziz543333@gmail.com)
--   - 1 Staff level khusus terbatas (staff.senopati@abbdullaziz.online, role = cashier)
--   - Curated 16-item F&B catalog with cleaned modifier groups & uniform R2 WebP product photos
--   - Active tables, cash drawer, feature flags, call-waiter templates; ZERO seeded transactions/orders.

-- 1. PLATFORM REFERENCE DATA
INSERT INTO subscription_plans (
    code, name_id, name_en, monthly_price_idr, annual_price_idr, commission_bps,
    max_tables, max_printers, supports_byok_midtrans, supports_kds_queue_tv, supports_whitelabel, supports_byo_notifications, max_kds_stations
) VALUES
    ('trial_14d', 'Uji Coba Gratis 14 Hari', '14-Day Free Trial', 0, 0, 0, 15, 1, true, true, false, false, 1),
    ('starter_umkm', 'Starter UMKM', 'Starter SME', 149000, 1490000, 0, 15, 1, true, true, false, false, 1),
    ('pro_cafe', 'Pro Cafe & Resto', 'Pro Cafe & Resto', 299000, 2990000, 0, 0, 0, true, true, false, true, 0),
    ('enterprise_whitelabel', 'Enterprise White-Label', 'Enterprise White-Label', 599000, 5990000, 0, 0, 0, true, true, true, true, 0)
ON CONFLICT (code) DO UPDATE SET
    name_id = EXCLUDED.name_id,
    name_en = EXCLUDED.name_en,
    monthly_price_idr = EXCLUDED.monthly_price_idr,
    annual_price_idr = EXCLUDED.annual_price_idr,
    supports_byok_midtrans = EXCLUDED.supports_byok_midtrans,
    supports_kds_queue_tv = EXCLUDED.supports_kds_queue_tv,
    supports_whitelabel = EXCLUDED.supports_whitelabel,
    supports_byo_notifications = EXCLUDED.supports_byo_notifications,
    max_kds_stations = EXCLUDED.max_kds_stations,
    updated_at = NOW();

INSERT INTO legal_documents (doc_type, version, locale, title, content_markdown, content_sha256)
VALUES
    ('terms_of_service', '1.0.0', 'id-ID', 'Syarat dan Ketentuan Layanan SaaS Cassy by AzureDevID',
     'AzureDevID bertindak murni sebagai penyedia perangkat lunak (SaaS Provider) dengan skema Bring Your Own Key (BYOK) dan 0% komisi per transaksi. Dana transaksi menu mengalir langsung ke rekening masing-masing Tenant.',
     encode(sha256('tos-cassy-azuredevid-v1.0.0-id'::bytea), 'hex')),
    ('privacy_policy_pdp', '1.0.0', 'id-ID', 'Kebijakan Privasi & Perlindungan Data Pribadi (UU No. 27 Tahun 2022)',
     'Pemrosesan data pribadi pelanggan dan staf gerai dilindungi dengan isolasi Row-Level Security PostgreSQL 18 dan enkripsi kredensial AES-256-GCM sesuai UU PDP No. 27/2022.',
     encode(sha256('pdp-cassy-azuredevid-v1.0.0-id'::bytea), 'hex')),
    ('dpa_tenant', '1.0.0', 'id-ID', 'Perjanjian Pemrosesan Data Mitra Usaha (Data Processing Agreement)',
     'Mitra Usaha bertindak sebagai Pengendali Data Pribadi pelanggan gerai dan menunjuk AzureDevID sebagai Prosesor Data Pribadi.',
     encode(sha256('dpa-cassy-azuredevid-v1.0.0-id'::bytea), 'hex')),
    ('refund_policy', '1.0.0', 'id-ID', 'Kebijakan Pembatalan dan Pengembalian Dana Langganan SaaS',
     'Langganan SaaS dapat dibatalkan kapan saja sebelum jatuh tempo berikutnya. Kelebihan bayar atau duplikasi transaksi dikembalikan 100% dalam 5-7 hari kerja.',
     encode(sha256('refund-cassy-azuredevid-v1.0.0-id'::bytea), 'hex')),
    ('terms_of_service', '1.0.0', 'en-US', 'Cassy by AzureDevID SaaS Terms of Service',
     'AzureDevID operates strictly as a pure Software-as-a-Service (SaaS) provider using Bring Your Own Key (BYOK) gateway credentials with 0% transaction commission. Customer menu payments settle directly to each Tenant merchant account.',
     encode(sha256('tos-cassy-azuredevid-v1.0.0-en'::bytea), 'hex')),
    ('privacy_policy_pdp', '1.0.0', 'en-US', 'Privacy Policy & Personal Data Protection (Indonesia Law No. 27/2022)',
     'Guest and staff personal data are isolated per tenant using PostgreSQL 18 Row-Level Security and AES-256-GCM credential encryption in compliance with Indonesia PDP Law No. 27/2022.',
     encode(sha256('pdp-cassy-azuredevid-v1.0.0-en'::bytea), 'hex'))
ON CONFLICT (doc_type, version, locale) DO UPDATE SET
    title = EXCLUDED.title,
    content_markdown = EXCLUDED.content_markdown;

-- 2. SINGLE ACTIVE TENANT STORE (1 Store Focus: Cassy Roastery Senopati)
INSERT INTO stores (
    id, slug, name, address, phone, timezone, is_active,
    order_prefix, queue_prefix, default_billing_mode, shift_mode,
    enabled_payment_methods, tax_bps, service_bps, qris_mdr_bps, currency
)
VALUES (
    '11111111-1111-4111-8111-111111111111',
    'central-cafe',
    'Cassy Roastery Senopati',
    'Jl. Senopati No. 88, Kebayoran Baru, Jakarta Selatan 12190',
    '021-52908811',
    'Asia/Jakarta',
    true,
    'ORD',
    'A',
    'pay_first',
    'single_auto',
    '["qris","bank_transfer","gopay","shopeepay","cash","static_qris","edc"]'::jsonb,
    1000,
    500,
    70,
    'IDR'
)
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    address = EXCLUDED.address,
    phone = EXCLUDED.phone,
    enabled_payment_methods = EXCLUDED.enabled_payment_methods,
    updated_at = NOW();

-- FL1 W4: demo and e2e stores run on Pro so plan quotas and the expired-trial read-only guard
-- never block the live Android and web suites.
UPDATE platform_tenant_registry SET plan_code = 'pro_cafe', subscription_status = 'active', updated_at = NOW()
WHERE store_id IN ('11111111-1111-4111-8111-111111111111') AND (plan_code, subscription_status) <> ('pro_cafe', 'active');

-- 3. ACCOUNTS & ROLE-LEVEL RBAC (1 Owner Lengkap + 1 Restricted Staff Cashier)
-- Local dev PIN: 482913 (rotated immediately in staging/demo via -action=demo-pins + .env.demo).
INSERT INTO users (id, store_id, email, full_name, role, phone, pin_hash, is_active, pin_set_at)
VALUES
    (
        'a0000001-0000-4000-8000-000000000001',
        '11111111-1111-4111-8111-111111111111',
        'abdulaziz543333@gmail.com',
        'Abdul Aziz (Owner)',
        'owner',
        '+628567060477',
        '$argon2id$v=19$m=19456,t=2,p=1$asaQjWwUQ7vf35jdVNPtlw$giPi5ZwfklHxgmJ2rcCuvkNw3zn5mEfDTdbaGvy+sFY',
        true,
        NOW()
    ),
    (
        'a0000003-0000-4000-8000-000000000003',
        '11111111-1111-4111-8111-111111111111',
        'staff.senopati@abbdullaziz.online',
        'Nadya Kusuma (Kasir & Pelayan Meja)',
        'cashier',
        '+628129008813',
        '$argon2id$v=19$m=19456,t=2,p=1$asaQjWwUQ7vf35jdVNPtlw$giPi5ZwfklHxgmJ2rcCuvkNw3zn5mEfDTdbaGvy+sFY',
        true,
        NOW()
    ),
    (
        'a0000004-0000-4000-8000-000000000004',
        '11111111-1111-4111-8111-111111111111',
        'barista.senopati@abbdullaziz.online',
        'Bima Pratama (Barista KDS)',
        'barista',
        '+628129008814',
        '$argon2id$v=19$m=19456,t=2,p=1$asaQjWwUQ7vf35jdVNPtlw$giPi5ZwfklHxgmJ2rcCuvkNw3zn5mEfDTdbaGvy+sFY',
        true,
        NOW()
    ),
    (
        'a0000005-0000-4000-8000-000000000005',
        '11111111-1111-4111-8111-111111111111',
        'kitchen.senopati@abbdullaziz.online',
        'Rina Dewi (Dapur KDS)',
        'kitchen',
        '+628129008815',
        '$argon2id$v=19$m=19456,t=2,p=1$asaQjWwUQ7vf35jdVNPtlw$giPi5ZwfklHxgmJ2rcCuvkNw3zn5mEfDTdbaGvy+sFY',
        true,
        NOW()
    )
ON CONFLICT (id) DO UPDATE SET
    email = EXCLUDED.email,
    full_name = EXCLUDED.full_name,
    role = EXCLUDED.role,
    phone = EXCLUDED.phone,
    pin_hash = EXCLUDED.pin_hash,
    is_active = true,
    pin_set_at = COALESCE(users.pin_set_at, NOW());

-- 4. STORE SETTINGS, FEATURE FLAGS & CALL-WAITER TEMPLATES
INSERT INTO store_settings (
    store_id, void_refund_owner_threshold, cashier_max_discount_bps, cash_variance_tolerance,
    cashier_handles_bar, table_idle_close_minutes, qr_ordering_paused, static_qris_fallback_mode,
    table_time_limit_minutes, table_timer_warning_minutes, qr_service_mode, updated_by
)
VALUES (
    '11111111-1111-4111-8111-111111111111',
    100000, 1000, 5000, true, 30, false, 'auto', 90, 15, 'table',
    'a0000001-0000-4000-8000-000000000001'
)
ON CONFLICT (store_id) DO NOTHING;

INSERT INTO store_feature_flags (store_id, flag_key, enabled, updated_by, updated_at)
VALUES
    ('11111111-1111-4111-8111-111111111111', 'call_waiter', true, 'a0000001-0000-4000-8000-000000000001', NOW()),
    ('11111111-1111-4111-8111-111111111111', 'kds', true, 'a0000001-0000-4000-8000-000000000001', NOW()),
    ('11111111-1111-4111-8111-111111111111', 'split_bill', true, 'a0000001-0000-4000-8000-000000000001', NOW()),
    ('11111111-1111-4111-8111-111111111111', 'open_bill', true, 'a0000001-0000-4000-8000-000000000001', NOW()),
    ('11111111-1111-4111-8111-111111111111', 'queue_display', true, 'a0000001-0000-4000-8000-000000000001', NOW()),
    ('11111111-1111-4111-8111-111111111111', 'dual_screen', false, 'a0000001-0000-4000-8000-000000000001', NOW()),
    ('11111111-1111-4111-8111-111111111111', 'stock', true, 'a0000001-0000-4000-8000-000000000001', NOW()),
    ('11111111-1111-4111-8111-111111111111', 'whatsapp', false, 'a0000001-0000-4000-8000-000000000001', NOW())
ON CONFLICT (store_id, flag_key) DO NOTHING;

INSERT INTO service_request_templates (id, store_id, label, sort_order, is_active, created_at, updated_at)
VALUES
    ('80000001-0000-4000-8000-000000000001', '11111111-1111-4111-8111-111111111111', 'Minta Tisu & Alat Makan', 1, true, NOW(), NOW()),
    ('80000002-0000-4000-8000-000000000002', '11111111-1111-4111-8111-111111111111', 'Isi Ulang Air Minum', 2, true, NOW(), NOW()),
    ('80000003-0000-4000-8000-000000000003', '11111111-1111-4111-8111-111111111111', 'Panggil Staf ke Meja', 3, true, NOW(), NOW()),
    ('80000004-0000-4000-8000-000000000004', '11111111-1111-4111-8111-111111111111', 'Minta Tagihan Meja', 4, true, NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET
    label = EXCLUDED.label,
    sort_order = EXCLUDED.sort_order,
    is_active = true,
    updated_at = NOW();

-- 5. CATEGORIES & 16 REALISTIC MENU ITEMS WITH UNIFORM STUDIO PHOTOS
INSERT INTO categories (id, store_id, name, display_order, is_active)
VALUES
    ('c0000001-0000-4000-8000-000000000001', '11111111-1111-4111-8111-111111111111', 'Signature & Kopi', 1, true),
    ('c0000002-0000-4000-8000-000000000002', '11111111-1111-4111-8111-111111111111', 'Non-Kopi & Teh', 2, true),
    ('c0000003-0000-4000-8000-000000000003', '11111111-1111-4111-8111-111111111111', 'Pastry & Kue', 3, true),
    ('c0000004-0000-4000-8000-000000000004', '11111111-1111-4111-8111-111111111111', 'Makanan & Camilan', 4, true)
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    display_order = EXCLUDED.display_order,
    is_active = EXCLUDED.is_active;

INSERT INTO products (
    id, store_id, category_id, name, description, base_price, image_url,
    product_type, prep_station, display_order, is_available, track_stock, stock_quantity
)
VALUES
    -- Signature & Kopi
    ('d0000001-0000-4000-8000-000000000001', '11111111-1111-4111-8111-111111111111', 'c0000001-0000-4000-8000-000000000001', 'Classic Iced Latte', 'Double espresso shot dengan susu segar dingin dan gula cair', 24000, 'https://cdn.abbdullaziz.online/seeds/menu/classic-iced-latte.webp', 'prepared', 'bar', 1, true, false, 0),
    ('d0000007-0000-4000-8000-000000000007', '11111111-1111-4111-8111-111111111111', 'c0000001-0000-4000-8000-000000000001', 'Es Kopi Susu Aren Senopati', 'Double ristretto house blend dengan gula aren organik dan susu segar', 26000, 'https://cdn.abbdullaziz.online/seeds/menu/es-kopi-susu.webp', 'prepared', 'bar', 2, true, false, 0),
    ('d0000002-0000-4000-8000-000000000002', '11111111-1111-4111-8111-111111111111', 'c0000001-0000-4000-8000-000000000001', 'Caffe Latte', 'Espresso house blend dengan steamed milk dan microfoam lembut', 26000, 'https://cdn.abbdullaziz.online/seeds/menu/caffe-latte.webp', 'prepared', 'bar', 3, true, false, 0),
    ('d0000003-0000-4000-8000-000000000003', '11111111-1111-4111-8111-111111111111', 'c0000001-0000-4000-8000-000000000001', 'Americano (Double Shot)', 'Ekstraksi double ristretto diseduh dengan air mineral panas', 20000, 'https://cdn.abbdullaziz.online/seeds/menu/americano.webp', 'prepared', 'bar', 4, true, false, 0),
    ('d0000011-0000-4000-8000-000000000011', '11111111-1111-4111-8111-111111111111', 'c0000001-0000-4000-8000-000000000001', 'Velvet Cappuccino', 'Double espresso dengan microfoam beludru dan taburan kakao', 28000, 'https://cdn.abbdullaziz.online/seeds/menu/cappuccino.webp', 'prepared', 'bar', 5, true, false, 0),
    ('d0000004-0000-4000-8000-000000000004', '11111111-1111-4111-8111-111111111111', 'c0000001-0000-4000-8000-000000000001', 'V60 Single Origin Gayo', 'Manual pour-over biji pilihan Aceh Gayo dengan catatan aroma jeruk bersih', 32000, 'https://cdn.abbdullaziz.online/seeds/menu/v60-pour-over.webp', 'prepared', 'bar', 6, true, true, 45),
    -- Non-Kopi & Teh
    ('d0000020-0000-4000-8000-000000000020', '11111111-1111-4111-8111-111111111111', 'c0000002-0000-4000-8000-000000000002', 'Uji Ceremonial Matcha Latte', 'Matcha Kyoto panen pertama dikocok manual di atas susu segar dingin', 36000, 'https://cdn.abbdullaziz.online/seeds/menu/matcha-latte.webp', 'prepared', 'bar', 1, true, false, 0),
    ('d0000022-0000-4000-8000-000000000022', '11111111-1111-4111-8111-111111111111', 'c0000002-0000-4000-8000-000000000002', '72% Single Origin Dark Chocolate', 'Cokelat hitam single-origin pekat dengan tekstur lembut', 34000, 'https://cdn.abbdullaziz.online/seeds/menu/dark-chocolate.webp', 'prepared', 'bar', 2, true, false, 0),
    ('d0000023-0000-4000-8000-000000000023', '11111111-1111-4111-8111-111111111111', 'c0000002-0000-4000-8000-000000000002', 'Es Teh Lemon Artisan', 'Seduhan teh hitam aromatik dengan perasan lemon segar dan es batu', 22000, 'https://cdn.abbdullaziz.online/seeds/menu/es-teh.webp', 'prepared', 'bar', 3, true, false, 0),
    -- Pastry & Kue
    ('d0000005-0000-4000-8000-000000000005', '11111111-1111-4111-8111-111111111111', 'c0000003-0000-4000-8000-000000000003', 'Butter Croissant Artisanal', 'Croissant berlapis mentega Prancis dipanggang segar tiap pagi', 24000, 'https://cdn.abbdullaziz.online/seeds/menu/croissant.webp', 'ready_to_serve', 'kitchen', 1, true, true, 18),
    ('d0000017-0000-4000-8000-000000000017', '11111111-1111-4111-8111-111111111111', 'c0000003-0000-4000-8000-000000000003', 'Pain au Chocolat Valrhona', 'Pastri berlapis renyah berisi cokelat hitam panggang', 28000, 'https://cdn.abbdullaziz.online/seeds/menu/choco-croissant.webp', 'ready_to_serve', 'kitchen', 2, true, true, 24),
    ('d0000019-0000-4000-8000-000000000019', '11111111-1111-4111-8111-111111111111', 'c0000003-0000-4000-8000-000000000003', 'Basque Burnt Cheesecake', 'Cheesecake panggang karamel gaya San Sebastian dengan tengah lembut', 42000, 'https://cdn.abbdullaziz.online/seeds/menu/burnt-cheesecake.webp', 'ready_to_serve', 'kitchen', 3, true, true, 16),
    ('d0000018-0000-4000-8000-000000000018', '11111111-1111-4111-8111-111111111111', 'c0000003-0000-4000-8000-000000000003', 'Banana Walnut Loaf', 'Kue pisang raja panggang dengan kacang kenari renyah', 26000, 'https://cdn.abbdullaziz.online/seeds/menu/banana-bread.webp', 'ready_to_serve', 'kitchen', 4, true, true, 20),
    -- Makanan & Camilan
    ('d0000025-0000-4000-8000-000000000025', '11111111-1111-4111-8111-111111111111', 'c0000004-0000-4000-8000-000000000004', 'Nasi Goreng Kampung Roastery', 'Nasi goreng rempah autentik dengan telur mata sapi dan acar', 48000, 'https://cdn.abbdullaziz.online/seeds/menu/nasi-goreng.webp', 'prepared', 'kitchen', 1, true, true, 25),
    ('d0000026-0000-4000-8000-000000000026', '11111111-1111-4111-8111-111111111111', 'c0000004-0000-4000-8000-000000000004', 'Ayam Goreng Sambal Matah Bowl', 'Nasi hangat dengan paha ayam goreng renyah dan sambal matah Bali', 52000, 'https://cdn.abbdullaziz.online/seeds/menu/ayam-goreng-nasi.webp', 'prepared', 'kitchen', 2, true, true, 25),
    ('d0000027-0000-4000-8000-000000000027', '11111111-1111-4111-8111-111111111111', 'c0000004-0000-4000-8000-000000000004', 'Kentang Goreng Truffle Parmesan', 'Kentang goreng renyah dengan minyak truffle dan taburan keju parut', 38000, 'https://cdn.abbdullaziz.online/seeds/menu/kentang-goreng.webp', 'prepared', 'kitchen', 3, true, false, 0)
ON CONFLICT (id) DO UPDATE SET
    category_id = EXCLUDED.category_id,
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    base_price = EXCLUDED.base_price,
    image_url = EXCLUDED.image_url,
    product_type = EXCLUDED.product_type,
    prep_station = EXCLUDED.prep_station,
    display_order = EXCLUDED.display_order;

INSERT INTO product_variants (store_id, product_id, name, sku, price, is_default, is_available, display_order)
SELECT p.store_id, p.id, 'Reguler', '', p.base_price, true, true, 0
FROM products p
WHERE NOT EXISTS (
    SELECT 1 FROM product_variants v WHERE v.store_id = p.store_id AND v.product_id = p.id
);

INSERT INTO product_variants (id, store_id, product_id, name, sku, price, is_default, is_available, display_order)
VALUES
    ('e0000002-0000-4000-8000-000000000022', '11111111-1111-4111-8111-111111111111', 'd0000002-0000-4000-8000-000000000002', 'Large', 'LAT-L', 32000, false, true, 1),
    ('e0000003-0000-4000-8000-000000000003', '11111111-1111-4111-8111-111111111111', 'd0000007-0000-4000-8000-000000000007', 'Large (16 oz)', 'KSA-L', 32000, false, true, 1),
    ('e0000006-0000-4000-8000-000000000006', '11111111-1111-4111-8111-111111111111', 'd0000011-0000-4000-8000-000000000011', 'Large (16 oz)', 'CAP-L', 34000, false, true, 1),
    ('e0000007-0000-4000-8000-000000000007', '11111111-1111-4111-8111-111111111111', 'd0000020-0000-4000-8000-000000000020', 'Large (16 oz)', 'UJM-L', 42000, false, true, 1)
ON CONFLICT (id) DO NOTHING;

-- 6. CLEAN MODIFIER GROUPS & SUB-ADDONS
INSERT INTO modifier_groups (id, store_id, name, selection_type, is_required, min_select, max_select, display_order, is_active)
VALUES
    ('10000001-0000-4000-8000-000000000001', '11111111-1111-4111-8111-111111111111', 'Milk', 'single', true, 1, 1, 1, true),
    ('10000002-0000-4000-8000-000000000002', '11111111-1111-4111-8111-111111111111', 'Sweetness', 'single', true, 1, 1, 2, true),
    ('10000003-0000-4000-8000-000000000003', '11111111-1111-4111-8111-111111111111', 'Plant Milk Topping', 'single', true, 1, 1, 3, true),
    ('10000004-0000-4000-8000-000000000004', '11111111-1111-4111-8111-111111111111', 'Ice Level', 'single', false, 0, 1, 4, true),
    ('10000005-0000-4000-8000-000000000005', '11111111-1111-4111-8111-111111111111', 'Extra Shot & Beans', 'multiple', false, 0, 2, 5, true),
    ('10000006-0000-4000-8000-000000000006', '11111111-1111-4111-8111-111111111111', 'Serving & Spread', 'multiple', false, 0, 2, 6, true),
    ('10000007-0000-4000-8000-000000000007', '11111111-1111-4111-8111-111111111111', 'Sides & Spice', 'multiple', false, 0, 3, 7, true)
ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, updated_at = NOW();

INSERT INTO modifier_options (id, store_id, group_id, name, price_delta, is_available, display_order)
VALUES
    ('20000001-0000-4000-8000-000000000001', '11111111-1111-4111-8111-111111111111', '10000001-0000-4000-8000-000000000001', 'Fresh Dairy Milk', 0, true, 1),
    ('20000002-0000-4000-8000-000000000002', '11111111-1111-4111-8111-111111111111', '10000001-0000-4000-8000-000000000001', 'Oat Milk Barista', 8000, true, 2),
    ('20000007-0000-4000-8000-000000000007', '11111111-1111-4111-8111-111111111111', '10000001-0000-4000-8000-000000000001', 'Almond Barista', 8000, true, 3),
    ('20000003-0000-4000-8000-000000000003', '11111111-1111-4111-8111-111111111111', '10000002-0000-4000-8000-000000000002', 'Normal Sugar (100%)', 0, true, 1),
    ('20000004-0000-4000-8000-000000000004', '11111111-1111-4111-8111-111111111111', '10000002-0000-4000-8000-000000000002', 'Less Sugar (50%)', 0, true, 2),
    ('20000009-0000-4000-8000-000000000009', '11111111-1111-4111-8111-111111111111', '10000002-0000-4000-8000-000000000002', 'Tanpa Gula (0%)', 0, true, 3),
    ('20000005-0000-4000-8000-000000000005', '11111111-1111-4111-8111-111111111111', '10000003-0000-4000-8000-000000000003', 'Tuang Standar', 0, true, 1),
    ('20000006-0000-4000-8000-000000000006', '11111111-1111-4111-8111-111111111111', '10000003-0000-4000-8000-000000000003', 'Whipped Oat Cold Foam', 4000, true, 2),
    ('20000011-0000-4000-8000-000000000011', '11111111-1111-4111-8111-111111111111', '10000004-0000-4000-8000-000000000004', 'Es Normal', 0, true, 1),
    ('20000012-0000-4000-8000-000000000012', '11111111-1111-4111-8111-111111111111', '10000004-0000-4000-8000-000000000004', 'Less Ice (70%)', 0, true, 2),
    ('20000013-0000-4000-8000-000000000013', '11111111-1111-4111-8111-111111111111', '10000005-0000-4000-8000-000000000005', 'Extra Espresso Shot', 6000, true, 1),
    ('20000014-0000-4000-8000-000000000014', '11111111-1111-4111-8111-111111111111', '10000005-0000-4000-8000-000000000005', 'Single Origin Aceh Gayo', 8000, true, 2),
    ('20000016-0000-4000-8000-000000000016', '11111111-1111-4111-8111-111111111111', '10000006-0000-4000-8000-000000000006', 'Hangatkan Sebelum Disajikan', 0, true, 1),
    ('20000017-0000-4000-8000-000000000017', '11111111-1111-4111-8111-111111111111', '10000006-0000-4000-8000-000000000006', 'Ekstra Mentega & Selai', 6000, true, 2),
    ('20000018-0000-4000-8000-000000000018', '11111111-1111-4111-8111-111111111111', '10000007-0000-4000-8000-000000000007', 'Sambal Dipisah', 0, true, 1),
    ('20000019-0000-4000-8000-000000000019', '11111111-1111-4111-8111-111111111111', '10000007-0000-4000-8000-000000000007', 'Tambah Telur Onsen', 7000, true, 2)
ON CONFLICT (id) DO NOTHING;

INSERT INTO product_modifier_groups (store_id, product_id, group_id, display_order)
VALUES
    ('11111111-1111-4111-8111-111111111111', 'd0000001-0000-4000-8000-000000000001', '10000001-0000-4000-8000-000000000001', 1),
    ('11111111-1111-4111-8111-111111111111', 'd0000001-0000-4000-8000-000000000001', '10000002-0000-4000-8000-000000000002', 2),
    ('11111111-1111-4111-8111-111111111111', 'd0000001-0000-4000-8000-000000000001', '10000004-0000-4000-8000-000000000004', 3),
    ('11111111-1111-4111-8111-111111111111', 'd0000001-0000-4000-8000-000000000001', '10000005-0000-4000-8000-000000000005', 4),
    ('11111111-1111-4111-8111-111111111111', 'd0000007-0000-4000-8000-000000000007', '10000001-0000-4000-8000-000000000001', 1),
    ('11111111-1111-4111-8111-111111111111', 'd0000007-0000-4000-8000-000000000007', '10000002-0000-4000-8000-000000000002', 2),
    ('11111111-1111-4111-8111-111111111111', 'd0000020-0000-4000-8000-000000000020', '10000001-0000-4000-8000-000000000001', 1),
    ('11111111-1111-4111-8111-111111111111', 'd0000020-0000-4000-8000-000000000020', '10000002-0000-4000-8000-000000000002', 2),
    ('11111111-1111-4111-8111-111111111111', 'd0000005-0000-4000-8000-000000000005', '10000006-0000-4000-8000-000000000006', 1),
    ('11111111-1111-4111-8111-111111111111', 'd0000017-0000-4000-8000-000000000017', '10000006-0000-4000-8000-000000000006', 1),
    ('11111111-1111-4111-8111-111111111111', 'd0000025-0000-4000-8000-000000000025', '10000007-0000-4000-8000-000000000007', 1),
    ('11111111-1111-4111-8111-111111111111', 'd0000026-0000-4000-8000-000000000026', '10000007-0000-4000-8000-000000000007', 1)
ON CONFLICT (store_id, product_id, group_id) DO NOTHING;

INSERT INTO option_modifier_groups (store_id, parent_option_id, child_group_id, display_order)
VALUES
    ('11111111-1111-4111-8111-111111111111', '20000002-0000-4000-8000-000000000002', '10000003-0000-4000-8000-000000000003', 1),
    ('11111111-1111-4111-8111-111111111111', '20000007-0000-4000-8000-000000000007', '10000003-0000-4000-8000-000000000003', 1)
ON CONFLICT (store_id, parent_option_id, child_group_id) DO NOTHING;

-- 7. FLOOR TABLES & CASH DRAWER
INSERT INTO tables (id, store_id, table_number, label, section, capacity, qr_token, is_active)
VALUES
    ('50000001-0000-4000-8000-000000000001', '11111111-1111-4111-8111-111111111111', 'M-01', 'Meja A1 Indoor', 'Indoor Roastery', 2, 'qr_senopati_m01_7f8a9b2c3d4e5f6a7b8c9d01', true),
    ('50000002-0000-4000-8000-000000000002', '11111111-1111-4111-8111-111111111111', 'M-02', 'Meja A2 Indoor', 'Indoor Roastery', 2, 'qr_senopati_m02_7f8a9b2c3d4e5f6a7b8c9d02', true),
    ('50000003-0000-4000-8000-000000000003', '11111111-1111-4111-8111-111111111111', 'M-03', 'Sofa Jendela Utara', 'Indoor Roastery', 4, 'qr_senopati_m03_7f8a9b2c3d4e5f6a7b8c9d03', true),
    ('50000004-0000-4000-8000-000000000004', '11111111-1111-4111-8111-111111111111', 'M-04', 'Sofa Jendela Selatan', 'Indoor Roastery', 4, 'qr_senopati_m04_7f8a9b2c3d4e5f6a7b8c9d04', true),
    ('50000006-0000-4000-8000-000000000006', '11111111-1111-4111-8111-111111111111', 'G-01', 'Teras Taman 1', 'Outdoor Garden', 4, 'qr_senopati_g01_7f8a9b2c3d4e5f6a7b8c9d06', true),
    ('50000008-0000-4000-8000-000000000008', '11111111-1111-4111-8111-111111111111', 'VIP-01', 'Ruang Rapat Roastery', 'VIP Lounge', 8, 'qr_senopati_vip1_7f8a9b2c3d4e5f6a7b8c9d08', true)
ON CONFLICT (id) DO NOTHING;

INSERT INTO cash_drawers (id, store_id, name, is_active)
VALUES
    ('60000001-0000-4000-8000-000000000001', '11111111-1111-4111-8111-111111111111', 'Laci Kasir Utama Lt. 1', true)
ON CONFLICT (id) DO NOTHING;

INSERT INTO shifts (id, store_id, drawer_id, opened_by, opening_float, opened_at, status)
SELECT
    '61000001-0000-4000-8000-000000000001',
    '11111111-1111-4111-8111-111111111111',
    '60000001-0000-4000-8000-000000000001',
    u.id,
    500000,
    NOW(),
    'open'
FROM users u
WHERE u.store_id = '11111111-1111-4111-8111-111111111111'
ORDER BY u.created_at
LIMIT 1
ON CONFLICT DO NOTHING;

-- 8. COMPLETED MEDIA UPLOADS LEDGER
INSERT INTO media_uploads (
    id, store_id, kind, product_id, status, object_key, content_type, size_bytes,
    variants, created_by, created_at, expires_at, completed_at
)
SELECT
    ('700000' || right(p.id::text, 2) || '-0000-4000-8000-0000000000' || right(p.id::text, 2))::uuid,
    p.store_id,
    'product_image',
    p.id,
    'completed',
    replace(p.image_url, 'https://cdn.abbdullaziz.online/', ''),
    'image/webp',
    56000,
    jsonb_build_object(
        'original', p.image_url,
        'card', p.image_url,
        'thumb', p.image_url
    ),
    'a0000001-0000-4000-8000-000000000001'::uuid,
    NOW(),
    NOW() + INTERVAL '365 days',
    NOW()
FROM products p
WHERE p.store_id = '11111111-1111-4111-8111-111111111111' AND p.image_url IS NOT NULL
ON CONFLICT (id) DO NOTHING;

-- 9. STORE CMS BRANDING & SALES CHANNELS (COMPLETE SEED)
INSERT INTO store_cms_configs (
    store_id, default_locale, supported_locales,
    hero_title_id, hero_title_en, hero_subtitle_id, hero_subtitle_en,
    announcement_banner_id, announcement_banner_en,
    primary_hex, accent_hex, operating_hours,
    public_refund_policy, public_contact_phone, public_contact_email
)
VALUES (
    '11111111-1111-4111-8111-111111111111',
    'id-ID',
    '["id-ID", "en-US"]'::jsonb,
    'Cassy Roastery Senopati',
    'Cassy Roastery Senopati',
    'Kopi specialty, pastry hangat, dan hidangan rumahan langsung dari meja Anda.',
    'Specialty coffee, fresh pastries, and comfort meals ordered right from your table.',
    'Buka setiap hari 07.00 - 22.00 WIB. Pesan via QR tanpa antre.',
    'Open daily 07:00 - 22:00 WIB. Order via QR with zero queue.',
    '#0D6E6E',
    '#D97706',
    '{"mon":"07:00-22:00","tue":"07:00-22:00","wed":"07:00-22:00","thu":"07:00-22:00","fri":"07:00-23:00","sat":"07:00-23:00","sun":"07:00-22:00"}'::jsonb,
    'Pembatalan sebelum pesanan diproses dapur dikembalikan penuh melalui kasir gerai.',
    '+628567060477',
    'abdulaziz543333@gmail.com'
)
ON CONFLICT (store_id) DO UPDATE SET
    hero_title_id = EXCLUDED.hero_title_id,
    hero_title_en = EXCLUDED.hero_title_en,
    hero_subtitle_id = EXCLUDED.hero_subtitle_id,
    hero_subtitle_en = EXCLUDED.hero_subtitle_en,
    announcement_banner_id = EXCLUDED.announcement_banner_id,
    announcement_banner_en = EXCLUDED.announcement_banner_en,
    primary_hex = EXCLUDED.primary_hex,
    accent_hex = EXCLUDED.accent_hex,
    public_contact_phone = EXCLUDED.public_contact_phone,
    public_contact_email = EXCLUDED.public_contact_email,
    updated_at = NOW();

INSERT INTO sales_channels (id, store_id, name, commission_bps, is_active)
VALUES
    ('90000001-0000-4000-8000-000000000001', '11111111-1111-4111-8111-111111111111', 'Dine-In', 0, true),
    ('90000002-0000-4000-8000-000000000002', '11111111-1111-4111-8111-111111111111', 'Takeaway', 0, true),
    ('90000003-0000-4000-8000-000000000003', '11111111-1111-4111-8111-111111111111', 'GoFood', 2000, true),
    ('90000004-0000-4000-8000-000000000004', '11111111-1111-4111-8111-111111111111', 'GrabFood', 2000, true),
    ('90000005-0000-4000-8000-000000000005', '11111111-1111-4111-8111-111111111111', 'ShopeeFood', 2000, true)
ON CONFLICT (store_id, name) DO UPDATE SET
    commission_bps = EXCLUDED.commission_bps,
    is_active = EXCLUDED.is_active,
    updated_at = NOW();

-- 9. BATCH 2 SMART PAIRING, BADGES & KDS STATION PREP CONFIGS
INSERT INTO kitchen_station_configs (store_id, prep_station, default_prep_minutes, per_active_order_minutes, updated_at)
VALUES
    ('11111111-1111-4111-8111-111111111111', 'bar', 4, 2, NOW()),
    ('11111111-1111-4111-8111-111111111111', 'kitchen', 8, 3, NOW()),
    ('11111111-1111-4111-8111-111111111111', 'none', 1, 0, NOW())
ON CONFLICT (store_id, prep_station) DO UPDATE SET
    default_prep_minutes = EXCLUDED.default_prep_minutes,
    per_active_order_minutes = EXCLUDED.per_active_order_minutes,
    updated_at = NOW();

UPDATE products SET badge_label = 'Favorit', prep_minutes = 4
WHERE store_id = '11111111-1111-4111-8111-111111111111'
  AND id IN ('d0000001-0000-4000-8000-000000000001', 'd0000007-0000-4000-8000-000000000007', 'd0000020-0000-4000-8000-000000000020');

UPDATE products SET badge_label = 'Rekomendasi Chef', prep_minutes = 3
WHERE store_id = '11111111-1111-4111-8111-111111111111'
  AND id IN ('d0000005-0000-4000-8000-000000000005', 'd0000019-0000-4000-8000-000000000019');

UPDATE products SET badge_label = 'Favorit', prep_minutes = 10
WHERE store_id = '11111111-1111-4111-8111-111111111111'
  AND id IN ('d0000025-0000-4000-8000-000000000025', 'd0000026-0000-4000-8000-000000000026');

INSERT INTO product_recommendations (store_id, product_id, recommended_product_id, sort_order)
VALUES
    -- Kopi -> Pastry & Camilan
    ('11111111-1111-4111-8111-111111111111', 'd0000001-0000-4000-8000-000000000001', 'd0000005-0000-4000-8000-000000000005', 1),
    ('11111111-1111-4111-8111-111111111111', 'd0000001-0000-4000-8000-000000000001', 'd0000019-0000-4000-8000-000000000019', 2),
    ('11111111-1111-4111-8111-111111111111', 'd0000007-0000-4000-8000-000000000007', 'd0000005-0000-4000-8000-000000000005', 1),
    ('11111111-1111-4111-8111-111111111111', 'd0000007-0000-4000-8000-000000000007', 'd0000017-0000-4000-8000-000000000017', 2),
    ('11111111-1111-4111-8111-111111111111', 'd0000002-0000-4000-8000-000000000002', 'd0000005-0000-4000-8000-000000000005', 1),
    ('11111111-1111-4111-8111-111111111111', 'd0000020-0000-4000-8000-000000000020', 'd0000019-0000-4000-8000-000000000019', 1),
    -- Makanan -> Minuman
    ('11111111-1111-4111-8111-111111111111', 'd0000025-0000-4000-8000-000000000025', 'd0000023-0000-4000-8000-000000000023', 1),
    ('11111111-1111-4111-8111-111111111111', 'd0000025-0000-4000-8000-000000000025', 'd0000001-0000-4000-8000-000000000001', 2),
    ('11111111-1111-4111-8111-111111111111', 'd0000026-0000-4000-8000-000000000026', 'd0000023-0000-4000-8000-000000000023', 1),
    ('11111111-1111-4111-8111-111111111111', 'd0000026-0000-4000-8000-000000000026', 'd0000007-0000-4000-8000-000000000007', 2)
ON CONFLICT (store_id, product_id, recommended_product_id) DO UPDATE SET
    sort_order = EXCLUDED.sort_order;
