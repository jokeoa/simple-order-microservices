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

	paymentapp "github.com/jokeoa/simple-order-microservices/internal/payment/app"
	paymentrepo "github.com/jokeoa/simple-order-microservices/internal/payment/repository/postgres"
	paymenthttp "github.com/jokeoa/simple-order-microservices/internal/payment/transport/http"
	paymentusecase "github.com/jokeoa/simple-order-microservices/internal/payment/usecase"
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
	service := paymentusecase.NewService(repository)
	handler := paymenthttp.NewHandler(service, validator)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	handler.Register(mux)

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           mux,
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

	log.Printf("payment-service listening on %s", cfg.HTTPAddr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("serve payment-service: %v", err)
	}

	fmt.Println("payment-service stopped")
}
