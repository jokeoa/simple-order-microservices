package grpc

import (
	"context"
	"errors"
	"testing"
	"time"

	paymentv1 "github.com/jokeoa/simple-order-microservices/internal/gen/payment/v1"
	"github.com/jokeoa/simple-order-microservices/internal/payment/domain"
	"github.com/jokeoa/simple-order-microservices/internal/payment/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type stubPaymentService struct {
	payment  domain.Payment
	payments []domain.Payment
	created  bool
	err      error
	input    usecase.AuthorizeInput
	listMin  int64
	listMax  int64
}

func (s *stubPaymentService) Authorize(_ context.Context, input usecase.AuthorizeInput) (domain.Payment, bool, error) {
	s.input = input
	return s.payment, s.created, s.err
}

func (s *stubPaymentService) ListPayments(_ context.Context, min, max int64) ([]domain.Payment, error) {
	s.listMin = min
	s.listMax = max
	return s.payments, s.err
}

func TestServerProcessPaymentReturnsPaymentResponse(t *testing.T) {
	t.Parallel()

	processedAt := time.Date(2026, time.April, 13, 12, 0, 0, 0, time.UTC)
	service := &stubPaymentService{
		payment: domain.Payment{
			OrderID:       "order-1",
			Amount:        500,
			Status:        domain.StatusAuthorized,
			TransactionID: "txn-1",
			UpdatedAt:     processedAt,
		},
		created: true,
	}

	server := NewServer(service)

	response, err := server.ProcessPayment(context.Background(), &paymentv1.PaymentRequest{
		OrderId: "order-1",
		Amount:  500,
	})
	if err != nil {
		t.Fatalf("ProcessPayment returned error: %v", err)
	}

	if service.input.OrderID != "order-1" || service.input.Amount != 500 {
		t.Fatalf("ProcessPayment forwarded wrong input: %+v", service.input)
	}
	if response.GetOrderId() != "order-1" {
		t.Fatalf("unexpected order id: %q", response.GetOrderId())
	}
	if response.GetStatus() != string(domain.StatusAuthorized) {
		t.Fatalf("unexpected status: %q", response.GetStatus())
	}
	if response.GetTransactionId() != "txn-1" {
		t.Fatalf("unexpected transaction id: %q", response.GetTransactionId())
	}
	if got := response.GetProcessedAt().AsTime(); !got.Equal(processedAt) {
		t.Fatalf("unexpected processed_at: %v", got)
	}
}

func TestServerProcessPaymentValidatesRequest(t *testing.T) {
	t.Parallel()

	server := NewServer(&stubPaymentService{})

	_, err := server.ProcessPayment(context.Background(), &paymentv1.PaymentRequest{
		OrderId: "",
		Amount:  0,
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", status.Code(err))
	}
}

func TestServerProcessPaymentMapsUseCaseErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		code codes.Code
	}{
		{name: "not found", err: usecase.ErrNotFound, code: codes.NotFound},
		{name: "internal", err: errors.New("boom"), code: codes.Internal},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := NewServer(&stubPaymentService{err: tt.err})
			_, err := server.ProcessPayment(context.Background(), &paymentv1.PaymentRequest{
				OrderId: "order-1",
				Amount:  100,
			})
			if status.Code(err) != tt.code {
				t.Fatalf("expected %v, got %v", tt.code, status.Code(err))
			}
		})
	}
}

func TestServerListPaymentsReturnsPayments(t *testing.T) {
	t.Parallel()

	firstProcessedAt := time.Date(2026, time.April, 14, 8, 0, 0, 0, time.UTC)
	secondProcessedAt := time.Date(2026, time.April, 13, 8, 0, 0, 0, time.UTC)
	service := &stubPaymentService{
		payments: []domain.Payment{
			{OrderID: "order-2", Status: domain.StatusDeclined, UpdatedAt: firstProcessedAt},
			{OrderID: "order-1", Status: domain.StatusAuthorized, TransactionID: "txn-1", CreatedAt: secondProcessedAt},
		},
	}

	server := NewServer(service)
	response, err := server.ListPayments(context.Background(), &paymentv1.ListPaymentsRequest{
		MinAmount: 100,
		MaxAmount: 200,
	})
	if err != nil {
		t.Fatalf("ListPayments returned error: %v", err)
	}
	if service.listMin != 100 || service.listMax != 200 {
		t.Fatalf("ListPayments forwarded wrong range: [%d,%d]", service.listMin, service.listMax)
	}
	if len(response.GetPayments()) != 2 {
		t.Fatalf("ListPayments len = %d, want 2", len(response.GetPayments()))
	}
	if response.GetPayments()[0].GetOrderId() != "order-2" {
		t.Fatalf("unexpected first order id: %q", response.GetPayments()[0].GetOrderId())
	}
	if got := response.GetPayments()[0].GetProcessedAt().AsTime(); !got.Equal(firstProcessedAt) {
		t.Fatalf("unexpected first processed_at: %v", got)
	}
	if response.GetPayments()[1].GetTransactionId() != "txn-1" {
		t.Fatalf("unexpected second transaction id: %q", response.GetPayments()[1].GetTransactionId())
	}
	if got := response.GetPayments()[1].GetProcessedAt().AsTime(); !got.Equal(secondProcessedAt) {
		t.Fatalf("unexpected second processed_at: %v", got)
	}
}

func TestServerListPaymentsValidatesRequest(t *testing.T) {
	t.Parallel()

	server := NewServer(&stubPaymentService{})

	_, err := server.ListPayments(context.Background(), nil)
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", status.Code(err))
	}
}

func TestServerListPaymentsMapsUseCaseErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		code codes.Code
	}{
		{name: "invalid amount range", err: usecase.ErrInvalidAmountRange, code: codes.InvalidArgument},
		{name: "internal", err: errors.New("boom"), code: codes.Internal},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := NewServer(&stubPaymentService{err: tt.err})
			_, err := server.ListPayments(context.Background(), &paymentv1.ListPaymentsRequest{})
			if status.Code(err) != tt.code {
				t.Fatalf("expected %v, got %v", tt.code, status.Code(err))
			}
		})
	}
}
