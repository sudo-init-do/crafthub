-- Add delivery time and status for moderation and discovery

ALTER TABLE services
    ADD COLUMN IF NOT EXISTS delivery_time_days INTEGER CHECK (delivery_time_days >= 0),
    ADD COLUMN IF NOT EXISTS status TEXT DEFAULT 'active' CHECK (status IN ('active','suspended','pending'));

CREATE INDEX IF NOT EXISTS idx_services_status ON services(status);
CREATE INDEX IF NOT EXISTS idx_services_category ON services(category);
