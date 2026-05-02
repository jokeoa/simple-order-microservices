package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/jokeoa/simple-order-microservices/internal/payment/domain"
	"github.com/jokeoa/simple-order-microservices/internal/platform/events"
)

type paymentRepoStub struct {
	getResult  domain.Payment
	getErr     error
	findResult []domain.Payment
	findErr    error
	findMin    int64
	findMax    int64
	createFunc func(domain.Payment) (domain.Payment, error)
}

type paymentPublisherStub struct {
	event events.PaymentCompleted
	err   error
	calls int
}

func (s *paymentPublisherStub) PublishPaymentCompleted(_ context.Context, event events.PaymentCompleted) error {
	s.event = event
	s.calls++
	return s.err
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

func (s *paymentRepoStub) FindByAmountRange(_ context.Context, min, max int64) ([]domain.Payment, error) {
	s.findMin = min
	s.findMax = max
	return s.findResult, s.findErr
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

func TestAuthorizePublishesPaymentCompletedEvent(t *testing.T) {
	repo := &paymentRepoStub{getErr: ErrNotFound}
	publisher := &paymentPublisherStub{}
	service := NewService(repo, publisher)

	payment, created, err := service.Authorize(context.Background(), AuthorizeInput{
		OrderID:       "order-1",
		Amount:        500,
		CustomerEmail: "customer@example.com",
	})
	if err != nil {
		t.Fatalf("Authorize() error = %v", err)
	}
	if !created {
		t.Fatalf("Authorize() created = false, want true")
	}
	if publisher.calls != 1 {
		t.Fatalf("PublishPaymentCompleted() calls = %d, want 1", publisher.calls)
	}
	if publisher.event.MessageID != "payment.completed:order-1" {
		t.Fatalf("event.MessageID = %q, want payment.completed:order-1", publisher.event.MessageID)
	}
	if publisher.event.OrderID != payment.OrderID || publisher.event.Amount != payment.Amount {
		t.Fatalf("event payment fields = %#v, want order_id=%s amount=%d", publisher.event, payment.OrderID, payment.Amount)
	}
	if publisher.event.CustomerEmail != "customer@example.com" {
		t.Fatalf("event.CustomerEmail = %q, want customer@example.com", publisher.event.CustomerEmail)
	}
	if publisher.event.Status != string(domain.StatusAuthorized) {
		t.Fatalf("event.Status = %q, want %s", publisher.event.Status, domain.StatusAuthorized)
	}
}

func TestAuthorizeReturnsUnavailableWhenEventPublishFails(t *testing.T) {
	repo := &paymentRepoStub{getErr: ErrNotFound}
	service := NewService(repo, &paymentPublisherStub{err: errors.New("nats unavailable")})

	_, _, err := service.Authorize(context.Background(), AuthorizeInput{OrderID: "order-1", Amount: 500})
	if !errors.Is(err, ErrEventPublishFailed) {
		t.Fatalf("Authorize() error = %v, want %v", err, ErrEventPublishFailed)
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

func TestListPaymentsReturnsFilteredPayments(t *testing.T) {
	repo := &paymentRepoStub{
		findResult: []domain.Payment{
			{OrderID: "order-2", Amount: 300},
			{OrderID: "order-1", Amount: 200},
		},
	}
	service := NewService(repo)

	payments, err := service.ListPayments(context.Background(), 200, 300)
	if err != nil {
		t.Fatalf("ListPayments() error = %v", err)
	}
	if repo.findMin != 200 || repo.findMax != 300 {
		t.Fatalf("ListPayments() forwarded range = [%d,%d], want [200,300]", repo.findMin, repo.findMax)
	}
	if len(payments) != 2 {
		t.Fatalf("ListPayments() len = %d, want 2", len(payments))
	}
	if payments[0].OrderID != "order-2" || payments[1].OrderID != "order-1" {
		t.Fatalf("ListPayments() unexpected payments = %#v", payments)
	}
}

func TestListPaymentsReturnsAllForUnboundedRange(t *testing.T) {
	repo := &paymentRepoStub{
		findResult: []domain.Payment{{OrderID: "order-1", Amount: 100}},
	}
	service := NewService(repo)

	payments, err := service.ListPayments(context.Background(), 0, 0)
	if err != nil {
		t.Fatalf("ListPayments() error = %v", err)
	}
	if repo.findMin != 0 || repo.findMax != 0 {
		t.Fatalf("ListPayments() forwarded range = [%d,%d], want [0,0]", repo.findMin, repo.findMax)
	}
	if len(payments) != 1 {
		t.Fatalf("ListPayments() len = %d, want 1", len(payments))
	}
}

func TestListPaymentsRejectsInvalidRanges(t *testing.T) {
	service := NewService(&paymentRepoStub{})

	tests := []struct {
		name string
		min  int64
		max  int64
	}{
		{name: "negative min", min: -1, max: 0},
		{name: "negative max", min: 0, max: -1},
		{name: "min greater than max", min: 300, max: 200},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.ListPayments(context.Background(), tt.min, tt.max)
			if !errors.Is(err, ErrInvalidAmountRange) {
				t.Fatalf("ListPayments() error = %v, want %v", err, ErrInvalidAmountRange)
			}
		})
	}
}
