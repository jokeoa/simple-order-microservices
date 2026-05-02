package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jokeoa/simple-order-microservices/internal/payment/domain"
	"github.com/jokeoa/simple-order-microservices/internal/platform/events"
)

var ErrNotFound = errors.New("payment not found")
var ErrInvalidAmountRange = errors.New("invalid payment amount range")
var ErrEventPublishFailed = errors.New("payment event publish failed")

type Repository interface {
	GetByOrderID(ctx context.Context, orderID string) (domain.Payment, error)
	FindByAmountRange(ctx context.Context, min, max int64) ([]domain.Payment, error)
	Create(ctx context.Context, payment domain.Payment) (domain.Payment, error)
}

type PaymentCompletedPublisher interface {
	PublishPaymentCompleted(ctx context.Context, event events.PaymentCompleted) error
}

type Service struct {
	repository Repository
	publisher  PaymentCompletedPublisher
}

type AuthorizeInput struct {
	OrderID       string
	Amount        int64
	CustomerEmail string
}

func NewService(repository Repository, publishers ...PaymentCompletedPublisher) *Service {
	var publisher PaymentCompletedPublisher
	if len(publishers) > 0 {
		publisher = publishers[0]
	}

	return &Service{repository: repository, publisher: publisher}
}

func (s *Service) Authorize(ctx context.Context, input AuthorizeInput) (domain.Payment, bool, error) {
	existing, err := s.repository.GetByOrderID(ctx, input.OrderID)
	if err == nil {
		if err := s.publishPaymentCompleted(ctx, existing, input.CustomerEmail); err != nil {
			return domain.Payment{}, false, err
		}
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
			if err := s.publishPaymentCompleted(ctx, reloaded, input.CustomerEmail); err != nil {
				return domain.Payment{}, false, err
			}
			return reloaded, false, nil
		}

		return domain.Payment{}, false, fmt.Errorf("create payment: %w", err)
	}

	if err := s.publishPaymentCompleted(ctx, created, input.CustomerEmail); err != nil {
		return domain.Payment{}, true, err
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

func (s *Service) publishPaymentCompleted(ctx context.Context, payment domain.Payment, customerEmail string) error {
	if s.publisher == nil {
		return nil
	}

	occurredAt := payment.UpdatedAt
	if occurredAt.IsZero() {
		occurredAt = payment.CreatedAt
	}
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}

	event := events.PaymentCompleted{
		MessageID:     "payment.completed:" + payment.OrderID,
		OrderID:       payment.OrderID,
		Amount:        payment.Amount,
		CustomerEmail: customerEmail,
		Status:        string(payment.Status),
		OccurredAt:    occurredAt,
	}

	if err := s.publisher.PublishPaymentCompleted(ctx, event); err != nil {
		return fmt.Errorf("%w: %v", ErrEventPublishFailed, err)
	}

	return nil
}
