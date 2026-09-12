package rest

import (
	"net/http"
	"time"

	ratesvc "github.com/rsmrtk/finance-engine/internal/service/rate"
)

type rateHandler struct {
	service *ratesvc.Service
}

func newRateHandler(service *ratesvc.Service) *rateHandler {
	return &rateHandler{service: service}
}

func (h *rateHandler) list(w http.ResponseWriter, r *http.Request) {
	rates, err := h.service.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list rates")
		return
	}
	out := make([]rateJSON, len(rates))
	for i, rt := range rates {
		out[i] = rateToJSON(rt)
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *rateHandler) history(w http.ResponseWriter, r *http.Request) {
	dateStr := r.URL.Query().Get("date")
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid date (expected YYYY-MM-DD)")
		return
	}
	rates, err := h.service.HistoryAt(r.Context(), date)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch historical rates")
		return
	}
	writeJSON(w, http.StatusOK, rates)
}
