package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	orderv1 "github.com/jokeoa/simple-order-microservices/internal/gen/order/v1"
	paymentv1 "github.com/jokeoa/simple-order-microservices/internal/gen/payment/v1"
	orderapp "github.com/jokeoa/simple-order-microservices/internal/order/app"
	ordergrpcclient "github.com/jokeoa/simple-order-microservices/internal/order/paymentclient/grpc"
	orderrepo "github.com/jokeoa/simple-order-microservices/internal/order/repository/postgres"
	orderstream "github.com/jokeoa/simple-order-microservices/internal/order/stream"
	ordergrpc "github.com/jokeoa/simple-order-microservices/internal/order/transport/grpc"
	ordertransport "github.com/jokeoa/simple-order-microservices/internal/order/transport/http"
	orderusecase "github.com/jokeoa/simple-order-microservices/internal/order/usecase"
	"github.com/jokeoa/simple-order-microservices/internal/platform/grpcx"
	"github.com/jokeoa/simple-order-microservices/internal/platform/metrics"
	"github.com/jokeoa/simple-order-microservices/internal/platform/migrate"
	"github.com/jokeoa/simple-order-microservices/internal/platform/postgres"
	"github.com/jokeoa/simple-order-microservices/internal/platform/validate"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := orderapp.LoadConfig()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	pool, err := postgres.NewPool(ctx, cfg.PostgresDSN, cfg.PostgresMaxConns)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	defer pool.Close()

	if err := migrate.Run(ctx, pool, orderrepo.Migrations()); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	validator := validate.New()
	repository := orderrepo.NewRepository(pool)
	registry := metrics.NewRegistry()
	httpMetrics := metrics.NewHTTPServerMetrics("order-service")
	paymentClientMetrics := metrics.NewPaymentClientMetrics("order-service", "payment-service")
	registry.MustRegister(httpMetrics.Collectors()...)
	registry.MustRegister(paymentClientMetrics.Collectors()...)
	registry.MustRegister(
		metrics.NewGoRuntimeCollector("order-service"),
		metrics.NewPostgresPoolConnectionsCollector("order-service", pool),
		metrics.NewPostgresPoolUtilizationCollector("order-service", pool),
	)

	paymentConnection, err := grpcx.DialPayment(ctx, cfg.PaymentGRPCTarget)
	if err != nil {
		log.Fatalf("connect payment grpc: %v", err)
	}
	defer paymentConnection.Close()

	paymentClient := ordergrpcclient.New(paymentv1.NewPaymentServiceClient(paymentConnection), cfg.PaymentTimeout, paymentClientMetrics)
	service := orderusecase.NewService(repository, paymentClient)
	handler := ordertransport.NewHandler(service, validator)
	broker := orderstream.NewBroker()
	defer broker.Close()

	orderUpdatesListener := orderstream.NewPostgresListener(cfg.PostgresDSN, broker, log.Default())
	if err := orderUpdatesListener.Start(ctx); err != nil {
		log.Fatalf("start order updates listener: %v", err)
	}

	grpcHandler := ordergrpc.NewServer(service, broker, cfg.StreamTimeout)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle("GET /metrics", registry.Handler())
	handler.Register(mux)

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           httpMetrics.Middleware(mux),
		ReadHeaderTimeout: 2 * time.Second,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
	}

	grpcListener, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		log.Fatalf("listen order grpc: %v", err)
	}
	defer grpcListener.Close()

	grpcServer := grpcx.NewServer(log.Default())
	orderv1.RegisterOrderServiceServer(grpcServer, grpcHandler)

	serverErrors := make(chan error, 2)

	go func() {
		log.Printf("order-service http listening on %s", cfg.HTTPAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrors <- fmt.Errorf("serve order http: %w", err)
		}
	}()

	go func() {
		log.Printf("order-service grpc listening on %s", cfg.GRPCAddr)
		if err := grpcServer.Serve(grpcListener); err != nil {
			serverErrors <- fmt.Errorf("serve order grpc: %w", err)
		}
	}()

	select {
	case err := <-serverErrors:
		log.Fatalf("%v", err)
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	broker.Close()
	_ = server.Shutdown(shutdownCtx)

	grpcStopped := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(grpcStopped)
	}()

	select {
	case <-grpcStopped:
	case <-shutdownCtx.Done():
		grpcServer.Stop()
	}

	fmt.Println("order-service stopped")
}
