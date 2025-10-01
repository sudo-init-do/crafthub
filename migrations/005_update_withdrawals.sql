ALTER TABLE withdrawals
ADD COLUMN approved_by UUID NULL REFERENCES users(id),
ADD COLUMN approved_at TIMESTAMP NULL,
ALTER COLUMN status SET DEFAULT 'pending',
ADD CONSTRAINT withdrawals_status_check CHECK (status IN ('pending', 'approved', 'rejected', 'completed'));
