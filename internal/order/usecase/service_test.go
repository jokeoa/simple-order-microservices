package usecase

import (
	"context"
	"testing"

	"github.com/jokeoa/simple-order-microservices/internal/order/domain"
)

type orderRepoStub struct {
	created          []domain.Order
	byID             map[string]domain.Order
	byIdempotencyKey map[string]domain.Order
	revenue          domain.Revenue
	createErr        error
}

func newOrderRepoStub() *orderRepoStub {
	return &orderRepoStub{
		byID:             map[string]domain.Order{},
		byIdempotencyKey: map[string]domain.Order{},
	}
}

func (s *orderRepoStub) Create(_ context.Context, order domain.Order) (domain.Order, error) {
	if s.createErr != nil {
		return domain.Order{}, s.createErr
	}

	s.created = append(s.created, order)
	s.byID[order.ID] = order
	s.byIdempotencyKey[order.IdempotencyKey] = order
	return order, nil
}

func (s *orderRepoStub) GetByID(_ context.Context, id string) (domain.Order, error) {
	order, ok := s.byID[id]
	if !ok {
		return domain.Order{}, ErrNotFound
	}
	return order, nil
}

func (s *orderRepoStub) GetByIdempotencyKey(_ context.Context, key string) (domain.Order, error) {
	order, ok := s.byIdempotencyKey[key]
	if !ok {
		return domain.Order{}, ErrNotFound
	}
	return order, nil
}

func (s *orderRepoStub) GetRevenueByCustomerID(_ context.Context, customerID string) (domain.Revenue, error) {
	revenue := s.revenue
	revenue.CustomerID = customerID
	return revenue, nil
}

func (s *orderRepoStub) UpdatePaymentStatus(_ context.Context, id string, status domain.Status, transactionID string) (domain.Order, error) {
	order := s.byID[id]
	order.Status = status
	order.PaymentTransactionID = transactionID
	s.byID[id] = order
	s.byIdempotencyKey[order.IdempotencyKey] = order
	return order, nil
}

func (s *orderRepoStub) UpdateStatus(_ context.Context, id string, status domain.Status) (domain.Order, error) {
	order := s.byID[id]
	order.Status = status
	s.byID[id] = order
	s.byIdempotencyKey[order.IdempotencyKey] = order
	return order, nil
}

type paymentAuthorizerStub struct {
	result  PaymentResult
	funcErr error
	calls   int
}

func (s *paymentAuthorizerStub) Authorize(context.Context, PaymentInput) (PaymentResult, error) {
	s.calls++
	return s.result, s.funcErr
}

func TestCreateMarksOrderPaid(t *testing.T) {
	repo := newOrderRepoStub()
	payments := &paymentAuthorizerStub{result: PaymentResult{Status: "Authorized", TransactionID: "tx-1"}}
	service := NewService(repo, payments)

	order, created, err := service.Create(context.Background(), CreateOrderInput{IdempotencyKey: "key-1", CustomerID: "cust-1", ItemName: "book", Amount: 500})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if !created {
		t.Fatalf("Create() created = false, want true")
	}
	if order.Status != domain.StatusPaid {
		t.Fatalf("order.Status = %s, want %s", order.Status, domain.StatusPaid)
	}
	if order.PaymentTransactionID != "tx-1" {
		t.Fatalf("order.PaymentTransactionID = %q, want tx-1", order.PaymentTransactionID)
	}
}

func TestCreateReturnsPendingOnPaymentUnavailable(t *testing.T) {
	repo := newOrderRepoStub()
	payments := &paymentAuthorizerStub{funcErr: ErrPaymentUnavailable}
	service := NewService(repo, payments)

	order, created, err := service.Create(context.Background(), CreateOrderInput{IdempotencyKey: "key-1", CustomerID: "cust-1", ItemName: "book", Amount: 500})
	if err != ErrPaymentUnavailable {
		t.Fatalf("Create() error = %v, want %v", err, ErrPaymentUnavailable)
	}
	if !created {
		t.Fatalf("Create() created = false, want true")
	}
	if order.Status != domain.StatusPending {
		t.Fatalf("order.Status = %s, want %s", order.Status, domain.StatusPending)
	}
}

func TestCreateReplaysExistingPendingOrder(t *testing.T) {
	repo := newOrderRepoStub()
	service := NewService(repo, &paymentAuthorizerStub{funcErr: ErrPaymentUnavailable})

	first, _, err := service.Create(context.Background(), CreateOrderInput{IdempotencyKey: "key-1", CustomerID: "cust-1", ItemName: "book", Amount: 500})
	if err != ErrPaymentUnavailable {
		t.Fatalf("first Create() error = %v, want %v", err, ErrPaymentUnavailable)
	}

	payments := &paymentAuthorizerStub{result: PaymentResult{Status: "Authorized", TransactionID: "tx-2"}}
	service = NewService(repo, payments)

	order, created, err := service.Create(context.Background(), CreateOrderInput{IdempotencyKey: "key-1", CustomerID: "cust-1", ItemName: "book", Amount: 500})
	if err != nil {
		t.Fatalf("second Create() error = %v", err)
	}
	if created {
		t.Fatalf("second Create() created = true, want false")
	}
	if order.ID != first.ID {
		t.Fatalf("order.ID = %s, want %s", order.ID, first.ID)
	}
	if order.Status != domain.StatusPaid {
		t.Fatalf("order.Status = %s, want %s", order.Status, domain.StatusPaid)
	}
	if len(repo.created) != 1 {
		t.Fatalf("len(repo.created) = %d, want 1", len(repo.created))
	}
}

func TestCreateRejectsPayloadMismatchForSameIdempotencyKey(t *testing.T) {
	repo := newOrderRepoStub()
	repo.byIdempotencyKey["key-1"] = domain.Order{ID: "order-1", IdempotencyKey: "key-1", RequestFingerprint: "fingerprint-a", Status: domain.StatusPending}
	service := NewService(repo, &paymentAuthorizerStub{})

	_, _, err := service.Create(context.Background(), CreateOrderInput{IdempotencyKey: "key-1", CustomerID: "cust-2", ItemName: "pen", Amount: 100})
	if err != ErrIdempotencyConflict {
		t.Fatalf("Create() error = %v, want %v", err, ErrIdempotencyConflict)
	}
}

func TestCancelRejectsPaidOrder(t *testing.T) {
	repo := newOrderRepoStub()
	repo.byID["order-1"] = domain.Order{ID: "order-1", Status: domain.StatusPaid}
	service := NewService(repo, &paymentAuthorizerStub{})

	_, err := service.Cancel(context.Background(), "order-1")
	if err != ErrConflict {
		t.Fatalf("Cancel() error = %v, want %v", err, ErrConflict)
	}
}

func TestGetRevenueByCustomerID(t *testing.T) {
	repo := newOrderRepoStub()
	repo.revenue = domain.Revenue{TotalAmount: 75000, OrdersCount: 5}
	service := NewService(repo, &paymentAuthorizerStub{})

	revenue, err := service.GetRevenueByCustomerID(context.Background(), "c1")
	if err != nil {
		t.Fatalf("GetRevenueByCustomerID() error = %v", err)
	}
	if revenue.CustomerID != "c1" {
		t.Fatalf("revenue.CustomerID = %s, want c1", revenue.CustomerID)
	}
	if revenue.TotalAmount != 75000 {
		t.Fatalf("revenue.TotalAmount = %d, want 75000", revenue.TotalAmount)
	}
	if revenue.OrdersCount != 5 {
		t.Fatalf("revenue.OrdersCount = %d, want 5", revenue.OrdersCount)
	}
}
