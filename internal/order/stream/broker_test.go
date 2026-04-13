package stream

import (
	"testing"
	"time"

	"github.com/jokeoa/simple-order-microservices/internal/order/domain"
)

func TestBrokerPublishesUpdatesToAllSubscribers(t *testing.T) {
	t.Parallel()

	broker := NewBroker()
	defer broker.Close()

	first, cancelFirst := broker.Subscribe("order-1")
	defer cancelFirst()

	second, cancelSecond := broker.Subscribe("order-1")
	defer cancelSecond()

	update := Update{
		OrderID:              "order-1",
		Status:               domain.StatusPaid,
		PaymentTransactionID: "txn-1",
		Timestamp:            time.Now().UTC(),
	}
	broker.Publish(update)

	assertUpdate(t, first, update)
	assertUpdate(t, second, update)
}

func TestBrokerStopsPublishingAfterUnsubscribe(t *testing.T) {
	t.Parallel()

	broker := NewBroker()
	defer broker.Close()

	updates, cancel := broker.Subscribe("order-1")
	cancel()

	broker.Publish(Update{
		OrderID:   "order-1",
		Status:    domain.StatusFailed,
		Timestamp: time.Now().UTC(),
	})

	select {
	case update, ok := <-updates:
		if ok {
			t.Fatalf("expected closed channel, received %+v", update)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timed out waiting for subscription shutdown")
	}
}

func assertUpdate(t *testing.T, updates <-chan Update, want Update) {
	t.Helper()

	select {
	case got := <-updates:
		if got.OrderID != want.OrderID {
			t.Fatalf("unexpected order id: got %q want %q", got.OrderID, want.OrderID)
		}
		if got.Status != want.Status {
			t.Fatalf("unexpected status: got %q want %q", got.Status, want.Status)
		}
		if got.PaymentTransactionID != want.PaymentTransactionID {
			t.Fatalf("unexpected transaction id: got %q want %q", got.PaymentTransactionID, want.PaymentTransactionID)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for update")
	}
}
