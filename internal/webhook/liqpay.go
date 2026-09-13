package webhook

import (
	"net/http"

	billingsvc "github.com/rsmrtk/finance-engine/internal/service/billing"
	"github.com/rsmrtk/finance-engine/pkg/logger"
)

// LiqPayHandler handles POST /webhooks/liqpay — LiqPay's server_url
// callback for the Max plan's trial/subscription (internal/service/billing).
// Unauthenticated by cookie/JWT like every other webhook here; the
// data+signature pair is itself the authentication (see pkg/liqpay).
type LiqPayHandler struct {
	billing *billingsvc.Service
	log     logger.Logger
}

func NewLiqPayHandler(billing *billingsvc.Service, log logger.Logger) *LiqPayHandler {
	return &LiqPayHandler{billing: billing, log: log}
}

func (h *LiqPayHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	data, signature := r.FormValue("data"), r.FormValue("signature")
	if data == "" || signature == "" {
		h.log.Error("liqpay callback missing data/signature", logger.H{"remoteAddr": r.RemoteAddr})
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	h.log.Info("liqpay callback received", logger.H{"remoteAddr": r.RemoteAddr})
	if err := h.billing.HandleCallback(r.Context(), data, signature); err != nil {
		h.log.Error("liqpay callback failed", logger.H{"error": err.Error()})
		// Still 200: LiqPay retries on non-2xx, and a signature/lookup
		// failure won't fix itself on retry — just log it for a human.
	}
	w.WriteHeader(http.StatusOK)
}
