package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/jokeoa/simple-order-microservices/internal/payment/domain"
)

type paymentRepoStub struct {
	getResult  domain.Payment
	getErr     error
	createFunc func(domain.Payment) (domain.Payment, error)
}

func (s *paymentRepoStub) GetByOrderID(context.Context, string) (domain.Payment, error) {
	return s.getResult, s.getErr
}

func (s *paymentRepoStub) Create(_ context.Context, payment domain.Payment) (domain.Payment, error) {
	if s.createFunc != nil {
		return s.createFunc(payment)
	}
	return payment, nil
}

func TestAuthorizeDeclinesLargeAmount(t *testing.T) {
	repo := &paymentRepoStub{getErr: ErrNotFound}
	service := NewService(repo)

	payment, created, err := service.Authorize(context.Background(), AuthorizeInput{OrderID: "order-1", Amount: 100001})
	if err != nil {
		t.Fatalf("Authorize() error = %v", err)
	}
	if !created {
		t.Fatalf("Authorize() created = false, want true")
	}
	if payment.Status != domain.StatusDeclined {
		t.Fatalf("payment.Status = %s, want %s", payment.Status, domain.StatusDeclined)
	}
	if payment.TransactionID != "" {
		t.Fatalf("payment.TransactionID = %q, want empty", payment.TransactionID)
	}
}

func TestAuthorizeReturnsExistingPayment(t *testing.T) {
	existing := domain.Payment{OrderID: "order-1", Amount: 500, Status: domain.StatusAuthorized, TransactionID: "tx-1"}
	repo := &paymentRepoStub{getResult: existing}
	service := NewService(repo)

	payment, created, err := service.Authorize(context.Background(), AuthorizeInput{OrderID: "order-1", Amount: 500})
	if err != nil {
		t.Fatalf("Authorize() error = %v", err)
	}
	if created {
		t.Fatalf("Authorize() created = true, want false")
	}
	if payment != existing {
		t.Fatalf("Authorize() payment = %#v, want %#v", payment, existing)
	}
}

func TestAuthorizeReloadsOnConcurrentDuplicate(t *testing.T) {
	existing := domain.Payment{OrderID: "order-1", Amount: 500, Status: domain.StatusAuthorized, TransactionID: "tx-1"}
	repo := &paymentRepoStub{
		getErr: ErrNotFound,
		createFunc: func(domain.Payment) (domain.Payment, error) {
			return domain.Payment{}, errors.New("duplicate key")
		},
	}
	service := NewService(repo)

	repo.getResult = existing
	repo.getErr = nil

	payment, created, err := service.Authorize(context.Background(), AuthorizeInput{OrderID: "order-1", Amount: 500})
	if err != nil {
		t.Fatalf("Authorize() error = %v", err)
	}
	if created {
		t.Fatalf("Authorize() created = true, want false")
	}
	if payment != existing {
		t.Fatalf("Authorize() payment = %#v, want %#v", payment, existing)
	}
}
