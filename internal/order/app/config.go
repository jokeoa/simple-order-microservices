package app

import (
	"fmt"
	"time"

	"github.com/jokeoa/simple-order-microservices/internal/platform/env"
)

type Config struct {
	HTTPAddr         string
	PostgresDSN      string
	PostgresMaxConns int32
	ReadTimeout      time.Duration
	WriteTimeout     time.Duration
	PaymentBaseURL   string
	PaymentTimeout   time.Duration
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

	paymentTimeout, err := env.Duration("PAYMENT_CLIENT_TIMEOUT", 2*time.Second)
	if err != nil {
		return Config{}, err
	}

	paymentBaseURL, err := env.MustString("PAYMENT_BASE_URL")
	if err != nil {
		return Config{}, err
	}

	if paymentTimeout > 2*time.Second {
		return Config{}, fmt.Errorf("PAYMENT_CLIENT_TIMEOUT must be <= 2s")
	}

	return Config{
		HTTPAddr:         env.String("ORDER_HTTP_ADDR", ":8080"),
		PostgresDSN:      postgresDSN,
		PostgresMaxConns: int32(maxConns),
		ReadTimeout:      readTimeout,
		WriteTimeout:     writeTimeout,
		PaymentBaseURL:   paymentBaseURL,
		PaymentTimeout:   paymentTimeout,
	}, nil
}
