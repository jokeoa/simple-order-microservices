package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) MarkProcessed(ctx context.Context, messageID, orderID string) (bool, error) {
	const query = `
		INSERT INTO processed_messages (message_id, order_id)
		VALUES ($1, $2)
		ON CONFLICT (message_id) DO NOTHING
	`

	tag, err := r.pool.Exec(ctx, query, messageID, orderID)
	if err != nil {
		return false, fmt.Errorf("mark message processed: %w", err)
	}

	return tag.RowsAffected() == 1, nil
}
