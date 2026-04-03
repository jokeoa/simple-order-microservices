package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jokeoa/simple-order-microservices/internal/order/domain"
)

var (
	ErrNotFound           = errors.New("order not found")
	ErrConflict           = errors.New("order state conflict")
	ErrPaymentUnavailable = errors.New("payment service unavailable")
)

type Repository interface {
	Create(ctx context.Context, order domain.Order) (domain.Order, error)
	GetByID(ctx context.Context, id string) (domain.Order, error)
	UpdatePaymentStatus(ctx context.Context, id string, status domain.Status, transactionID string) (domain.Order, error)
	UpdateStatus(ctx context.Context, id string, status domain.Status) (domain.Order, error)
}

type PaymentAuthorizer interface {
	Authorize(ctx context.Context, input PaymentInput) (PaymentResult, error)
}

type PaymentInput struct {
	OrderID string
	Amount  int64
}

type PaymentResult struct {
	Status        string
	TransactionID string
}

type Service struct {
	repository Repository
	payments   PaymentAuthorizer
}

type CreateOrderInput struct {
	CustomerID string
	ItemName   string
	Amount     int64
}

func NewService(repository Repository, payments PaymentAuthorizer) *Service {
	return &Service{repository: repository, payments: payments}
}

func (s *Service) Create(ctx context.Context, input CreateOrderInput) (domain.Order, error) {
	order := domain.Order{
		ID:         uuid.NewString(),
		CustomerID: input.CustomerID,
		ItemName:   input.ItemName,
		Amount:     input.Amount,
		Status:     domain.StatusPending,
	}

	created, err := s.repository.Create(ctx, order)
	if err != nil {
		return domain.Order{}, fmt.Errorf("create order: %w", err)
	}

	payment, err := s.payments.Authorize(ctx, PaymentInput{OrderID: created.ID, Amount: created.Amount})
	if err != nil {
		if errors.Is(err, ErrPaymentUnavailable) {
			return created, ErrPaymentUnavailable
		}

		return domain.Order{}, fmt.Errorf("authorize payment: %w", err)
	}

	switch payment.Status {
	case "Authorized":
		updated, err := s.repository.UpdatePaymentStatus(ctx, created.ID, domain.StatusPaid, payment.TransactionID)
		if err != nil {
			return domain.Order{}, fmt.Errorf("mark order paid: %w", err)
		}
		return updated, nil
	case "Declined":
		updated, err := s.repository.UpdatePaymentStatus(ctx, created.ID, domain.StatusFailed, "")
		if err != nil {
			return domain.Order{}, fmt.Errorf("mark order failed: %w", err)
		}
		return updated, nil
	default:
		return domain.Order{}, fmt.Errorf("unexpected payment status: %s", payment.Status)
	}
}

func (s *Service) GetByID(ctx context.Context, id string) (domain.Order, error) {
	return s.repository.GetByID(ctx, id)
}

func (s *Service) Cancel(ctx context.Context, id string) (domain.Order, error) {
	order, err := s.repository.GetByID(ctx, id)
	if err != nil {
		return domain.Order{}, err
	}

	if order.Status != domain.StatusPending {
		return domain.Order{}, ErrConflict
	}

	updated, err := s.repository.UpdateStatus(ctx, id, domain.StatusCancelled)
	if err != nil {
		return domain.Order{}, fmt.Errorf("cancel order: %w", err)
	}

	return updated, nil
}
