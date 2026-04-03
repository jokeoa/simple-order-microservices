package postgres

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jokeoa/simple-order-microservices/internal/order/domain"
	"github.com/jokeoa/simple-order-microservices/internal/order/usecase"
	"github.com/jokeoa/simple-order-microservices/internal/platform/migrate"
)

func TestRepositoryRejectsDuplicateIdempotencyKey(t *testing.T) {
	dsn := os.Getenv("TEST_ORDER_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_ORDER_POSTGRES_DSN is not set")
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
	if _, err := pool.Exec(ctx, `TRUNCATE TABLE orders`); err != nil {
		t.Fatalf("truncate orders: %v", err)
	}

	repo := NewRepository(pool)
	order := domain.Order{ID: "order-1", CustomerID: "cust-1", ItemName: "book", Amount: 100, Status: domain.StatusPending, IdempotencyKey: "key-1", RequestFingerprint: "fp-1"}
	if _, err := repo.Create(ctx, order); err != nil {
		t.Fatalf("repo.Create(first) error = %v", err)
	}

	_, err = repo.Create(ctx, domain.Order{ID: "order-2", CustomerID: "cust-2", ItemName: "pen", Amount: 100, Status: domain.StatusPending, IdempotencyKey: "key-1", RequestFingerprint: "fp-2"})
	if err != usecase.ErrDuplicateIdempotencyKey {
		t.Fatalf("repo.Create(second) error = %v, want %v", err, usecase.ErrDuplicateIdempotencyKey)
	}
}

func TestRepositoryGetRevenueByCustomerIDCountsOnlyPaidOrders(t *testing.T) {
	dsn := os.Getenv("TEST_ORDER_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_ORDER_POSTGRES_DSN is not set")
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
	if _, err := pool.Exec(ctx, `TRUNCATE TABLE orders`); err != nil {
		t.Fatalf("truncate orders: %v", err)
	}

	repo := NewRepository(pool)
	orders := []domain.Order{
		{ID: "order-1", CustomerID: "c1", ItemName: "book", Amount: 50000, Status: domain.StatusPaid, IdempotencyKey: "key-1", RequestFingerprint: "fp-1"},
		{ID: "order-2", CustomerID: "c1", ItemName: "pen", Amount: 25000, Status: domain.StatusPaid, IdempotencyKey: "key-2", RequestFingerprint: "fp-2"},
		{ID: "order-3", CustomerID: "c1", ItemName: "bag", Amount: 99999, Status: domain.StatusFailed, IdempotencyKey: "key-3", RequestFingerprint: "fp-3"},
		{ID: "order-4", CustomerID: "c2", ItemName: "lamp", Amount: 12345, Status: domain.StatusPaid, IdempotencyKey: "key-4", RequestFingerprint: "fp-4"},
	}
	for _, order := range orders {
		if _, err := repo.Create(ctx, order); err != nil {
			t.Fatalf("repo.Create(%s) error = %v", order.ID, err)
		}
	}

	revenue, err := repo.GetRevenueByCustomerID(ctx, "c1")
	if err != nil {
		t.Fatalf("repo.GetRevenueByCustomerID() error = %v", err)
	}
	if revenue.CustomerID != "c1" {
		t.Fatalf("revenue.CustomerID = %s, want c1", revenue.CustomerID)
	}
	if revenue.TotalAmount != 75000 {
		t.Fatalf("revenue.TotalAmount = %d, want 75000", revenue.TotalAmount)
	}
	if revenue.OrdersCount != 2 {
		t.Fatalf("revenue.OrdersCount = %d, want 2", revenue.OrdersCount)
	}
}
