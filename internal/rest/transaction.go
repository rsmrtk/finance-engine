package rest

import (
	"net/http"
	"time"

	"github.com/google/uuid"

	transactionsvc "github.com/rsmrtk/finance-engine/internal/service/transaction"
)

type transactionHandler struct {
	service *transactionsvc.Service
}

func newTransactionHandler(service *transactionsvc.Service) *transactionHandler {
	return &transactionHandler{service: service}
}

type createTransactionRequest struct {
	CategoryID string `json:"categoryId"`
	Amount     string `json:"amount"`
	Currency   string `json:"currency"`
	Type       string `json:"type"`
	Date       string `json:"date"`
	Note       string `json:"note"`
}

func (h *transactionHandler) list(w http.ResponseWriter, r *http.Request) {
	userID, _ := userIDFromContext(r.Context())

	from, err := parseOptionalTime(r.URL.Query().Get("from"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid from")
		return
	}
	to, err := parseOptionalTime(r.URL.Query().Get("to"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid to")
		return
	}

	transactions, err := h.service.List(r.Context(), userID, from, to)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list transactions")
		return
	}
	out := make([]transactionJSON, len(transactions))
	for i, t := range transactions {
		out[i] = transactionToJSON(t)
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *transactionHandler) create(w http.ResponseWriter, r *http.Request) {
	userID, _ := userIDFromContext(r.Context())

	var req createTransactionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	date, err := time.Parse(timeLayout, req.Date)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid date")
		return
	}
	var categoryID uuid.UUID
	if req.CategoryID != "" {
		categoryID, err = uuid.Parse(req.CategoryID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid categoryId")
			return
		}
	}

	created, err := h.service.Create(r.Context(), transactionsvc.CreateParams{
		UserID: userID, CategoryID: categoryID, Amount: req.Amount,
		Currency: req.Currency, Type: req.Type, Date: date, Note: req.Note,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, transactionToJSON(created))
}

func (h *transactionHandler) update(w http.ResponseWriter, r *http.Request) {
	userID, _ := userIDFromContext(r.Context())
	transactionID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid transaction id")
		return
	}

	var req createTransactionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	date, err := time.Parse(timeLayout, req.Date)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid date")
		return
	}
	var categoryID uuid.UUID
	if req.CategoryID != "" {
		categoryID, err = uuid.Parse(req.CategoryID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid categoryId")
			return
		}
	}

	updated, err := h.service.Update(r.Context(), transactionsvc.UpdateParams{
		ID: transactionID, UserID: userID, CategoryID: categoryID, Amount: req.Amount,
		Currency: req.Currency, Type: req.Type, Date: date, Note: req.Note,
	})
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, transactionToJSON(updated))
}

func (h *transactionHandler) delete(w http.ResponseWriter, r *http.Request) {
	userID, _ := userIDFromContext(r.Context())
	transactionID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid transaction id")
		return
	}
	if err := h.service.Delete(r.Context(), userID, transactionID); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func parseOptionalTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	return time.Parse(timeLayout, s)
}
