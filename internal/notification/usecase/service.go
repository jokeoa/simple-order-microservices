package usecase

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/jokeoa/simple-order-microservices/internal/platform/events"
)

var ErrDuplicateMessage = errors.New("duplicate message")

type ProcessedStore interface {
	MarkProcessed(ctx context.Context, messageID, orderID string) (bool, error)
}

type Service struct {
	store  ProcessedStore
	logger *log.Logger
}

func NewService(store ProcessedStore, logger *log.Logger) *Service {
	return &Service{store: store, logger: logger}
}

func (s *Service) HandlePaymentCompleted(ctx context.Context, event events.PaymentCompleted) error {
	if strings.TrimSpace(event.MessageID) == "" {
		return errors.New("message_id is required")
	}
	if strings.TrimSpace(event.OrderID) == "" {
		return errors.New("order_id is required")
	}

	inserted, err := s.store.MarkProcessed(ctx, event.MessageID, event.OrderID)
	if err != nil {
		return err
	}
	if !inserted {
		s.logger.Printf("notification skipped duplicate: message_id=%s order_id=%s", event.MessageID, event.OrderID)
		return nil
	}

	s.logger.Printf(
		"notification sent: message_id=%s order_id=%s customer_email=%s amount=%d status=%s",
		event.MessageID,
		event.OrderID,
		event.CustomerEmail,
		event.Amount,
		event.Status,
	)

	return nil
}

func FormatFailure(event events.PaymentCompleted, err error) error {
	if event.MessageID == "" {
		return fmt.Errorf("handle payment.completed: %w", err)
	}

	return fmt.Errorf("handle payment.completed message_id=%s: %w", event.MessageID, err)
}
