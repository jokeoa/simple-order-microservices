package main

import (
	"context"
	"errors"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	orderv1 "github.com/jokeoa/simple-order-microservices/internal/gen/order/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatalf("usage: %s <order-id>", os.Args[0])
	}

	target := os.Getenv("ORDER_GRPC_TARGET")
	if target == "" {
		log.Fatal("ORDER_GRPC_TARGET is required")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dialCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	connection, err := grpc.DialContext(
		dialCtx,
		target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		log.Fatalf("connect order grpc: %v", err)
	}
	defer connection.Close()

	stream, err := orderv1.NewOrderServiceClient(connection).SubscribeToOrderUpdates(ctx, &orderv1.OrderRequest{
		OrderId: os.Args[1],
	})
	if err != nil {
		log.Fatalf("subscribe to order updates: %v", err)
	}

	for {
		update, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return
		}
		if err != nil {
			log.Fatalf("receive order update: %v", err)
		}

		log.Printf(
			"order=%s status=%s transaction_id=%s timestamp=%s",
			update.GetOrderId(),
			update.GetStatus(),
			update.GetPaymentTransactionId(),
			update.GetTimestamp().AsTime().Format(time.RFC3339Nano),
		)
	}
}
