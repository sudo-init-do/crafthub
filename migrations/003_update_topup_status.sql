-- 003_update_topup_status.sql
ALTER TABLE topups DROP CONSTRAINT IF EXISTS topups_status_check;

ALTER TABLE topups
    ADD CONSTRAINT topups_status_check
    CHECK (status IN ('pending', 'completed'));
