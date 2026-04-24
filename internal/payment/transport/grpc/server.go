package grpc

import (
	"context"
	"errors"
	"strings"
	"time"

	paymentv1 "github.com/jokeoa/simple-order-microservices/internal/gen/payment/v1"
	"github.com/jokeoa/simple-order-microservices/internal/payment/domain"
	"github.com/jokeoa/simple-order-microservices/internal/payment/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type paymentService interface {
	Authorize(ctx context.Context, input usecase.AuthorizeInput) (domain.Payment, bool, error)
	ListPayments(ctx context.Context, min, max int64) ([]domain.Payment, error)
}

type Server struct {
	paymentv1.UnimplementedPaymentServiceServer
	service paymentService
}

func NewServer(service paymentService) *Server {
	return &Server{service: service}
}

func (s *Server) ProcessPayment(ctx context.Context, request *paymentv1.PaymentRequest) (*paymentv1.PaymentResponse, error) {
	if request == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if strings.TrimSpace(request.GetOrderId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "order_id is required")
	}
	if request.GetAmount() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "amount must be greater than zero")
	}

	payment, _, err := s.service.Authorize(ctx, usecase.AuthorizeInput{
		OrderID: request.GetOrderId(),
		Amount:  request.GetAmount(),
	})
	if err != nil {
		return nil, mapError(err)
	}

	return mapPayment(payment), nil
}

func (s *Server) ListPayments(ctx context.Context, request *paymentv1.ListPaymentsRequest) (*paymentv1.ListPaymentsResponse, error) {
	if request == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	payments, err := s.service.ListPayments(ctx, request.GetMinAmount(), request.GetMaxAmount())
	if err != nil {
		return nil, mapError(err)
	}

	response := &paymentv1.ListPaymentsResponse{
		Payments: make([]*paymentv1.PaymentResponse, 0, len(payments)),
	}
	for _, payment := range payments {
		response.Payments = append(response.Payments, mapPayment(payment))
	}

	return response, nil
}

func mapPayment(payment domain.Payment) *paymentv1.PaymentResponse {
	processedAt := payment.UpdatedAt
	if processedAt.IsZero() {
		processedAt = payment.CreatedAt
	}
	if processedAt.IsZero() {
		processedAt = time.Now().UTC()
	}

	return &paymentv1.PaymentResponse{
		OrderId:       payment.OrderID,
		Status:        string(payment.Status),
		TransactionId: payment.TransactionID,
		ProcessedAt:   timestamppb.New(processedAt),
	}
}

func mapError(err error) error {
	switch {
	case errors.Is(err, usecase.ErrInvalidAmountRange):
		return status.Error(codes.InvalidArgument, "invalid amount range")
	case errors.Is(err, usecase.ErrNotFound):
		return status.Error(codes.NotFound, "payment not found")
	default:
		return status.Error(codes.Internal, "failed to process payment")
	}
}
