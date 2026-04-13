package grpcx

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials/insecure"
)

const paymentServiceRetryPolicy = `{
  "methodConfig": [{
    "name": [{"service": "payment.v1.PaymentService"}],
    "retryPolicy": {
      "maxAttempts": 3,
      "initialBackoff": "0.2s",
      "maxBackoff": "1s",
      "backoffMultiplier": 2.0,
      "retryableStatusCodes": ["UNAVAILABLE", "DEADLINE_EXCEEDED"]
    }
  }]
}`

func DialPayment(ctx context.Context, target string) (*grpc.ClientConn, error) {
	connection, err := grpc.DialContext(
		ctx,
		target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithConnectParams(grpc.ConnectParams{
			Backoff: backoff.Config{
				BaseDelay:  200 * time.Millisecond,
				Multiplier: 1.6,
				Jitter:     0.2,
				MaxDelay:   time.Second,
			},
			MinConnectTimeout: 2 * time.Second,
		}),
		grpc.WithDefaultServiceConfig(paymentServiceRetryPolicy),
	)
	if err != nil {
		return nil, fmt.Errorf("dial grpc target %s: %w", target, err)
	}

	return connection, nil
}
