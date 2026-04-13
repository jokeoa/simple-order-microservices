package grpc

import (
	"context"
	"errors"
	"strings"
	"time"

	orderv1 "github.com/jokeoa/simple-order-microservices/internal/gen/order/v1"
	"github.com/jokeoa/simple-order-microservices/internal/order/domain"
	"github.com/jokeoa/simple-order-microservices/internal/order/stream"
	"github.com/jokeoa/simple-order-microservices/internal/order/usecase"
	grpcpkg "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type orderReader interface {
	GetByID(ctx context.Context, id string) (domain.Order, error)
}

type updateSubscriber interface {
	Subscribe(orderID string) (<-chan stream.Update, func())
}

type Server struct {
	orderv1.UnimplementedOrderServiceServer
	reader      orderReader
	subscribers updateSubscriber
	timeout     time.Duration
}

func NewServer(reader orderReader, subscribers updateSubscriber, timeout time.Duration) *Server {
	return &Server{
		reader:      reader,
		subscribers: subscribers,
		timeout:     timeout,
	}
}

func (s *Server) SubscribeToOrderUpdates(request *orderv1.OrderRequest, server grpcpkg.ServerStreamingServer[orderv1.OrderStatusUpdate]) error {
	if request == nil {
		return status.Error(codes.InvalidArgument, "request is required")
	}
	if strings.TrimSpace(request.GetOrderId()) == "" {
		return status.Error(codes.InvalidArgument, "order_id is required")
	}

	streamContext := server.Context()
	if s.timeout > 0 {
		var cancel context.CancelFunc
		streamContext, cancel = context.WithTimeout(streamContext, s.timeout)
		defer cancel()
	}

	updates, cancel := s.subscribers.Subscribe(request.GetOrderId())
	defer cancel()

	order, err := s.reader.GetByID(streamContext, request.GetOrderId())
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrNotFound):
			return status.Error(codes.NotFound, "order not found")
		default:
			return status.Error(codes.Internal, "failed to load order")
		}
	}

	initial := mapOrder(order)
	if latest, ok := drainLatest(updates); ok && isNewerUpdate(initial, latest) {
		initial = mapUpdate(latest)
	}

	if err := server.Send(initial); err != nil {
		if errors.Is(err, context.Canceled) {
			return nil
		}
		return err
	}

	for {
		select {
		case <-streamContext.Done():
			return nil
		case update, ok := <-updates:
			if !ok {
				return nil
			}
			if err := server.Send(mapUpdate(update)); err != nil {
				if errors.Is(err, context.Canceled) {
					return nil
				}
				return err
			}
		}
	}
}

func drainLatest(updates <-chan stream.Update) (stream.Update, bool) {
	var (
		latest stream.Update
		ok     bool
	)

	for {
		select {
		case update, open := <-updates:
			if !open {
				return latest, ok
			}
			latest = update
			ok = true
		default:
			return latest, ok
		}
	}
}

func isNewerUpdate(initial *orderv1.OrderStatusUpdate, update stream.Update) bool {
	initialTimestamp := initial.GetTimestamp().AsTime()
	if update.Timestamp.After(initialTimestamp) {
		return true
	}

	if update.Timestamp.Equal(initialTimestamp) {
		return update.Status != domain.Status(initial.GetStatus()) || update.PaymentTransactionID != initial.GetPaymentTransactionId()
	}

	return false
}

func mapOrder(order domain.Order) *orderv1.OrderStatusUpdate {
	timestamp := order.UpdatedAt
	if timestamp.IsZero() {
		timestamp = order.CreatedAt
	}
	if timestamp.IsZero() {
		timestamp = time.Now().UTC()
	}

	return &orderv1.OrderStatusUpdate{
		OrderId:              order.ID,
		Status:               string(order.Status),
		Timestamp:            timestamppb.New(timestamp),
		PaymentTransactionId: order.PaymentTransactionID,
	}
}

func mapUpdate(update stream.Update) *orderv1.OrderStatusUpdate {
	timestamp := update.Timestamp
	if timestamp.IsZero() {
		timestamp = time.Now().UTC()
	}

	return &orderv1.OrderStatusUpdate{
		OrderId:              update.OrderID,
		Status:               string(update.Status),
		Timestamp:            timestamppb.New(timestamp),
		PaymentTransactionId: update.PaymentTransactionID,
	}
}
