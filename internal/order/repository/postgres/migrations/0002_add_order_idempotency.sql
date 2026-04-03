ALTER TABLE orders
    ADD COLUMN IF NOT EXISTS idempotency_key TEXT,
    ADD COLUMN IF NOT EXISTS request_fingerprint TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS orders_idempotency_key_unique_idx
    ON orders (idempotency_key)
    WHERE idempotency_key IS NOT NULL;
