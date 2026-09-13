package rest

import (
	"net/http"

	"github.com/rsmrtk/finance-engine/internal/plan"
	authsvc "github.com/rsmrtk/finance-engine/internal/service/auth"
	monobanksvc "github.com/rsmrtk/finance-engine/internal/service/monobank"
	"github.com/rsmrtk/finance-engine/pkg/logger"
)

type monobankHandler struct {
	service *monobanksvc.Service
	auth    *authsvc.Service
	log     logger.Logger
}

func newMonobankHandler(service *monobanksvc.Service, auth *authsvc.Service, log logger.Logger) *monobankHandler {
	return &monobankHandler{service: service, auth: auth, log: log}
}

type accountsMonobankRequest struct {
	PersonalToken string `json:"personalToken"`
}

type monobankAccountJSON struct {
	ID        string `json:"id"`
	MaskedPan string `json:"maskedPan"`
	Currency  string `json:"currency"`
	Type      string `json:"type"`
	Selected  bool   `json:"selected,omitempty"`
}

// accounts lets the frontend show a picker before connecting — a personal
// token can cover several cards/currencies/jars, and there's no safe way
// to guess which one the user means to track.
func (h *monobankHandler) accounts(w http.ResponseWriter, r *http.Request) {
	userID, _ := userIDFromContext(r.Context())
	user, err := h.auth.Me(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "not signed in")
		return
	}
	if !plan.AllowsMonobank(user.Plan) {
		writeError(w, http.StatusForbidden, "Monobank sync is available on the Max plan")
		return
	}

	var req accountsMonobankRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	options, err := h.service.ListAccounts(r.Context(), req.PersonalToken)
	if err != nil {
		h.log.Error("monobank list accounts failed", logger.H{"userId": userID.String(), "error": err.Error()})
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	h.log.Info("monobank accounts listed", logger.H{"userId": userID.String(), "count": len(options)})
	out := make([]monobankAccountJSON, len(options))
	for i, o := range options {
		out[i] = monobankAccountJSON{ID: o.ID, MaskedPan: o.MaskedPan, Currency: o.Currency, Type: o.Type}
	}
	writeJSON(w, http.StatusOK, out)
}

type connectMonobankRequest struct {
	PersonalToken string   `json:"personalToken"`
	AccountIDs    []string `json:"accountIds"`
	MaskedPans    []string `json:"maskedPans"`
}

func (h *monobankHandler) connect(w http.ResponseWriter, r *http.Request) {
	userID, _ := userIDFromContext(r.Context())
	user, err := h.auth.Me(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "not signed in")
		return
	}
	if !plan.AllowsMonobank(user.Plan) {
		writeError(w, http.StatusForbidden, "Monobank sync is available on the Max plan")
		return
	}

	var req connectMonobankRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	status, err := h.service.Connect(r.Context(), userID, req.PersonalToken, req.AccountIDs, req.MaskedPans)
	if err != nil {
		h.log.Error("monobank connect failed", logger.H{"userId": userID.String(), "accountIds": req.AccountIDs, "error": err.Error()})
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	h.log.Info("monobank connected", logger.H{"userId": userID.String(), "accountIds": req.AccountIDs, "isConnected": status.IsConnected})
	writeJSON(w, http.StatusOK, statusToJSON(status))
}

// myAccounts lets an already-connected user re-open the picker to add or
// remove tracked cards — decrypts the token we already stored instead of
// asking for it again.
func (h *monobankHandler) myAccounts(w http.ResponseWriter, r *http.Request) {
	userID, _ := userIDFromContext(r.Context())
	user, err := h.auth.Me(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "not signed in")
		return
	}
	if !plan.AllowsMonobank(user.Plan) {
		writeError(w, http.StatusForbidden, "Monobank sync is available on the Max plan")
		return
	}
	options, err := h.service.ListMyAccounts(r.Context(), userID)
	if err != nil {
		h.log.Error("monobank list my-accounts failed", logger.H{"userId": userID.String(), "error": err.Error()})
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	out := make([]monobankAccountJSON, len(options))
	for i, o := range options {
		out[i] = monobankAccountJSON{ID: o.ID, MaskedPan: o.MaskedPan, Currency: o.Currency, Type: o.Type, Selected: o.Selected}
	}
	writeJSON(w, http.StatusOK, out)
}

type updateAccountsMonobankRequest struct {
	AccountIDs []string `json:"accountIds"`
	MaskedPans []string `json:"maskedPans"`
}

func (h *monobankHandler) updateAccounts(w http.ResponseWriter, r *http.Request) {
	userID, _ := userIDFromContext(r.Context())
	user, err := h.auth.Me(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "not signed in")
		return
	}
	if !plan.AllowsMonobank(user.Plan) {
		writeError(w, http.StatusForbidden, "Monobank sync is available on the Max plan")
		return
	}
	var req updateAccountsMonobankRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	status, err := h.service.UpdateAccounts(r.Context(), userID, req.AccountIDs, req.MaskedPans)
	if err != nil {
		h.log.Error("monobank update accounts failed", logger.H{"userId": userID.String(), "accountIds": req.AccountIDs, "error": err.Error()})
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	h.log.Info("monobank tracked accounts updated", logger.H{"userId": userID.String(), "accountIds": req.AccountIDs})
	writeJSON(w, http.StatusOK, statusToJSON(status))
}

func (h *monobankHandler) status(w http.ResponseWriter, r *http.Request) {
	userID, _ := userIDFromContext(r.Context())
	status, err := h.service.Status(r.Context(), userID)
	if err != nil {
		h.log.Error("monobank status lookup failed", logger.H{"userId": userID.String(), "error": err.Error()})
		writeError(w, http.StatusInternalServerError, "failed to get monobank status")
		return
	}
	writeJSON(w, http.StatusOK, statusToJSON(status))
}

func (h *monobankHandler) disconnect(w http.ResponseWriter, r *http.Request) {
	userID, _ := userIDFromContext(r.Context())
	h.log.Info("monobank disconnect requested", logger.H{"userId": userID.String()})
	if err := h.service.Disconnect(r.Context(), userID); err != nil {
		h.log.Error("monobank disconnect failed", logger.H{"userId": userID.String(), "error": err.Error()})
		writeError(w, http.StatusInternalServerError, "failed to disconnect")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func statusToJSON(s monobanksvc.Status) monobankConnectionJSON {
	return monobankConnectionJSON{
		IsConnected:  s.IsConnected,
		MaskedPans:   s.MaskedPans,
		ConnectedAt:  formatOptionalTime(s.ConnectedAt),
		LastSyncedAt: formatOptionalTime(s.LastSyncedAt),
	}
}
