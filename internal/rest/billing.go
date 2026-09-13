package rest

import (
	"net/http"
	"time"

	billingsvc "github.com/rsmrtk/finance-engine/internal/service/billing"
	"github.com/rsmrtk/finance-engine/pkg/logger"
)

type billingHandler struct {
	service *billingsvc.Service
	log     logger.Logger
}

func newBillingHandler(service *billingsvc.Service, log logger.Logger) *billingHandler {
	return &billingHandler{service: service, log: log}
}

type checkoutJSON struct {
	URL       string `json:"url"`
	Data      string `json:"data"`
	Signature string `json:"signature"`
}

func (h *billingHandler) startTrial(w http.ResponseWriter, r *http.Request) {
	userID, _ := userIDFromContext(r.Context())
	checkout, err := h.service.StartTrial(r.Context(), userID)
	if err != nil {
		h.log.Error("billing start-trial failed", logger.H{"userId": userID.String(), "error": err.Error()})
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	h.log.Info("billing checkout built", logger.H{"userId": userID.String(), "flow": "trial", "plan": "max"})
	writeJSON(w, http.StatusOK, checkoutJSON{URL: checkout.URL, Data: checkout.Data, Signature: checkout.Signature})
}

type subscribeRequest struct {
	Plan string `json:"plan"`
}

func (h *billingHandler) subscribe(w http.ResponseWriter, r *http.Request) {
	userID, _ := userIDFromContext(r.Context())
	var req subscribeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	checkout, err := h.service.Subscribe(r.Context(), userID, req.Plan)
	if err != nil {
		h.log.Error("billing subscribe failed", logger.H{"userId": userID.String(), "plan": req.Plan, "error": err.Error()})
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	h.log.Info("billing checkout built", logger.H{"userId": userID.String(), "flow": "direct", "plan": req.Plan})
	writeJSON(w, http.StatusOK, checkoutJSON{URL: checkout.URL, Data: checkout.Data, Signature: checkout.Signature})
}

type receiptJSON struct {
	OrderID          string  `json:"orderId"`
	Plan             string  `json:"plan"`
	Status           string  `json:"status"`
	Amount           float64 `json:"amount"`
	Currency         string  `json:"currency"`
	ErrorDescription string  `json:"errorDescription"`
	CreatedAt        string  `json:"createdAt"`
}

func (h *billingHandler) receipts(w http.ResponseWriter, r *http.Request) {
	userID, _ := userIDFromContext(r.Context())
	events, err := h.service.Receipts(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load receipts")
		return
	}
	out := make([]receiptJSON, len(events))
	for i, e := range events {
		out[i] = receiptJSON{
			OrderID:          e.OrderID,
			Plan:             e.Plan,
			Status:           e.Status,
			Amount:           e.Amount,
			Currency:         e.Currency,
			ErrorDescription: e.ErrorDescription,
			CreatedAt:        e.CreatedAt.Format(time.RFC3339),
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *billingHandler) cancel(w http.ResponseWriter, r *http.Request) {
	userID, _ := userIDFromContext(r.Context())
	if err := h.service.Cancel(r.Context(), userID); err != nil {
		h.log.Error("billing cancel failed", logger.H{"userId": userID.String(), "error": err.Error()})
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	h.log.Info("billing subscription canceled", logger.H{"userId": userID.String()})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
