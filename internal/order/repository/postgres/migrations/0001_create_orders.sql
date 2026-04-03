CREATE TABLE IF NOT EXISTS orders (
    id TEXT PRIMARY KEY,
    customer_id TEXT NOT NULL,
    item_name TEXT NOT NULL,
    amount BIGINT NOT NULL,
    status TEXT NOT NULL,
    payment_transaction_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (status IN ('Pending', 'Paid', 'Failed', 'Cancelled')),
    CHECK (amount > 0)
);
