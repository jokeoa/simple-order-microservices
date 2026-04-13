package grpc

import (
	"context"
	"testing"
	"time"

	paymentv1 "github.com/jokeoa/simple-order-microservices/internal/gen/payment/v1"
	"github.com/jokeoa/simple-order-microservices/internal/order/usecase"
	grpcpkg "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type stubPaymentClient struct {
	response *paymentv1.PaymentResponse
	err      error
	request  *paymentv1.PaymentRequest
}

func (c *stubPaymentClient) ProcessPayment(_ context.Context, request *paymentv1.PaymentRequest, _ ...grpcpkg.CallOption) (*paymentv1.PaymentResponse, error) {
	c.request = request
	return c.response, c.err
}

func TestClientAuthorizeReturnsPaymentResult(t *testing.T) {
	t.Parallel()

	client := New(&stubPaymentClient{
		response: &paymentv1.PaymentResponse{
			OrderId:       "order-1",
			Status:        "Authorized",
			TransactionId: "txn-1",
			ProcessedAt:   timestamppb.New(time.Now()),
		},
	}, time.Second, nil)

	result, err := client.Authorize(context.Background(), usecase.PaymentInput{
		OrderID: "order-1",
		Amount:  500,
	})
	if err != nil {
		t.Fatalf("Authorize returned error: %v", err)
	}

	if result.Status != "Authorized" || result.TransactionID != "txn-1" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestClientAuthorizeMapsUnavailableErrors(t *testing.T) {
	t.Parallel()

	client := New(&stubPaymentClient{
		err: status.Error(codes.Unavailable, "payment service unavailable"),
	}, time.Second, nil)

	_, err := client.Authorize(context.Background(), usecase.PaymentInput{
		OrderID: "order-1",
		Amount:  500,
	})
	if err != usecase.ErrPaymentUnavailable {
		t.Fatalf("expected ErrPaymentUnavailable, got %v", err)
	}
}

func TestClientAuthorizeRejectsUnexpectedStatus(t *testing.T) {
	t.Parallel()

	client := New(&stubPaymentClient{
		response: &paymentv1.PaymentResponse{
			OrderId:     "order-1",
			Status:      "Unknown",
			ProcessedAt: timestamppb.New(time.Now()),
		},
	}, time.Second, nil)

	_, err := client.Authorize(context.Background(), usecase.PaymentInput{
		OrderID: "order-1",
		Amount:  500,
	})
	if err == nil {
		t.Fatal("expected error for unexpected payment status")
	}
}
