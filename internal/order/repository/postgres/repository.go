package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jokeoa/simple-order-microservices/internal/order/domain"
	"github.com/jokeoa/simple-order-microservices/internal/order/usecase"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Create(ctx context.Context, order domain.Order) (domain.Order, error) {
	const query = `
		INSERT INTO orders (id, customer_id, item_name, amount, status, idempotency_key, request_fingerprint, payment_transaction_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8, ''))
		RETURNING id, customer_id, item_name, amount, status, COALESCE(idempotency_key, ''), COALESCE(request_fingerprint, ''), COALESCE(payment_transaction_id, ''), created_at, updated_at
	`

	var created domain.Order
	if err := r.pool.QueryRow(ctx, query, order.ID, order.CustomerID, order.ItemName, order.Amount, order.Status, order.IdempotencyKey, order.RequestFingerprint, order.PaymentTransactionID).Scan(
		&created.ID,
		&created.CustomerID,
		&created.ItemName,
		&created.Amount,
		&created.Status,
		&created.IdempotencyKey,
		&created.RequestFingerprint,
		&created.PaymentTransactionID,
		&created.CreatedAt,
		&created.UpdatedAt,
	); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.Order{}, usecase.ErrDuplicateIdempotencyKey
		}

		return domain.Order{}, fmt.Errorf("insert order: %w", err)
	}

	return created, nil
}

func (r *Repository) GetByID(ctx context.Context, id string) (domain.Order, error) {
	const query = `
		SELECT id, customer_id, item_name, amount, status, COALESCE(idempotency_key, ''), COALESCE(request_fingerprint, ''), COALESCE(payment_transaction_id, ''), created_at, updated_at
		FROM orders
		WHERE id = $1
	`

	var order domain.Order
	if err := r.pool.QueryRow(ctx, query, id).Scan(
		&order.ID,
		&order.CustomerID,
		&order.ItemName,
		&order.Amount,
		&order.Status,
		&order.IdempotencyKey,
		&order.RequestFingerprint,
		&order.PaymentTransactionID,
		&order.CreatedAt,
		&order.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Order{}, usecase.ErrNotFound
		}

		return domain.Order{}, fmt.Errorf("query order by id: %w", err)
	}

	return order, nil
}

func (r *Repository) GetByIdempotencyKey(ctx context.Context, key string) (domain.Order, error) {
	const query = `
		SELECT id, customer_id, item_name, amount, status, COALESCE(idempotency_key, ''), COALESCE(request_fingerprint, ''), COALESCE(payment_transaction_id, ''), created_at, updated_at
		FROM orders
		WHERE idempotency_key = $1
	`

	var order domain.Order
	if err := r.pool.QueryRow(ctx, query, key).Scan(
		&order.ID,
		&order.CustomerID,
		&order.ItemName,
		&order.Amount,
		&order.Status,
		&order.IdempotencyKey,
		&order.RequestFingerprint,
		&order.PaymentTransactionID,
		&order.CreatedAt,
		&order.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Order{}, usecase.ErrNotFound
		}

		return domain.Order{}, fmt.Errorf("query order by idempotency key: %w", err)
	}

	return order, nil
}

func (r *Repository) UpdatePaymentStatus(ctx context.Context, id string, status domain.Status, transactionID string) (domain.Order, error) {
	const query = `
		UPDATE orders
		SET status = $2,
		    payment_transaction_id = NULLIF($3, ''),
		    updated_at = NOW()
		WHERE id = $1
		RETURNING id, customer_id, item_name, amount, status, COALESCE(idempotency_key, ''), COALESCE(request_fingerprint, ''), COALESCE(payment_transaction_id, ''), created_at, updated_at
	`

	return r.update(ctx, query, id, status, transactionID)
}

func (r *Repository) UpdateStatus(ctx context.Context, id string, status domain.Status) (domain.Order, error) {
	const query = `
		UPDATE orders
		SET status = $2,
		    updated_at = NOW()
		WHERE id = $1
		RETURNING id, customer_id, item_name, amount, status, COALESCE(idempotency_key, ''), COALESCE(request_fingerprint, ''), COALESCE(payment_transaction_id, ''), created_at, updated_at
	`

	return r.update(ctx, query, id, status)
}

func (r *Repository) update(ctx context.Context, query string, args ...any) (domain.Order, error) {
	var order domain.Order
	if err := r.pool.QueryRow(ctx, query, args...).Scan(
		&order.ID,
		&order.CustomerID,
		&order.ItemName,
		&order.Amount,
		&order.Status,
		&order.IdempotencyKey,
		&order.RequestFingerprint,
		&order.PaymentTransactionID,
		&order.CreatedAt,
		&order.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Order{}, usecase.ErrNotFound
		}

		return domain.Order{}, fmt.Errorf("update order: %w", err)
	}

	return order, nil
}
