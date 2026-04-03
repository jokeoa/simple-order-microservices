package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jokeoa/simple-order-microservices/internal/payment/domain"
	"github.com/jokeoa/simple-order-microservices/internal/payment/usecase"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) GetByOrderID(ctx context.Context, orderID string) (domain.Payment, error) {
	const query = `
		SELECT order_id, amount, status, COALESCE(transaction_id, ''), created_at, updated_at
		FROM payments
		WHERE order_id = $1
	`

	var payment domain.Payment
	if err := r.pool.QueryRow(ctx, query, orderID).Scan(
		&payment.OrderID,
		&payment.Amount,
		&payment.Status,
		&payment.TransactionID,
		&payment.CreatedAt,
		&payment.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Payment{}, usecase.ErrNotFound
		}

		return domain.Payment{}, fmt.Errorf("query payment by order id: %w", err)
	}

	return payment, nil
}

func (r *Repository) Create(ctx context.Context, payment domain.Payment) (domain.Payment, error) {
	const query = `
		INSERT INTO payments (order_id, amount, status, transaction_id)
		VALUES ($1, $2, $3, NULLIF($4, ''))
		RETURNING order_id, amount, status, COALESCE(transaction_id, ''), created_at, updated_at
	`

	var created domain.Payment
	if err := r.pool.QueryRow(ctx, query, payment.OrderID, payment.Amount, payment.Status, payment.TransactionID).Scan(
		&created.OrderID,
		&created.Amount,
		&created.Status,
		&created.TransactionID,
		&created.CreatedAt,
		&created.UpdatedAt,
	); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.Payment{}, fmt.Errorf("duplicate payment: %w", err)
		}

		return domain.Payment{}, fmt.Errorf("insert payment: %w", err)
	}

	return created, nil
}
