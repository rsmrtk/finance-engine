package rest

import (
	"net/http"

	"github.com/google/uuid"

	categorysvc "github.com/rsmrtk/finance-engine/internal/service/category"
)

type categoryHandler struct {
	service *categorysvc.Service
}

func newCategoryHandler(service *categorysvc.Service) *categoryHandler {
	return &categoryHandler{service: service}
}

type createCategoryRequest struct {
	Name     string `json:"name"`
	IconName string `json:"iconName"`
	ColorHex string `json:"colorHex"`
	Type     string `json:"type"`
}

func (h *categoryHandler) list(w http.ResponseWriter, r *http.Request) {
	userID, _ := userIDFromContext(r.Context())
	categories, err := h.service.List(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list categories")
		return
	}
	out := make([]categoryJSON, len(categories))
	for i, c := range categories {
		out[i] = categoryToJSON(c)
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *categoryHandler) create(w http.ResponseWriter, r *http.Request) {
	userID, _ := userIDFromContext(r.Context())
	var req createCategoryRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	created, err := h.service.Create(r.Context(), userID, req.Name, req.IconName, req.ColorHex, req.Type)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, categoryToJSON(created))
}

func (h *categoryHandler) delete(w http.ResponseWriter, r *http.Request) {
	userID, _ := userIDFromContext(r.Context())
	categoryID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid category id")
		return
	}
	if err := h.service.Delete(r.Context(), userID, categoryID); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
