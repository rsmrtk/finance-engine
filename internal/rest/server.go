package rest

import (
	"net/http"
	"time"

	grpcserver "github.com/rsmrtk/finance-engine/internal/grpc"
	adminsvc "github.com/rsmrtk/finance-engine/internal/service/admin"
	advisorsvc "github.com/rsmrtk/finance-engine/internal/service/advisor"
	billingsvc "github.com/rsmrtk/finance-engine/internal/service/billing"
	sessionsvc "github.com/rsmrtk/finance-engine/internal/service/session"
	"github.com/rsmrtk/finance-engine/pkg/jwt"
	"github.com/rsmrtk/finance-engine/pkg/logger"
)

type Options struct {
	Services      grpcserver.Services
	Sessions      *sessionsvc.Service
	Advisor       *advisorsvc.Service // Web-only — no gRPC/iOS counterpart, so it lives outside Services.
	Billing       *billingsvc.Service // Web-only — the Max plan trial only exists on the web app.
	Admin         *adminsvc.Service   // finance-dashboard only.
	JWT           jwt.JWT             // Verifies access tokens — same secret as gRPC, see cmd/server/main.go.
	CORSOrigin    string              // finance-ui's origin — also used for cookie scheme + LiqPay result_url.
	AdminOrigin   string              // finance-dashboard's origin — CORS only, no cookies involved.
	AdminPassword string
	JWTSecret     string // Signs both user and admin tokens (distinct subjects — see pkg/adminauth).
	AccessMaxAge  time.Duration
	RefreshMaxAge time.Duration
	Log           logger.Logger
}

// NewMux builds the web frontend's JSON API. Every protected route calls
// straight into the same service-layer struct the gRPC controllers use
// (o.Services.*) — this package only adds JSON marshalling, cookie-based
// auth, and CORS, none of which gRPC/iOS needs.
func NewMux(o Options) http.Handler {
	setErrorLogger(o.Log)
	mux := http.NewServeMux()

	auth := newAuthHandler(o.Services.Auth, o.Sessions, o.CORSOrigin, o.AccessMaxAge, o.RefreshMaxAge)
	mux.HandleFunc("POST /api/auth/signup", auth.signup)
	mux.HandleFunc("POST /api/auth/login", auth.login)
	mux.HandleFunc("POST /api/auth/google", auth.google)
	mux.HandleFunc("POST /api/auth/refresh", auth.refresh)
	mux.HandleFunc("POST /api/auth/logout", auth.logout)
	mux.HandleFunc("GET /api/auth/me", withAuth(o.JWT, auth.me))
	mux.HandleFunc("PATCH /api/auth/preferences", withAuth(o.JWT, auth.updatePreferences))
	mux.HandleFunc("PATCH /api/auth/profile", withAuth(o.JWT, auth.updateProfile))
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

	monobank := newMonobankHandler(o.Services.Monobank, o.Services.Auth, o.Log)
	mux.HandleFunc("POST /api/monobank/accounts", withAuth(o.JWT, monobank.accounts))
	mux.HandleFunc("POST /api/monobank/connect", withAuth(o.JWT, monobank.connect))
	mux.HandleFunc("GET /api/monobank/my-accounts", withAuth(o.JWT, monobank.myAccounts))
	mux.HandleFunc("PUT /api/monobank/accounts", withAuth(o.JWT, monobank.updateAccounts))
	mux.HandleFunc("GET /api/monobank/status", withAuth(o.JWT, monobank.status))
	mux.HandleFunc("POST /api/monobank/disconnect", withAuth(o.JWT, monobank.disconnect))

	advisor := newAdvisorHandler(o.Advisor, o.Services.Auth)
	mux.HandleFunc("POST /api/advisor/chat", withAuth(o.JWT, advisor.chat))
	mux.HandleFunc("GET /api/advisor/insights", withAuth(o.JWT, advisor.insights))
	mux.HandleFunc("GET /api/advisor/score", withAuth(o.JWT, advisor.score))

	billing := newBillingHandler(o.Billing, o.Log)
	mux.HandleFunc("POST /api/billing/start-trial", withAuth(o.JWT, billing.startTrial))
	mux.HandleFunc("POST /api/billing/subscribe", withAuth(o.JWT, billing.subscribe))
	mux.HandleFunc("POST /api/billing/cancel", withAuth(o.JWT, billing.cancel))
	mux.HandleFunc("GET /api/billing/receipts", withAuth(o.JWT, billing.receipts))

	admin := newAdminHandler(o.Admin, o.AdminPassword, o.JWTSecret)
	mux.HandleFunc("POST /api/admin/login", admin.login)
	mux.HandleFunc("GET /api/admin/users", withAdminAuth(o.JWTSecret, admin.listUsers))
	mux.HandleFunc("GET /api/admin/users/{id}", withAdminAuth(o.JWTSecret, admin.getUser))
	mux.HandleFunc("PATCH /api/admin/users/{id}/plan", withAdminAuth(o.JWTSecret, admin.setPlan))
	mux.HandleFunc("POST /api/admin/users/{id}/extend-trial", withAdminAuth(o.JWTSecret, admin.extendTrial))
	mux.HandleFunc("POST /api/admin/users/{id}/cancel", withAdminAuth(o.JWTSecret, admin.cancelSubscription))
	mux.HandleFunc("GET /api/admin/stats", withAdminAuth(o.JWTSecret, admin.stats))

	return withCORS([]string{o.CORSOrigin, o.AdminOrigin}, mux)
}
