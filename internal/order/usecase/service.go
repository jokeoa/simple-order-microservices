package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"

	"github.com/google/uuid"
	"github.com/jokeoa/simple-order-microservices/internal/order/domain"
)

var (
	ErrNotFound                = errors.New("order not found")
	ErrConflict                = errors.New("order state conflict")
	ErrPaymentUnavailable      = errors.New("payment service unavailable")
	ErrIdempotencyConflict     = errors.New("idempotency key conflict")
	ErrDuplicateIdempotencyKey = errors.New("duplicate idempotency key")
)

type Repository interface {
	Create(ctx context.Context, order domain.Order) (domain.Order, error)
	GetByID(ctx context.Context, id string) (domain.Order, error)
	GetByIdempotencyKey(ctx context.Context, key string) (domain.Order, error)
	GetRevenueByCustomerID(ctx context.Context, customerID string) (domain.Revenue, error)
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
	IdempotencyKey string
	CustomerID     string
	ItemName       string
	Amount         int64
}

func NewService(repository Repository, payments PaymentAuthorizer) *Service {
	return &Service{repository: repository, payments: payments}
}

func (s *Service) Create(ctx context.Context, input CreateOrderInput) (domain.Order, bool, error) {
	fingerprint := requestFingerprint(input)

	existing, err := s.repository.GetByIdempotencyKey(ctx, input.IdempotencyKey)
	if err == nil {
		if existing.RequestFingerprint != fingerprint {
			return domain.Order{}, false, ErrIdempotencyConflict
		}

		return s.resumeExistingOrder(ctx, existing)
	}
	if !errors.Is(err, ErrNotFound) {
		return domain.Order{}, false, fmt.Errorf("load order by idempotency key: %w", err)
	}

	order := domain.Order{
		ID:                 uuid.NewString(),
		CustomerID:         input.CustomerID,
		ItemName:           input.ItemName,
		Amount:             input.Amount,
		Status:             domain.StatusPending,
		IdempotencyKey:     input.IdempotencyKey,
		RequestFingerprint: fingerprint,
	}

	created, err := s.repository.Create(ctx, order)
	if err != nil {
		if errors.Is(err, ErrDuplicateIdempotencyKey) {
			reloaded, reloadErr := s.repository.GetByIdempotencyKey(ctx, input.IdempotencyKey)
			if reloadErr != nil {
				return domain.Order{}, false, fmt.Errorf("reload duplicate idempotent order: %w", reloadErr)
			}
			if reloaded.RequestFingerprint != fingerprint {
				return domain.Order{}, false, ErrIdempotencyConflict
			}

			return s.resumeExistingOrder(ctx, reloaded)
		}

		return domain.Order{}, false, fmt.Errorf("create order: %w", err)
	}

	resolved, err := s.reconcilePayment(ctx, created)
	if err != nil {
		if errors.Is(err, ErrPaymentUnavailable) {
			return created, true, ErrPaymentUnavailable
		}

		return domain.Order{}, true, err
	}

	return resolved, true, nil
}

func (s *Service) GetByID(ctx context.Context, id string) (domain.Order, error) {
	return s.repository.GetByID(ctx, id)
}

func (s *Service) GetRevenueByCustomerID(ctx context.Context, customerID string) (domain.Revenue, error) {
	revenue, err := s.repository.GetRevenueByCustomerID(ctx, customerID)
	if err != nil {
		return domain.Revenue{}, fmt.Errorf("get customer revenue: %w", err)
	}

	return revenue, nil
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

func (s *Service) resumeExistingOrder(ctx context.Context, order domain.Order) (domain.Order, bool, error) {
	if order.Status != domain.StatusPending {
		return order, false, nil
	}

	resolved, err := s.reconcilePayment(ctx, order)
	if err != nil {
		if errors.Is(err, ErrPaymentUnavailable) {
			return order, false, ErrPaymentUnavailable
		}

		return domain.Order{}, false, err
	}

	return resolved, false, nil
}

func (s *Service) reconcilePayment(ctx context.Context, order domain.Order) (domain.Order, error) {
	payment, err := s.payments.Authorize(ctx, PaymentInput{OrderID: order.ID, Amount: order.Amount})
	if err != nil {
		if errors.Is(err, ErrPaymentUnavailable) {
			return order, ErrPaymentUnavailable
		}

		return domain.Order{}, fmt.Errorf("authorize payment: %w", err)
	}

	switch payment.Status {
	case "Authorized":
		updated, err := s.repository.UpdatePaymentStatus(ctx, order.ID, domain.StatusPaid, payment.TransactionID)
		if err != nil {
			return domain.Order{}, fmt.Errorf("mark order paid: %w", err)
		}
		return updated, nil
	case "Declined":
		updated, err := s.repository.UpdatePaymentStatus(ctx, order.ID, domain.StatusFailed, "")
		if err != nil {
			return domain.Order{}, fmt.Errorf("mark order failed: %w", err)
		}
		return updated, nil
	default:
		return domain.Order{}, fmt.Errorf("unexpected payment status: %s", payment.Status)
	}
}

func requestFingerprint(input CreateOrderInput) string {
	sum := sha256.Sum256([]byte(input.CustomerID + "\x00" + input.ItemName + "\x00" + strconv.FormatInt(input.Amount, 10)))
	return hex.EncodeToString(sum[:])
}
