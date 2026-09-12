package rest

import (
	"net/http"

	monobanksvc "github.com/rsmrtk/finance-engine/internal/service/monobank"
)

type monobankHandler struct {
	service *monobanksvc.Service
}

func newMonobankHandler(service *monobanksvc.Service) *monobankHandler {
	return &monobankHandler{service: service}
}

type connectMonobankRequest struct {
	PersonalToken string `json:"personalToken"`
}

func (h *monobankHandler) connect(w http.ResponseWriter, r *http.Request) {
	userID, _ := userIDFromContext(r.Context())
	var req connectMonobankRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	status, err := h.service.Connect(r.Context(), userID, req.PersonalToken)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, statusToJSON(status))
}

func (h *monobankHandler) status(w http.ResponseWriter, r *http.Request) {
	userID, _ := userIDFromContext(r.Context())
	status, err := h.service.Status(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get monobank status")
		return
	}
	writeJSON(w, http.StatusOK, statusToJSON(status))
}

func (h *monobankHandler) disconnect(w http.ResponseWriter, r *http.Request) {
	userID, _ := userIDFromContext(r.Context())
	if err := h.service.Disconnect(r.Context(), userID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to disconnect")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func statusToJSON(s monobanksvc.Status) monobankConnectionJSON {
	return monobankConnectionJSON{
		IsConnected:  s.IsConnected,
		MaskedPan:    s.MaskedPan,
		ConnectedAt:  formatOptionalTime(s.ConnectedAt),
		LastSyncedAt: formatOptionalTime(s.LastSyncedAt),
	}
}
