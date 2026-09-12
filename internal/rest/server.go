package rest

import (
	"net/http"
	"time"

	grpcserver "github.com/rsmrtk/finance-engine/internal/grpc"
	sessionsvc "github.com/rsmrtk/finance-engine/internal/service/session"
	"github.com/rsmrtk/finance-engine/pkg/jwt"
)

type Options struct {
	Services      grpcserver.Services
	Sessions      *sessionsvc.Service
	JWT           jwt.JWT // Verifies access tokens — same secret as gRPC, see cmd/server/main.go.
	CORSOrigin    string
	AccessMaxAge  time.Duration
	RefreshMaxAge time.Duration
}

// NewMux builds the web frontend's JSON API. Every protected route calls
// straight into the same service-layer struct the gRPC controllers use
// (o.Services.*) — this package only adds JSON marshalling, cookie-based
// auth, and CORS, none of which gRPC/iOS needs.
func NewMux(o Options) http.Handler {
	mux := http.NewServeMux()

	auth := newAuthHandler(o.Services.Auth, o.Sessions, o.CORSOrigin, o.AccessMaxAge, o.RefreshMaxAge)
	mux.HandleFunc("POST /api/auth/signup", auth.signup)
	mux.HandleFunc("POST /api/auth/login", auth.login)
	mux.HandleFunc("POST /api/auth/google", auth.google)
	mux.HandleFunc("POST /api/auth/refresh", auth.refresh)
	mux.HandleFunc("POST /api/auth/logout", auth.logout)
	mux.HandleFunc("GET /api/auth/me", withAuth(o.JWT, auth.me))
	mux.HandleFunc("PATCH /api/auth/preferences", withAuth(o.JWT, auth.updatePreferences))
	mux.HandleFunc("PATCH /api/auth/goals", withAuth(o.JWT, auth.updateGoals))
	mux.HandleFunc("PATCH /api/auth/currency", withAuth(o.JWT, auth.updateCurrency))
	mux.HandleFunc("GET /api/auth/sessions", withAuth(o.JWT, auth.listSessions))
	mux.HandleFunc("DELETE /api/auth/sessions/{id}", withAuth(o.JWT, auth.revokeSession))

	categories := newCategoryHandler(o.Services.Category)
	mux.HandleFunc("GET /api/categories", withAuth(o.JWT, categories.list))
	mux.HandleFunc("POST /api/categories", withAuth(o.JWT, categories.create))
	mux.HandleFunc("DELETE /api/categories/{id}", withAuth(o.JWT, categories.delete))

	transactions := newTransactionHandler(o.Services.Transaction)
	mux.HandleFunc("GET /api/transactions", withAuth(o.JWT, transactions.list))
	mux.HandleFunc("POST /api/transactions", withAuth(o.JWT, transactions.create))
	mux.HandleFunc("PUT /api/transactions/{id}", withAuth(o.JWT, transactions.update))
	mux.HandleFunc("DELETE /api/transactions/{id}", withAuth(o.JWT, transactions.delete))

	rates := newRateHandler(o.Services.Rate)
	mux.HandleFunc("GET /api/rates", withAuth(o.JWT, rates.list))
	mux.HandleFunc("GET /api/rates/history", withAuth(o.JWT, rates.history))

	monobank := newMonobankHandler(o.Services.Monobank)
	mux.HandleFunc("POST /api/monobank/connect", withAuth(o.JWT, monobank.connect))
	mux.HandleFunc("GET /api/monobank/status", withAuth(o.JWT, monobank.status))
	mux.HandleFunc("POST /api/monobank/disconnect", withAuth(o.JWT, monobank.disconnect))

	return withCORS(o.CORSOrigin, mux)
}
