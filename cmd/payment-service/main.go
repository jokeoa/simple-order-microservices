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

	paymentv1 "github.com/jokeoa/simple-order-microservices/internal/gen/payment/v1"
	paymentapp "github.com/jokeoa/simple-order-microservices/internal/payment/app"
	paymentrepo "github.com/jokeoa/simple-order-microservices/internal/payment/repository/postgres"
	paymentgrpc "github.com/jokeoa/simple-order-microservices/internal/payment/transport/grpc"
	paymenthttp "github.com/jokeoa/simple-order-microservices/internal/payment/transport/http"
	paymentusecase "github.com/jokeoa/simple-order-microservices/internal/payment/usecase"
	"github.com/jokeoa/simple-order-microservices/internal/platform/grpcx"
	"github.com/jokeoa/simple-order-microservices/internal/platform/metrics"
	"github.com/jokeoa/simple-order-microservices/internal/platform/migrate"
	"github.com/jokeoa/simple-order-microservices/internal/platform/postgres"
	"github.com/jokeoa/simple-order-microservices/internal/platform/validate"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := paymentapp.LoadConfig()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	pool, err := postgres.NewPool(ctx, cfg.PostgresDSN, cfg.PostgresMaxConns)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	defer pool.Close()

	if err := migrate.Run(ctx, pool, paymentrepo.Migrations()); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	validator := validate.New()
	repository := paymentrepo.NewRepository(pool)
	registry := metrics.NewRegistry()
	httpMetrics := metrics.NewHTTPServerMetrics("payment-service")
	registry.MustRegister(httpMetrics.Collectors()...)
	registry.MustRegister(
		metrics.NewGoRuntimeCollector("payment-service"),
		metrics.NewPostgresPoolConnectionsCollector("payment-service", pool),
		metrics.NewPostgresPoolUtilizationCollector("payment-service", pool),
	)

	service := paymentusecase.NewService(repository)
	handler := paymenthttp.NewHandler(service, validator)
	grpcHandler := paymentgrpc.NewServer(service)

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
		log.Fatalf("listen payment grpc: %v", err)
	}
	defer grpcListener.Close()

	grpcServer := grpcx.NewServer(log.Default())
	paymentv1.RegisterPaymentServiceServer(grpcServer, grpcHandler)

	serverErrors := make(chan error, 2)

	go func() {
		log.Printf("payment-service http listening on %s", cfg.HTTPAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrors <- fmt.Errorf("serve payment http: %w", err)
		}
	}()

	go func() {
		log.Printf("payment-service grpc listening on %s", cfg.GRPCAddr)
		if err := grpcServer.Serve(grpcListener); err != nil {
			serverErrors <- fmt.Errorf("serve payment grpc: %w", err)
		}
	}()

	select {
	case err := <-serverErrors:
		log.Fatalf("%v", err)
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
	grpcServer.GracefulStop()

	fmt.Println("payment-service stopped")
}
