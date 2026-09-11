package interceptors

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/rsmrtk/finance-engine/pkg/logger"
)

type RecoveryInterceptor struct {
	log logger.Logger
}

func NewRecoveryInterceptor(log logger.Logger) *RecoveryInterceptor {
	return &RecoveryInterceptor{log: log}
}

func (i *RecoveryInterceptor) Unary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				i.log.Error("panic recovered", logger.H{"method": info.FullMethod, "panic": fmt.Sprintf("%v", r)})
				err = status.Error(codes.Internal, "internal server error")
			}
		}()
		return handler(ctx, req)
	}
}
