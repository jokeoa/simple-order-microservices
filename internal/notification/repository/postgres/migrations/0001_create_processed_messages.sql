CREATE TABLE IF NOT EXISTS processed_messages (
	message_id TEXT PRIMARY KEY,
	order_id TEXT NOT NULL,
	processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
