package rest

import (
	"io"
	"net/http"
	"time"

	authsvc "github.com/rsmrtk/finance-engine/internal/service/auth"
	reportssvc "github.com/rsmrtk/finance-engine/internal/service/reports"
)

type reportsHandler struct {
	service *reportssvc.Service
	auth    *authsvc.Service
}

func newReportsHandler(service *reportssvc.Service, auth *authsvc.Service) *reportsHandler {
	return &reportsHandler{service: service, auth: auth}
}

// export handles GET /api/reports/export?from=2026-09-01&to=2026-09-30 —
// both optional, defaulting to the current calendar month. Streams a CSV
// download rather than JSON; the frontend triggers it as a plain link.
func (h *reportsHandler) export(w http.ResponseWriter, r *http.Request) {
	userID, _ := userIDFromContext(r.Context())

	now := time.Now()
	from := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	to := now
	if v := r.URL.Query().Get("from"); v != "" {
		if parsed, err := time.Parse("2006-01-02", v); err == nil {
			from = parsed
		}
	}
	if v := r.URL.Query().Get("to"); v != "" {
		if parsed, err := time.Parse("2006-01-02", v); err == nil {
			// Inclusive of the whole end day.
			to = parsed.Add(24*time.Hour - time.Second)
		}
	}

	data, err := h.service.Export(r.Context(), userID, from, to)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to build export")
		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	filename := "vaultly-" + from.Format("2006-01-02") + "_" + to.Format("2006-01-02") + ".csv"
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

type importResultJSON struct {
	Imported int      `json:"imported"`
	Skipped  int      `json:"skipped"`
	Errors   []string `json:"errors"`
}

// importCSV handles POST /api/reports/import — multipart form with a
// "file" field, the same column shape export produces.
func (h *reportsHandler) importCSV(w http.ResponseWriter, r *http.Request) {
	userID, _ := userIDFromContext(r.Context())

	const maxUploadSize = 5 << 20 // 5MB — a spreadsheet of expenses is never remotely this large.
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		writeError(w, http.StatusBadRequest, "file too large or invalid upload")
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing file")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxUploadSize))
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read file")
		return
	}

	user, err := h.auth.Me(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "not signed in")
		return
	}

	result, err := h.service.Import(r.Context(), userID, user.BaseCurrency, data)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, importResultJSON{Imported: result.Imported, Skipped: result.Skipped, Errors: result.Errors})
}
