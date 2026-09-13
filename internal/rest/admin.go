package rest

import (
	"crypto/subtle"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/rsmrtk/finance-engine/internal/repository"
	adminsvc "github.com/rsmrtk/finance-engine/internal/service/admin"
	"github.com/rsmrtk/finance-engine/pkg/adminauth"
)

const adminTokenDuration = 12 * time.Hour

type adminHandler struct {
	service       *adminsvc.Service
	adminPassword string
	jwtSecret     string
}

func newAdminHandler(service *adminsvc.Service, adminPassword, jwtSecret string) *adminHandler {
	return &adminHandler{service: service, adminPassword: adminPassword, jwtSecret: jwtSecret}
}

// withAdminAuth checks the Authorization: Bearer header against a
// dedicated admin-subject JWT (pkg/adminauth) — a Bearer token, not a
// cookie, because finance-dashboard is a separate origin and this avoids
// the credentialed-CORS dance entirely.
func withAdminAuth(secret string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		if token == "" || adminauth.Verify(secret, token) != nil {
			writeError(w, http.StatusUnauthorized, "invalid or missing admin token")
			return
		}
		next(w, r)
	}
}

type adminLoginRequest struct {
	Password string `json:"password"`
}

func (h *adminHandler) login(w http.ResponseWriter, r *http.Request) {
	if h.adminPassword == "" {
		writeError(w, http.StatusServiceUnavailable, "admin login is not configured")
		return
	}
	var req adminLoginRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	// Constant-time comparison — this is a password check, not a lookup.
	if subtle.ConstantTimeCompare([]byte(req.Password), []byte(h.adminPassword)) != 1 {
		writeError(w, http.StatusUnauthorized, "incorrect password")
		return
	}

	token, err := adminauth.Generate(h.jwtSecret, adminTokenDuration)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to issue admin token")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}

type userJSONAdmin struct {
	ID                 string `json:"id"`
	Email              string `json:"email"`
	Plan               string `json:"plan"`
	SubscriptionStatus string `json:"subscriptionStatus"`
	TrialEndsAt        string `json:"trialEndsAt,omitempty"`
	BaseCurrency       string `json:"baseCurrency"`
	CreatedAt          string `json:"createdAt"`
}

func userToAdminJSON(u repository.User) userJSONAdmin {
	return userJSONAdmin{
		ID: u.ID.String(), Email: u.Email, Plan: u.Plan, SubscriptionStatus: u.SubscriptionStatus,
		TrialEndsAt: formatOptionalTime(u.TrialEndsAt), BaseCurrency: u.BaseCurrency,
		CreatedAt: u.CreatedAt.Format(timeLayout),
	}
}

func (h *adminHandler) listUsers(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)
	search := r.URL.Query().Get("q")
	planFilter := r.URL.Query().Get("plan")

	result, err := h.service.ListUsers(r.Context(), search, planFilter, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list users")
		return
	}
	out := make([]userJSONAdmin, len(result.Users))
	for i, u := range result.Users {
		out[i] = userToAdminJSON(u)
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": out, "total": result.Total})
}

func (h *adminHandler) getUser(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	detail, err := h.service.GetUser(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"user":             userToAdminJSON(detail.User),
		"transactionCount": detail.TransactionCount,
	})
}

type setPlanRequest struct {
	Plan string `json:"plan"`
}

func (h *adminHandler) setPlan(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	var req setPlanRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	user, err := h.service.SetPlan(r.Context(), id, req.Plan)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": userToAdminJSON(user)})
}

type extendTrialRequest struct {
	Days int `json:"days"`
}

func (h *adminHandler) extendTrial(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	var req extendTrialRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	user, err := h.service.ExtendTrial(r.Context(), id, req.Days)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": userToAdminJSON(user)})
}

func (h *adminHandler) cancelSubscription(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	if err := h.service.CancelSubscription(r.Context(), id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type daySignupsJSON struct {
	Day     string `json:"day"`
	Signups int64  `json:"signups"`
}

func (h *adminHandler) stats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.service.Stats(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load stats")
		return
	}
	signups := make([]daySignupsJSON, len(stats.SignupsByDay))
	for i, d := range stats.SignupsByDay {
		signups[i] = daySignupsJSON{Day: d.Day.Format("2006-01-02"), Signups: d.Signups}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"totalUsers":   stats.TotalUsers,
		"byPlan":       stats.ByPlan,
		"activeMRR":    stats.ActiveMRR,
		"trialCount":   stats.TrialCount,
		"signupsByDay": signups,
	})
}

func parsePagination(r *http.Request) (limit, offset int) {
	limit, offset = 50, 0
	if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && v > 0 && v <= 200 {
		limit = v
	}
	if v, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil && v >= 0 {
		offset = v
	}
	return limit, offset
}
