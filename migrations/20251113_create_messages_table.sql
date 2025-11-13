-- Messaging per-order threads
-- Each order has its own conversation between buyer and seller

CREATE TABLE IF NOT EXISTS messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    sender_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    recipient_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    read_at TIMESTAMP WITH TIME ZONE NULL
);

-- Efficient lookups for an order's conversation
CREATE INDEX IF NOT EXISTS idx_messages_order_id_created_at
    ON messages(order_id, created_at);

-- Unread tracking for recipients
CREATE INDEX IF NOT EXISTS idx_messages_recipient_unread
    ON messages(recipient_id)
    WHERE read_at IS NULL;

