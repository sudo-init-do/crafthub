-- Migration to support order holds, escrow lifecycle, and order references

-- Add wallets.locked_amount to track reserved funds on order creation
ALTER TABLE wallets
    ADD COLUMN IF NOT EXISTS locked_amount BIGINT NOT NULL DEFAULT 0;

-- Ensure locked_amount is non-negative
ALTER TABLE wallets
    ADD CONSTRAINT IF NOT EXISTS check_wallet_locked_amount
        CHECK (locked_amount >= 0);

-- Add transactions.reference to link to related entities (e.g., order id)
ALTER TABLE transactions
    ADD COLUMN IF NOT EXISTS reference UUID NULL;

CREATE INDEX IF NOT EXISTS idx_transactions_reference ON transactions(reference);

-- Broaden allowed transaction statuses to cover holds and refunds
-- Drop previous constraint if it exists, then add an expanded one
ALTER TABLE transactions DROP CONSTRAINT IF EXISTS transactions_status_check;
ALTER TABLE transactions DROP CONSTRAINT IF EXISTS transactions_status_allowed;
ALTER TABLE transactions
    ADD CONSTRAINT transactions_status_allowed
        CHECK (status IN (
            'pending',
            'completed',
            'failed',
            'pending_hold',
            'debited',
            'credited',
            'refunded'
        ));

-- Align order statuses to new lifecycle terms
ALTER TABLE orders DROP CONSTRAINT IF EXISTS orders_status_check;
ALTER TABLE orders DROP CONSTRAINT IF EXISTS orders_status_allowed;
ALTER TABLE orders
    ADD CONSTRAINT orders_status_allowed
        CHECK (status IN (
            'pending_acceptance',
            'in_progress',
            'delivered',
            'declined',
            'completed',
            'canceled'
        ));

-- Add orders.platform_fee for future scaling (default 0)
ALTER TABLE orders
    ADD COLUMN IF NOT EXISTS platform_fee BIGINT NOT NULL DEFAULT 0;
