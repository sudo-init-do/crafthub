ALTER TABLE withdrawals
ADD COLUMN approved_by UUID,
ADD COLUMN approved_at TIMESTAMP;
