package monobank

// WebhookPayload is what Monobank POSTs to our registered webhook URL for
// every new transaction. See https://api.monobank.ua/docs/#operation--personal-webhook-post.
type WebhookPayload struct {
	Type string `json:"type"` // Always "StatementItem" for transaction events.
	Data struct {
		Account       string        `json:"account"`
		StatementItem StatementItem `json:"statementItem"`
	} `json:"data"`
}

type StatementItem struct {
	ID          string `json:"id"`
	Time        int64  `json:"time"` // Unix seconds.
	Description string `json:"description"`
	MCC         int    `json:"mcc"`
	// Amount is in the TRACKED ACCOUNT's own currency (CurrencyCode) —
	// Monobank's /personal/statement is scoped to one specific accountID,
	// so this is what actually moved in that account's balance. Minor
	// units (kopecks/cents); negative = expense, positive = income.
	Amount       int64 `json:"amount"`
	CurrencyCode int   `json:"currencyCode"`
	// OperationAmount/OperationCurrencyCode describe the currency the
	// operation was actually carried out in (e.g. paying a USD-priced
	// merchant from a EUR account) — captured for diagnostics only, not
	// used to compute the stored transaction amount: Amount+CurrencyCode
	// is Monobank's own documented pair for "what changed in this
	// account," which is what this app needs. Kept around because a
	// wildly implausible Amount (e.g. a routine purchase showing five
	// figures in EUR) is easier to diagnose against OperationAmount than
	// to guess at blind.
	OperationAmount       int64 `json:"operationAmount"`
	OperationCurrencyCode int   `json:"operationCurrencyCode"`
}
