package interceptors

import (
	"context"
	"time"

	"google.golang.org/grpc"

	"github.com/rsmrtk/finance-engine/pkg/logger"
)

type LoggerInterceptor struct {
	log logger.Logger
}

func NewLoggerInterceptor(log logger.Logger) *LoggerInterceptor {
	return &LoggerInterceptor{log: log}
}

func (i *LoggerInterceptor) Unary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)

		fields := logger.H{
			"method":      info.FullMethod,
			"duration_ms": time.Since(start).Milliseconds(),
		}
		if err != nil {
			fields["error"] = err.Error()
			i.log.Error("grpc request failed", fields)
		} else {
			i.log.Info("grpc request", fields)
		}
		return resp, err
	}
}
