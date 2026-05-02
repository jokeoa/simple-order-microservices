package app

import (
	"time"

	"github.com/jokeoa/simple-order-microservices/internal/platform/env"
)

type Config struct {
	NATSURL          string
	PostgresDSN      string
	PostgresMaxConns int32
	MaxDeliver       int
	RetryDelay       time.Duration
	FetchWait        time.Duration
}

func LoadConfig() (Config, error) {
	postgresDSN, err := env.MustString("NOTIFICATION_POSTGRES_DSN")
	if err != nil {
		return Config{}, err
	}

	maxConns, err := env.Int("NOTIFICATION_POSTGRES_MAX_CONNS", 4)
	if err != nil {
		return Config{}, err
	}

	maxDeliver, err := env.Int("NOTIFICATION_MAX_DELIVER", 3)
	if err != nil {
		return Config{}, err
	}

	retryDelay, err := env.Duration("NOTIFICATION_RETRY_DELAY", 2*time.Second)
	if err != nil {
		return Config{}, err
	}

	fetchWait, err := env.Duration("NOTIFICATION_FETCH_WAIT", 2*time.Second)
	if err != nil {
		return Config{}, err
	}

	return Config{
		NATSURL:          env.String("NATS_URL", "nats://nats:4222"),
		PostgresDSN:      postgresDSN,
		PostgresMaxConns: int32(maxConns),
		MaxDeliver:       maxDeliver,
		RetryDelay:       retryDelay,
		FetchWait:        fetchWait,
	}, nil
}
