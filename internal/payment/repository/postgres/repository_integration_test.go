package postgres

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jokeoa/simple-order-microservices/internal/payment/domain"
	"github.com/jokeoa/simple-order-microservices/internal/platform/migrate"
)

func TestRepositoryPersistsPayment(t *testing.T) {
	dsn := os.Getenv("TEST_PAYMENT_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_PAYMENT_POSTGRES_DSN is not set")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New() error = %v", err)
	}
	defer pool.Close()

	if err := migrate.Run(ctx, pool, Migrations()); err != nil {
		t.Fatalf("migrate.Run() error = %v", err)
	}
	if _, err := pool.Exec(ctx, `TRUNCATE TABLE payments`); err != nil {
		t.Fatalf("truncate payments: %v", err)
	}

	repo := NewRepository(pool)
	created, err := repo.Create(ctx, domain.Payment{OrderID: "order-1", Amount: 100, Status: domain.StatusAuthorized, TransactionID: "tx-1"})
	if err != nil {
		t.Fatalf("repo.Create() error = %v", err)
	}

	loaded, err := repo.GetByOrderID(ctx, created.OrderID)
	if err != nil {
		t.Fatalf("repo.GetByOrderID() error = %v", err)
	}
	if loaded.TransactionID != "tx-1" {
		t.Fatalf("loaded.TransactionID = %q, want tx-1", loaded.TransactionID)
	}
}
