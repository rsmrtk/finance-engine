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
	ID           string `json:"id"`
	Time         int64  `json:"time"` // Unix seconds.
	Description  string `json:"description"`
	MCC          int    `json:"mcc"`
	Amount       int64  `json:"amount"` // Minor units (kopecks); negative = expense, positive = income.
	CurrencyCode int    `json:"currencyCode"`
}
