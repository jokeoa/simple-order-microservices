package usecase

import (
	"bytes"
	"context"
	"errors"
	"log"
	"testing"

	"github.com/jokeoa/simple-order-microservices/internal/platform/events"
)

type processedStoreStub struct {
	inserted bool
	err      error
	calls    int
}

func (s *processedStoreStub) MarkProcessed(context.Context, string, string) (bool, error) {
	s.calls++
	return s.inserted, s.err
}

func TestHandlePaymentCompletedLogsNotification(t *testing.T) {
	var output bytes.Buffer
	store := &processedStoreStub{inserted: true}
	service := NewService(store, log.New(&output, "", 0))

	err := service.HandlePaymentCompleted(context.Background(), events.PaymentCompleted{
		MessageID:     "payment.completed:order-1",
		OrderID:       "order-1",
		Amount:        500,
		CustomerEmail: "customer@example.com",
		Status:        "Authorized",
	})
	if err != nil {
		t.Fatalf("HandlePaymentCompleted() error = %v", err)
	}
	if store.calls != 1 {
		t.Fatalf("MarkProcessed() calls = %d, want 1", store.calls)
	}
	if got := output.String(); got == "" {
		t.Fatalf("expected notification log")
	}
}

func TestHandlePaymentCompletedSkipsDuplicate(t *testing.T) {
	var output bytes.Buffer
	store := &processedStoreStub{inserted: false}
	service := NewService(store, log.New(&output, "", 0))

	err := service.HandlePaymentCompleted(context.Background(), events.PaymentCompleted{
		MessageID: "payment.completed:order-1",
		OrderID:   "order-1",
	})
	if err != nil {
		t.Fatalf("HandlePaymentCompleted() error = %v", err)
	}
	if got := output.String(); got == "" {
		t.Fatalf("expected duplicate log")
	}
}

func TestHandlePaymentCompletedValidatesMessage(t *testing.T) {
	service := NewService(&processedStoreStub{}, log.New(&bytes.Buffer{}, "", 0))

	err := service.HandlePaymentCompleted(context.Background(), events.PaymentCompleted{})
	if err == nil {
		t.Fatalf("HandlePaymentCompleted() error = nil, want validation error")
	}
}

func TestHandlePaymentCompletedReturnsStoreError(t *testing.T) {
	wantErr := errors.New("db unavailable")
	service := NewService(&processedStoreStub{err: wantErr}, log.New(&bytes.Buffer{}, "", 0))

	err := service.HandlePaymentCompleted(context.Background(), events.PaymentCompleted{
		MessageID: "payment.completed:order-1",
		OrderID:   "order-1",
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("HandlePaymentCompleted() error = %v, want %v", err, wantErr)
	}
}
