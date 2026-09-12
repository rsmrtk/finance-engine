// Package rest is the browser-reachable JSON API for the web frontend.
// It sits next to (not instead of) the gRPC server: iOS keeps talking
// gRPC/Apple-JWT unchanged, this package is purely additive. Every
// protected handler here calls straight into the same service-layer
// structs the gRPC controllers use (internal/service/...) — no business
// logic is duplicated, only JSON marshalling and the auth/CORS transport
// concerns a browser needs that gRPC doesn't.
package rest

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/rsmrtk/finance-engine/pkg/jwt"
)

type contextKey string

const userIDContextKey contextKey = "user_id"

func userIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(userIDContextKey).(uuid.UUID)
	return id, ok
}

// withCORS restricts the API to one exact origin. A credentialed request
// (cookies) can never use "Access-Control-Allow-Origin: *" — the browser
// rejects that combination — so this always echoes back the one configured
// origin rather than a wildcard.
func withCORS(allowedOrigin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// withAuth reads the access token from the httpOnly cookie set at login,
// falling back to a Bearer header (useful for curl/future non-browser
// clients). Mirrors internal/grpc/interceptors.AuthInterceptor.
func withAuth(jwtManager jwt.JWT, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		if token == "" {
			if cookie, err := r.Cookie(accessCookieName); err == nil {
				token = cookie.Value
			}
		}
		if token == "" {
			writeError(w, http.StatusUnauthorized, "missing access token")
			return
		}

		userID, err := jwtManager.Verify(token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}

		ctx := context.WithValue(r.Context(), userIDContextKey, userID)
		next(w, r.WithContext(ctx))
	}
}

func bearerToken(r *http.Request) string {
	const prefix = "Bearer "
	h := r.Header.Get("Authorization")
	if len(h) > len(prefix) && h[:len(prefix)] == prefix {
		return h[len(prefix):]
	}
	return ""
}
