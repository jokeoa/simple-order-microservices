package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

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

func (r *Repository) FindByAmountRange(ctx context.Context, min, max int64) ([]domain.Payment, error) {
	query := strings.Builder{}
	query.WriteString(`
		SELECT order_id, amount, status, COALESCE(transaction_id, ''), created_at, updated_at
		FROM payments
	`)

	args := make([]any, 0, 2)
	conditions := make([]string, 0, 2)
	if min > 0 {
		args = append(args, min)
		conditions = append(conditions, fmt.Sprintf("amount >= $%d", len(args)))
	}
	if max > 0 {
		args = append(args, max)
		conditions = append(conditions, fmt.Sprintf("amount <= $%d", len(args)))
	}
	if len(conditions) > 0 {
		query.WriteString(" WHERE ")
		query.WriteString(strings.Join(conditions, " AND "))
	}
	query.WriteString(" ORDER BY created_at DESC, order_id ASC")

	rows, err := r.pool.Query(ctx, query.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("query payments by amount range: %w", err)
	}
	defer rows.Close()

	payments := make([]domain.Payment, 0)
	for rows.Next() {
		var payment domain.Payment
		if err := rows.Scan(
			&payment.OrderID,
			&payment.Amount,
			&payment.Status,
			&payment.TransactionID,
			&payment.CreatedAt,
			&payment.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan payment by amount range: %w", err)
		}
		payments = append(payments, payment)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate payments by amount range: %w", err)
	}

	return payments, nil
}
