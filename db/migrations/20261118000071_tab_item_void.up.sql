-- 20261118000071_tab_item_void.up.sql
-- Void part or all of one line on an open tab (A1). Adds the void_item approval action (ADR-007) and the
-- VOID_ORDER_ITEM audit action. No new column: an open tab has no splits or refund lines yet, so the line
-- quantity and subtotal shrink in place (or the line is removed) and the audit row keeps what was voided.
-- The runtime role gets DELETE on order_items only through a RESTRICTIVE policy: a line can be removed
-- while its order is an open tab, never once it is closed, paid or a pay-first order.

ALTER TABLE approval_requests DROP CONSTRAINT IF EXISTS chk_approval_requests_action;
ALTER TABLE approval_requests ADD CONSTRAINT chk_approval_requests_action CHECK (
    action IN ('void', 'void_item', 'refund', 'stock_override', 'shift_variance', 'outlet_expense', 'discount'));

ALTER TABLE audit_logs DROP CONSTRAINT IF EXISTS chk_audit_logs_action;
ALTER TABLE audit_logs ADD CONSTRAINT chk_audit_logs_action CHECK (
    action IN (
        'VOID_ORDER', 'VOID_ORDER_ITEM', 'REFUND', 'STOCK_OVERRIDE', 'PRICE_CHANGE',
        'SHIFT_OPEN', 'SHIFT_CLOSE', 'SHIFT_FORCE_CLOSE',
        'ROLE_CHANGE', 'STATIC_QRIS_RECONCILE', 'CASH_PAYMENT_RECORDED', 'EDC_RECONCILE',
        'ORDER_CREATE', 'ORDER_CANCEL', 'PAYMENT_SETTLED',
        'STAFF_LOGIN_LOCKED', 'STAFF_UNLOCK', 'STAFF_PIN_RESET', 'STAFF_DEACTIVATED',
        'STAFF_SESSION_REVOKED', 'STAFF_INVITED', 'STAFF_PIN_SET', 'STAFF_TEMP_PIN_ISSUED',
        'STORE_BOOTSTRAP', 'STORE_SETTINGS_CHANGED',
        'CASH_MOVEMENT', 'DRAWER_CREATE', 'DRAWER_UPDATE',
        'TAB_OPEN', 'ROUND_SUBMIT', 'TAB_CLOSE', 'TAB_REOPEN',
        'OWNER_PIN_FAILED',
        'TABLE_CREATE', 'TABLE_UPDATE', 'TABLE_QR_ROTATE', 'TABLE_SESSION_CLOSE',
        'PRODUCT_IMAGE_CHANGED',
        'APPROVAL_REQUESTED', 'APPROVAL_APPROVED', 'APPROVAL_REJECTED',
        'STAFF_PHONE_SET', 'CUSTOMER_PHONE_VERIFIED',
        'OUTLET_CASH_MOVEMENT',
        'ORDER_DISCOUNT',
        'STAFF_BADGE_ISSUED', 'STAFF_BADGE_REVOKED',
        'TABLE_MOVE', 'TABLE_MERGE', 'TABLE_SPLIT',
        'TABLE_TIMER_EXTEND',
        'DISPLAY_TOKEN_CREATED', 'DISPLAY_TOKEN_REVOKED',
        'PRINTER_CREATED', 'PRINTER_UPDATED', 'PRINTER_TOKEN_ROTATED', 'PRINT_REPRINT',
        'RECEIPT_SENT',
        'DEVICE_REGISTERED', 'DEVICE_RENAMED', 'DEVICE_REVOKED', 'DEVICE_ROLE_CHANGED', 'FEATURE_FLAG_CHANGED', 'DEVICE_PIN_VERIFIER_ISSUED',
        'OFFLINE_ORDER_SYNCED', 'OFFLINE_ORDER_REJECTED', 'OFFLINE_ORDER_RESOLVED', 'DEVICE_PIN_FAILED',
        'PROMOTION_CREATED', 'PROMOTION_UPDATED',
        'BUNDLE_CHANGED',
        'INGREDIENT_CREATED', 'INGREDIENT_UPDATED', 'RECIPE_CHANGED',
        'SUPPLIER_CHANGED', 'PURCHASE_ORDER_CHANGED', 'GOODS_RECEIVED',
        'PAYABLE_PAID',
        'STOCK_SESSION_SUBMITTED', 'STOCK_SESSION_APPROVED', 'STOCK_SESSION_REJECTED',
        'SALES_CHANNEL_CHANGED', 'CHANNEL_PRICES_CHANGED', 'PLATFORM_PAYMENT_RECORDED',
        'STAMP_PROGRAM_CHANGED', 'STAMP_REWARD_REDEEMED',
        'BUSINESS_CUSTOMER_CHANGED', 'INVOICE_CHARGED', 'INVOICE_ISSUED', 'INVOICE_PAYMENT_RECORDED'
    )
);

GRANT DELETE ON order_items TO cassy_app;
DROP POLICY IF EXISTS p_order_items_delete_open_tab_only ON order_items;
CREATE POLICY p_order_items_delete_open_tab_only ON order_items AS RESTRICTIVE FOR DELETE
    USING (EXISTS (SELECT 1 FROM orders o
        WHERE o.store_id = order_items.store_id AND o.id = order_items.order_id
          AND o.billing_mode = 'open_tab' AND o.status = 'open'));

-- A reopened tab keeps the item rows of its cancelled by_item splits, and their RESTRICT key would
-- block removing a voided line; only rows of cancelled splits may go.
GRANT DELETE ON payment_split_items TO cassy_app;
DROP POLICY IF EXISTS p_payment_split_items_delete_cancelled_only ON payment_split_items;
CREATE POLICY p_payment_split_items_delete_cancelled_only ON payment_split_items AS RESTRICTIVE FOR DELETE
    USING (EXISTS (SELECT 1 FROM payment_splits ps
        WHERE ps.store_id = payment_split_items.store_id AND ps.id = payment_split_items.split_id
          AND ps.status = 'cancelled'));
