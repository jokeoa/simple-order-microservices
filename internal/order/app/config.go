package app

import (
	"fmt"
	"time"

	"github.com/jokeoa/simple-order-microservices/internal/platform/env"
)

type Config struct {
	HTTPAddr          string
	GRPCAddr          string
	PostgresDSN       string
	PostgresMaxConns  int32
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	StreamTimeout     time.Duration
	PaymentGRPCTarget string
	PaymentTimeout    time.Duration
}

func LoadConfig() (Config, error) {
	postgresDSN, err := env.MustString("ORDER_POSTGRES_DSN")
	if err != nil {
		return Config{}, err
	}

	maxConns, err := env.Int("ORDER_POSTGRES_MAX_CONNS", 4)
	if err != nil {
		return Config{}, err
	}

	readTimeout, err := env.Duration("ORDER_HTTP_READ_TIMEOUT", 5*time.Second)
	if err != nil {
		return Config{}, err
	}

	writeTimeout, err := env.Duration("ORDER_HTTP_WRITE_TIMEOUT", 5*time.Second)
	if err != nil {
		return Config{}, err
	}

	streamTimeout, err := env.Duration("ORDER_GRPC_STREAM_TIMEOUT", 5*time.Minute)
	if err != nil {
		return Config{}, err
	}

	grpcAddr, err := env.MustString("ORDER_GRPC_ADDR")
	if err != nil {
		return Config{}, err
	}

	paymentTimeout, err := env.Duration("PAYMENT_GRPC_TIMEOUT", 2*time.Second)
	if err != nil {
		return Config{}, err
	}

	paymentTarget, err := env.MustString("PAYMENT_GRPC_TARGET")
	if err != nil {
		return Config{}, err
	}

	if paymentTimeout > 2*time.Second {
		return Config{}, fmt.Errorf("PAYMENT_CLIENT_TIMEOUT must be <= 2s")
	}

	return Config{
		HTTPAddr:          env.String("ORDER_HTTP_ADDR", ":8080"),
		GRPCAddr:          grpcAddr,
		PostgresDSN:       postgresDSN,
		PostgresMaxConns:  int32(maxConns),
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		StreamTimeout:     streamTimeout,
		PaymentGRPCTarget: paymentTarget,
		PaymentTimeout:    paymentTimeout,
	}, nil
}
