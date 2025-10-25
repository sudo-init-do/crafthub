-- Add escrow column to wallets table

ALTER TABLE wallets ADD COLUMN escrow BIGINT NOT NULL DEFAULT 0 CHECK (escrow >= 0);

-- Add index on escrow column for performance
CREATE INDEX IF NOT EXISTS idx_wallets_escrow ON wallets(escrow);

-- Add constraint to ensure balance + escrow is reasonable
ALTER TABLE wallets ADD CONSTRAINT check_wallet_amounts CHECK (balance >= 0 AND escrow >= 0);
