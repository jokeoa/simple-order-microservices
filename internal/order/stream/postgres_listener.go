package stream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jokeoa/simple-order-microservices/internal/order/domain"
)

const orderUpdatesChannel = "order_updates"

type listenerPayload struct {
	OrderID              string `json:"order_id"`
	Status               string `json:"status"`
	PaymentTransactionID string `json:"payment_transaction_id"`
	Timestamp            string `json:"timestamp"`
}

type PostgresListener struct {
	dsn    string
	broker *Broker
	logger *log.Logger
}

func NewPostgresListener(dsn string, broker *Broker, logger *log.Logger) *PostgresListener {
	if logger == nil {
		logger = log.Default()
	}

	return &PostgresListener{
		dsn:    dsn,
		broker: broker,
		logger: logger,
	}
}

func (l *PostgresListener) Start(ctx context.Context) error {
	connection, err := l.listen(ctx)
	if err != nil {
		return err
	}

	go l.run(ctx, connection)
	return nil
}

func (l *PostgresListener) run(ctx context.Context, connection *pgx.Conn) {
	for {
		if err := l.readNotifications(ctx, connection); err != nil {
			_ = connection.Close(context.Background())
			if ctx.Err() != nil {
				return
			}

			l.logger.Printf("order updates listener reconnecting after error: %v", err)
			nextConnection, reconnectErr := l.reconnect(ctx)
			if reconnectErr != nil {
				if ctx.Err() != nil {
					return
				}
				l.logger.Printf("order updates listener retry failed: %v", reconnectErr)
				continue
			}

			if err := l.publishSnapshots(ctx, nextConnection); err != nil {
				l.logger.Printf("order updates listener snapshot sync failed: %v", err)
			}

			connection = nextConnection
			continue
		}

		_ = connection.Close(context.Background())
		return
	}
}

func (l *PostgresListener) listen(ctx context.Context) (*pgx.Conn, error) {
	connection, err := pgx.Connect(ctx, l.dsn)
	if err != nil {
		return nil, fmt.Errorf("connect order notifications listener: %w", err)
	}

	if _, err := connection.Exec(ctx, "LISTEN "+orderUpdatesChannel); err != nil {
		_ = connection.Close(ctx)
		return nil, fmt.Errorf("listen for order updates: %w", err)
	}

	return connection, nil
}

func (l *PostgresListener) readNotifications(ctx context.Context, connection *pgx.Conn) error {
	for {
		notification, err := connection.WaitForNotification(ctx)
		if err != nil {
			return err
		}

		update, err := decodeUpdate(notification.Payload)
		if err != nil {
			l.logger.Printf("discard invalid order update payload: %v", err)
			continue
		}

		l.broker.Publish(update)
	}
}

func (l *PostgresListener) reconnect(ctx context.Context) (*pgx.Conn, error) {
	timer := time.NewTimer(time.Second)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
		}

		connection, err := l.listen(ctx)
		if err == nil {
			return connection, nil
		}

		l.logger.Printf("order updates listener waiting to retry: %v", err)
		timer.Reset(time.Second)
	}
}

func (l *PostgresListener) publishSnapshots(ctx context.Context, connection *pgx.Conn) error {
	for _, orderID := range l.broker.OrderIDs() {
		update, err := loadOrderSnapshot(ctx, connection, orderID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			return err
		}
		l.broker.Publish(update)
	}

	return nil
}

func loadOrderSnapshot(ctx context.Context, connection *pgx.Conn, orderID string) (Update, error) {
	const query = `
		SELECT id, status, COALESCE(payment_transaction_id, ''), updated_at
		FROM orders
		WHERE id = $1
	`

	var update Update
	if err := connection.QueryRow(ctx, query, orderID).Scan(
		&update.OrderID,
		&update.Status,
		&update.PaymentTransactionID,
		&update.Timestamp,
	); err != nil {
		return Update{}, fmt.Errorf("load order snapshot %s: %w", orderID, err)
	}

	return update, nil
}

func decodeUpdate(payload string) (Update, error) {
	var notification listenerPayload
	if err := json.Unmarshal([]byte(payload), &notification); err != nil {
		return Update{}, fmt.Errorf("unmarshal notification: %w", err)
	}

	timestamp, err := time.Parse(time.RFC3339Nano, notification.Timestamp)
	if err != nil {
		return Update{}, fmt.Errorf("parse notification timestamp: %w", err)
	}

	return Update{
		OrderID:              notification.OrderID,
		Status:               domain.Status(notification.Status),
		PaymentTransactionID: notification.PaymentTransactionID,
		Timestamp:            timestamp,
	}, nil
}
