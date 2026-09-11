package interceptors

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/rsmrtk/finance-engine/pkg/jwt"
)

type contextKey string

const userIDContextKey contextKey = "user_id"

type AuthInterceptor struct {
	jwt                     jwt.JWT
	methodsDoNotRequireAuth map[string]struct{}
}

func NewAuthInterceptor(jwtManager jwt.JWT, methodsDoNotRequireAuth []string) *AuthInterceptor {
	set := make(map[string]struct{}, len(methodsDoNotRequireAuth))
	for _, m := range methodsDoNotRequireAuth {
		set[m] = struct{}{}
	}
	return &AuthInterceptor{jwt: jwtManager, methodsDoNotRequireAuth: set}
}

func (i *AuthInterceptor) Unary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if _, ok := i.methodsDoNotRequireAuth[info.FullMethod]; ok {
			return handler(ctx, req)
		}
		authedCtx, err := i.authorize(ctx)
		if err != nil {
			return nil, err
		}
		return handler(authedCtx, req)
	}
}

func (i *AuthInterceptor) authorize(ctx context.Context) (context.Context, error) {
	values := metadata.ValueFromIncomingContext(ctx, "authorization")
	if len(values) == 0 {
		return nil, status.Error(codes.Unauthenticated, "authorization header is not provided")
	}

	parts := strings.SplitN(values[0], " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return nil, status.Error(codes.Unauthenticated, "expected 'Bearer <token>' authorization header")
	}

	userID, err := i.jwt.Verify(parts[1])
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid or expired token")
	}

	return context.WithValue(ctx, userIDContextKey, userID), nil
}

// UserIDFromContext reads the authenticated user id set by AuthInterceptor.
// Every controller method except SignInWithApple can rely on this being
// present, since the interceptor rejects the request otherwise.
func UserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(userIDContextKey).(uuid.UUID)
	return id, ok
}
