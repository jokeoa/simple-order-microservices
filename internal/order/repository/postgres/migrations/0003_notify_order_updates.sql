CREATE OR REPLACE FUNCTION notify_order_updates()
RETURNS TRIGGER AS $$
BEGIN
    PERFORM pg_notify(
        'order_updates',
        json_build_object(
            'order_id', NEW.id,
            'status', NEW.status,
            'payment_transaction_id', COALESCE(NEW.payment_transaction_id, ''),
            'timestamp', to_char(NEW.updated_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"')
        )::text
    );

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS orders_notify_updates ON orders;

CREATE TRIGGER orders_notify_updates
AFTER INSERT OR UPDATE OF status, payment_transaction_id ON orders
FOR EACH ROW
EXECUTE FUNCTION notify_order_updates();
