-- Rollback Guest PWA v4 feature flags and split share cashier request column.

ALTER TABLE payment_splits
    DROP COLUMN IF EXISTS cashier_requested_at;

DELETE FROM store_feature_flags
WHERE flag_key IN (
    'guest_self_cancel_enabled',
    'guest_split_equal_enabled',
    'guest_split_by_item_enabled',
    'guest_split_multi_payment_enabled',
    'guest_bell_sound_enabled',
    'cart_pairing_enabled',
    'guest_email_receipt_enabled'
);

ALTER TABLE store_feature_flags DROP CONSTRAINT IF EXISTS chk_store_feature_flags_key;
ALTER TABLE store_feature_flags ADD CONSTRAINT chk_store_feature_flags_key CHECK (flag_key IN (
    'call_waiter',
    'kds',
    'queue_display',
    'dual_screen',
    'open_bill',
    'split_bill',
    'stock',
    'whatsapp'
));
