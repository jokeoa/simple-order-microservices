CREATE TABLE IF NOT EXISTS payments (
    order_id TEXT PRIMARY KEY,
    amount BIGINT NOT NULL,
    status TEXT NOT NULL,
    transaction_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (status IN ('Authorized', 'Declined')),
    CHECK (amount > 0)
);
