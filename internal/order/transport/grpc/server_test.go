package grpc

import (
	"context"
	"testing"
	"time"

	orderv1 "github.com/jokeoa/simple-order-microservices/internal/gen/order/v1"
	"github.com/jokeoa/simple-order-microservices/internal/order/domain"
	"github.com/jokeoa/simple-order-microservices/internal/order/stream"
	"github.com/jokeoa/simple-order-microservices/internal/order/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type stubOrderReader struct {
	order domain.Order
	err   error
}

func (r *stubOrderReader) GetByID(_ context.Context, _ string) (domain.Order, error) {
	return r.order, r.err
}

type stubSubscriber struct {
	updates chan stream.Update
}

func (s *stubSubscriber) Subscribe(_ string) (<-chan stream.Update, func()) {
	return s.updates, func() {}
}

type recordingStream struct {
	ctx  context.Context
	sent []*orderv1.OrderStatusUpdate
}

func (s *recordingStream) SetHeader(metadata.MD) error  { return nil }
func (s *recordingStream) SendHeader(metadata.MD) error { return nil }
func (s *recordingStream) SetTrailer(metadata.MD)       {}
func (s *recordingStream) Context() context.Context     { return s.ctx }
func (s *recordingStream) SendMsg(any) error            { return nil }
func (s *recordingStream) RecvMsg(any) error            { return nil }
func (s *recordingStream) Send(message *orderv1.OrderStatusUpdate) error {
	s.sent = append(s.sent, message)
	return nil
}

func TestServerSubscribeToOrderUpdatesStreamsInitialAndLiveUpdates(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.April, 13, 12, 0, 0, 0, time.UTC)
	updates := make(chan stream.Update)
	go func() {
		updates <- stream.Update{
			OrderID:              "order-1",
			Status:               domain.StatusPaid,
			PaymentTransactionID: "txn-1",
			Timestamp:            now.Add(time.Second),
		}
		close(updates)
	}()

	server := NewServer(
		&stubOrderReader{
			order: domain.Order{
				ID:        "order-1",
				Status:    domain.StatusPending,
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
		&stubSubscriber{updates: updates},
		time.Minute,
	)

	stream := &recordingStream{ctx: context.Background()}
	err := server.SubscribeToOrderUpdates(&orderv1.OrderRequest{OrderId: "order-1"}, stream)
	if err != nil {
		t.Fatalf("SubscribeToOrderUpdates returned error: %v", err)
	}

	if len(stream.sent) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(stream.sent))
	}
	if got := stream.sent[0].GetStatus(); got != string(domain.StatusPending) {
		t.Fatalf("unexpected initial status: %q", got)
	}
	if got := stream.sent[1].GetStatus(); got != string(domain.StatusPaid) {
		t.Fatalf("unexpected update status: %q", got)
	}
}

func TestServerSubscribeToOrderUpdatesValidatesRequest(t *testing.T) {
	t.Parallel()

	server := NewServer(&stubOrderReader{}, &stubSubscriber{updates: make(chan stream.Update)}, time.Minute)
	stream := &recordingStream{ctx: context.Background()}

	err := server.SubscribeToOrderUpdates(&orderv1.OrderRequest{}, stream)
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", status.Code(err))
	}
}

func TestServerSubscribeToOrderUpdatesMapsNotFound(t *testing.T) {
	t.Parallel()

	server := NewServer(&stubOrderReader{err: usecase.ErrNotFound}, &stubSubscriber{updates: make(chan stream.Update)}, time.Minute)
	stream := &recordingStream{ctx: context.Background()}

	err := server.SubscribeToOrderUpdates(&orderv1.OrderRequest{OrderId: "missing"}, stream)
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound, got %v", status.Code(err))
	}
}

func TestServerSubscribeToOrderUpdatesPrefersBufferedUpdateOverStaleSnapshot(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.April, 13, 12, 0, 0, 0, time.UTC)
	updates := make(chan stream.Update, 1)
	updates <- stream.Update{
		OrderID:              "order-1",
		Status:               domain.StatusPaid,
		PaymentTransactionID: "txn-1",
		Timestamp:            now.Add(time.Second),
	}
	close(updates)

	server := NewServer(
		&stubOrderReader{
			order: domain.Order{
				ID:        "order-1",
				Status:    domain.StatusPending,
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
		&stubSubscriber{updates: updates},
		time.Minute,
	)

	stream := &recordingStream{ctx: context.Background()}
	err := server.SubscribeToOrderUpdates(&orderv1.OrderRequest{OrderId: "order-1"}, stream)
	if err != nil {
		t.Fatalf("SubscribeToOrderUpdates returned error: %v", err)
	}

	if len(stream.sent) == 0 {
		t.Fatal("expected at least one message")
	}
	if got := stream.sent[0].GetStatus(); got != string(domain.StatusPaid) {
		t.Fatalf("expected buffered update to win, got %q", got)
	}
}
