package grpcx

import (
	"context"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

func NewServer(logger *log.Logger, opts ...grpc.ServerOption) *grpc.Server {
	serverOptions := []grpc.ServerOption{
		grpc.UnaryInterceptor(LoggingUnaryInterceptor(logger)),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             20 * time.Second,
			PermitWithoutStream: true,
		}),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle: 2 * time.Minute,
			Time:              30 * time.Second,
			Timeout:           10 * time.Second,
		}),
		grpc.MaxConcurrentStreams(128),
	}
	serverOptions = append(serverOptions, opts...)

	return grpc.NewServer(serverOptions...)
}

func LoggingUnaryInterceptor(logger *log.Logger) grpc.UnaryServerInterceptor {
	if logger == nil {
		logger = log.Default()
	}

	return func(
		ctx context.Context,
		request any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		startedAt := time.Now()
		response, err := handler(ctx, request)
		logger.Printf("grpc method=%s duration=%s err=%v", info.FullMethod, time.Since(startedAt), err)
		return response, err
	}
}
