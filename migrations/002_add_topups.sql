CREATE TABLE IF NOT EXISTS topups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    amount BIGINT NOT NULL,
    status TEXT CHECK (status IN ('pending','success','failed')) DEFAULT 'pending',
    provider_ref TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);
