package grpc

import (
	"context"
	"errors"
	"fmt"
	"time"

	paymentv1 "github.com/jokeoa/simple-order-microservices/internal/gen/payment/v1"
	"github.com/jokeoa/simple-order-microservices/internal/order/usecase"
	"github.com/jokeoa/simple-order-microservices/internal/platform/metrics"
	grpcpkg "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type paymentServiceClient interface {
	ProcessPayment(ctx context.Context, request *paymentv1.PaymentRequest, opts ...grpcpkg.CallOption) (*paymentv1.PaymentResponse, error)
}

type Client struct {
	client  paymentServiceClient
	timeout time.Duration
	metrics *metrics.PaymentClientMetrics
}

func New(client paymentServiceClient, timeout time.Duration, clientMetrics *metrics.PaymentClientMetrics) *Client {
	return &Client{
		client:  client,
		timeout: timeout,
		metrics: clientMetrics,
	}
}

func (c *Client) Authorize(ctx context.Context, input usecase.PaymentInput) (usecase.PaymentResult, error) {
	if input.OrderID == "" {
		return usecase.PaymentResult{}, fmt.Errorf("process payment: order id is required")
	}
	if input.Amount <= 0 {
		return usecase.PaymentResult{}, fmt.Errorf("process payment: amount must be greater than zero")
	}

	callCtx := ctx
	cancel := func() {}
	if c.timeout > 0 {
		callCtx, cancel = context.WithTimeout(ctx, c.timeout)
	}
	defer cancel()

	startedAt := time.Now()
	response, err := c.client.ProcessPayment(callCtx, &paymentv1.PaymentRequest{
		OrderId: input.OrderID,
		Amount:  input.Amount,
	})
	if err != nil {
		grpcCode := status.Code(err)
		c.observe(int(grpcCode), classifyOutcome(grpcCode), time.Since(startedAt))
		if isUnavailable(callCtx, err) {
			return usecase.PaymentResult{}, usecase.ErrPaymentUnavailable
		}

		return usecase.PaymentResult{}, fmt.Errorf("process payment: %w", err)
	}

	c.observe(int(codes.OK), "success", time.Since(startedAt))

	switch response.GetStatus() {
	case "Authorized", "Declined":
		return usecase.PaymentResult{
			Status:        response.GetStatus(),
			TransactionID: response.GetTransactionId(),
		}, nil
	default:
		return usecase.PaymentResult{}, fmt.Errorf("process payment: unexpected payment status %q", response.GetStatus())
	}
}

func (c *Client) observe(statusCode int, outcome string, duration time.Duration) {
	if c.metrics == nil {
		return
	}

	c.metrics.Observe("POST", paymentv1.PaymentService_ProcessPayment_FullMethodName, statusCode, outcome, duration)
}

func isUnavailable(ctx context.Context, err error) bool {
	if errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return true
	}

	switch status.Code(err) {
	case codes.Canceled, codes.DeadlineExceeded, codes.Unavailable:
		return true
	default:
		return false
	}
}

func classifyOutcome(code codes.Code) string {
	switch code {
	case codes.Canceled:
		return "canceled"
	case codes.DeadlineExceeded:
		return "timeout"
	case codes.Unavailable:
		return "transport_error"
	case codes.OK:
		return "success"
	default:
		return "server_error"
	}
}
