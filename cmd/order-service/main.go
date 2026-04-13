package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	orderapp "github.com/jokeoa/simple-order-microservices/internal/order/app"
	orderhttpclient "github.com/jokeoa/simple-order-microservices/internal/order/paymentclient/http"
	orderrepo "github.com/jokeoa/simple-order-microservices/internal/order/repository/postgres"
	ordertransport "github.com/jokeoa/simple-order-microservices/internal/order/transport/http"
	orderusecase "github.com/jokeoa/simple-order-microservices/internal/order/usecase"
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

	paymentClient := orderhttpclient.New(cfg.PaymentBaseURL, &http.Client{Timeout: cfg.PaymentTimeout}, paymentClientMetrics)
	service := orderusecase.NewService(repository, paymentClient)
	handler := ordertransport.NewHandler(service, validator)

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

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	log.Printf("order-service listening on %s", cfg.HTTPAddr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("serve order-service: %v", err)
	}

	fmt.Println("order-service stopped")
}
