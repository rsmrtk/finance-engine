package monobank

// currencyByISONumeric maps ISO 4217 numeric codes (what Monobank sends) to
// the currency codes used throughout finance-engine.
var currencyByISONumeric = map[int]string{
	980: "UAH",
	840: "USD",
	978: "EUR",
	826: "GBP",
	985: "PLN",
}

// CurrencyCode converts an ISO 4217 numeric code to our currency code,
// defaulting to UAH for currencies we don't otherwise support.
func CurrencyCode(isoNumeric int) string {
	if code, ok := currencyByISONumeric[isoNumeric]; ok {
		return code
	}
	return "UAH"
}
