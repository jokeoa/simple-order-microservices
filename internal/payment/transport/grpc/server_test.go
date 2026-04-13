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
	payment domain.Payment
	created bool
	err     error
	input   usecase.AuthorizeInput
}

func (s *stubPaymentService) Authorize(_ context.Context, input usecase.AuthorizeInput) (domain.Payment, bool, error) {
	s.input = input
	return s.payment, s.created, s.err
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
