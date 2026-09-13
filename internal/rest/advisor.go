package rest

import (
	"net/http"

	"github.com/rsmrtk/finance-engine/internal/plan"
	advisorsvc "github.com/rsmrtk/finance-engine/internal/service/advisor"
	authsvc "github.com/rsmrtk/finance-engine/internal/service/auth"
	"github.com/rsmrtk/finance-engine/pkg/ollama"
)

type advisorHandler struct {
	service *advisorsvc.Service
	auth    *authsvc.Service
}

func newAdvisorHandler(service *advisorsvc.Service, auth *authsvc.Service) *advisorHandler {
	return &advisorHandler{service: service, auth: auth}
}

type chatMessageJSON struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Message string            `json:"message"`
	History []chatMessageJSON `json:"history"`
}

func (h *advisorHandler) chat(w http.ResponseWriter, r *http.Request) {
	userID, _ := userIDFromContext(r.Context())
	var req chatRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Message == "" {
		writeError(w, http.StatusBadRequest, "message is required")
		return
	}

	user, err := h.auth.Me(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "not signed in")
		return
	}
	if !plan.AllowsFelix(user.Plan) {
		writeError(w, http.StatusForbidden, "Felix is available from the Pro plan")
		return
	}

	history := make([]ollama.Message, len(req.History))
	for i, m := range req.History {
		history[i] = ollama.Message{Role: m.Role, Content: m.Content}
	}

	reply, err := h.service.Chat(r.Context(), userID, user.BaseCurrency, history, req.Message)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "advisor unavailable — make sure `ollama serve` is running")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"reply": reply})
}

type financialScoreJSON struct {
	Total            int     `json:"total"`
	SavingsRate      float64 `json:"savingsRate"`
	SavingsScore     int     `json:"savingsScore"`
	TopCategoryShare float64 `json:"topCategoryShare"`
	BalanceScore     int     `json:"balanceScore"`
	ActiveDays       int     `json:"activeDays"`
	ConsistencyScore int     `json:"consistencyScore"`
}

// score is a plain computation over the user's own transactions (no LLM
// call) — available on every plan, unlike chat/insights, since it's not
// an AI feature. Clicking through to discuss it with Felix hits the
// existing Felix plan gate on its own.
func (h *advisorHandler) score(w http.ResponseWriter, r *http.Request) {
	userID, _ := userIDFromContext(r.Context())
	user, err := h.auth.Me(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "not signed in")
		return
	}
	score, err := h.service.FinancialScore(r.Context(), userID, user.BaseCurrency)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to compute financial score")
		return
	}
	writeJSON(w, http.StatusOK, financialScoreJSON{
		Total:            score.Total,
		SavingsRate:      score.SavingsRate,
		SavingsScore:     score.SavingsScore,
		TopCategoryShare: score.TopCategoryShare,
		BalanceScore:     score.BalanceScore,
		ActiveDays:       score.ActiveDays,
		ConsistencyScore: score.ConsistencyScore,
	})
}

func (h *advisorHandler) insights(w http.ResponseWriter, r *http.Request) {
	userID, _ := userIDFromContext(r.Context())
	user, err := h.auth.Me(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "not signed in")
		return
	}
	if !plan.AllowsFelix(user.Plan) {
		writeError(w, http.StatusForbidden, "Felix is available from the Pro plan")
		return
	}

	insights, err := h.service.Insights(r.Context(), userID, user.BaseCurrency)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "advisor unavailable — make sure `ollama serve` is running")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"insights": insights})
}
