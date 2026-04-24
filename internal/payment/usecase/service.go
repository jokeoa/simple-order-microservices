package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jokeoa/simple-order-microservices/internal/payment/domain"
)

var ErrNotFound = errors.New("payment not found")
var ErrInvalidAmountRange = errors.New("invalid payment amount range")

type Repository interface {
	GetByOrderID(ctx context.Context, orderID string) (domain.Payment, error)
	FindByAmountRange(ctx context.Context, min, max int64) ([]domain.Payment, error)
	Create(ctx context.Context, payment domain.Payment) (domain.Payment, error)
}

type Service struct {
	repository Repository
}

type AuthorizeInput struct {
	OrderID string
	Amount  int64
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) Authorize(ctx context.Context, input AuthorizeInput) (domain.Payment, bool, error) {
	existing, err := s.repository.GetByOrderID(ctx, input.OrderID)
	if err == nil {
		return existing, false, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return domain.Payment{}, false, fmt.Errorf("load existing payment: %w", err)
	}

	payment := domain.Payment{
		OrderID: input.OrderID,
		Amount:  input.Amount,
	}

	if input.Amount > 100000 {
		payment.Status = domain.StatusDeclined
	} else {
		payment.Status = domain.StatusAuthorized
		payment.TransactionID = uuid.NewString()
	}

	created, err := s.repository.Create(ctx, payment)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Payment{}, false, fmt.Errorf("unexpected repository state: %w", err)
		}

		reloaded, reloadErr := s.repository.GetByOrderID(ctx, input.OrderID)
		if reloadErr == nil {
			return reloaded, false, nil
		}

		return domain.Payment{}, false, fmt.Errorf("create payment: %w", err)
	}

	return created, true, nil
}

func (s *Service) GetByOrderID(ctx context.Context, orderID string) (domain.Payment, error) {
	payment, err := s.repository.GetByOrderID(ctx, orderID)
	if err != nil {
		return domain.Payment{}, err
	}

	return payment, nil
}

func (s *Service) ListPayments(ctx context.Context, min, max int64) ([]domain.Payment, error) {
	if min < 0 || max < 0 {
		return nil, ErrInvalidAmountRange
	}
	if min > 0 && max > 0 && min > max {
		return nil, ErrInvalidAmountRange
	}

	payments, err := s.repository.FindByAmountRange(ctx, min, max)
	if err != nil {
		return nil, fmt.Errorf("list payments by amount range: %w", err)
	}

	return payments, nil
}
