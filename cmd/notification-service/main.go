package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	notificationapp "github.com/jokeoa/simple-order-microservices/internal/notification/app"
	notificationrepo "github.com/jokeoa/simple-order-microservices/internal/notification/repository/postgres"
	notificationusecase "github.com/jokeoa/simple-order-microservices/internal/notification/usecase"
	"github.com/jokeoa/simple-order-microservices/internal/platform/messagebus"
	"github.com/jokeoa/simple-order-microservices/internal/platform/migrate"
	"github.com/jokeoa/simple-order-microservices/internal/platform/postgres"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := notificationapp.LoadConfig()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	pool, err := postgres.NewPool(ctx, cfg.PostgresDSN, cfg.PostgresMaxConns)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	defer pool.Close()

	if err := migrate.Run(ctx, pool, notificationrepo.Migrations()); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	natsConn, err := messagebus.Connect(messagebus.NATSConfig{URL: cfg.NATSURL})
	if err != nil {
		log.Fatalf("connect nats: %v", err)
	}
	defer natsConn.Close()

	js, err := messagebus.EnsurePaymentStream(ctx, natsConn)
	if err != nil {
		log.Fatalf("ensure payment stream: %v", err)
	}

	repository := notificationrepo.NewRepository(pool)
	service := notificationusecase.NewService(repository, log.Default())
	publisher := messagebus.NewPaymentCompletedPublisher(js)
	consumer := messagebus.NewPaymentCompletedConsumer(
		js,
		service,
		publisher,
		messagebus.PaymentCompletedConsumerConfig{
			MaxDeliver: cfg.MaxDeliver,
			RetryDelay: cfg.RetryDelay,
			FetchWait:  cfg.FetchWait,
		},
		log.Default(),
	)

	errCh := make(chan error, 1)
	go func() {
		log.Printf("notification-service consuming %s via durable %s", messagebus.PaymentStream, messagebus.NotificationConsumer)
		errCh <- consumer.Run(ctx)
	}()

	select {
	case err := <-errCh:
		if err != nil {
			log.Fatalf("consume payment events: %v", err)
		}
	case <-ctx.Done():
	}

	if err := natsConn.Drain(); err != nil {
		log.Printf("drain nats: %v", err)
	}

	fmt.Println("notification-service stopped")
}
