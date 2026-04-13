package app

import (
	"time"

	"github.com/jokeoa/simple-order-microservices/internal/platform/env"
)

type Config struct {
	HTTPAddr         string
	GRPCAddr         string
	PostgresDSN      string
	PostgresMaxConns int32
	ReadTimeout      time.Duration
	WriteTimeout     time.Duration
}

func LoadConfig() (Config, error) {
	postgresDSN, err := env.MustString("PAYMENT_POSTGRES_DSN")
	if err != nil {
		return Config{}, err
	}

	maxConns, err := env.Int("PAYMENT_POSTGRES_MAX_CONNS", 4)
	if err != nil {
		return Config{}, err
	}

	readTimeout, err := env.Duration("PAYMENT_HTTP_READ_TIMEOUT", 5*time.Second)
	if err != nil {
		return Config{}, err
	}

	writeTimeout, err := env.Duration("PAYMENT_HTTP_WRITE_TIMEOUT", 5*time.Second)
	if err != nil {
		return Config{}, err
	}

	grpcAddr, err := env.MustString("PAYMENT_GRPC_ADDR")
	if err != nil {
		return Config{}, err
	}

	return Config{
		HTTPAddr:         env.String("PAYMENT_HTTP_ADDR", ":8081"),
		GRPCAddr:         grpcAddr,
		PostgresDSN:      postgresDSN,
		PostgresMaxConns: int32(maxConns),
		ReadTimeout:      readTimeout,
		WriteTimeout:     writeTimeout,
	}, nil
}
