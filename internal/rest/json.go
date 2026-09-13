package rest

import (
	"encoding/json"
	"net/http"

	"github.com/rsmrtk/finance-engine/pkg/logger"
)

// errLogger is set once from NewMux — every rest.go handler calls
// writeError instead of logging individually, so hooking it here catches
// every unexpected 5xx across the whole REST API without threading a
// logger field through each of the dozen handler structs. 4xx responses
// (bad input, wrong password, plan gate) are routine and expected, not
// logged — logging those would just bury the failures worth noticing.
var errLogger logger.Logger

func setErrorLogger(l logger.Logger) {
	errLogger = l
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	if status >= http.StatusInternalServerError && errLogger != nil {
		errLogger.Error("rest handler error", logger.H{"status": status, "message": message})
	}
	writeJSON(w, status, map[string]string{"error": message})
}

func decodeJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}
