// Package mcc maps merchant category codes (from card transactions) to the
// default category names seeded in migrations/00001_init.sql, so
// auto-imported Monobank transactions land in a sensible category instead
// of always "Інше".
package mcc

var categoryByMCC = map[int]string{
	// Продукти.
	5411: "Продукти", 5422: "Продукти", 5441: "Продукти",
	5451: "Продукти", 5462: "Продукти", 5499: "Продукти",
	5300: "Продукти", 5333: "Продукти",
	// Розваги (includes cafes/restaurants — no dedicated category for those yet).
	5812: "Розваги", 5813: "Розваги", 5814: "Розваги",
	7832: "Розваги", 7922: "Розваги", 7994: "Розваги", 7996: "Розваги",
	7998: "Розваги", 7999: "Розваги", 5815: "Розваги", 5816: "Розваги",
	5817: "Розваги", 5818: "Розваги", // App/game/streaming stores (Steam, App Store, Netflix-via-card, etc.).
	// Транспорт.
	4111: "Транспорт", 4121: "Транспорт", 4131: "Транспорт", 4112: "Транспорт",
	5541: "Транспорт", 5542: "Транспорт", 7523: "Транспорт", 7512: "Транспорт",
	4784: "Транспорт", // Tolls.
	// Одяг.
	5651: "Одяг", 5661: "Одяг", 5691: "Одяг", 5699: "Одяг", 5137: "Одяг", 5611: "Одяг", 5621: "Одяг", 5631: "Одяг",
	// Здоров'я.
	5912: "Здоров'я", 8011: "Здоров'я", 8021: "Здоров'я", 8062: "Здоров'я",
	8031: "Здоров'я", 8042: "Здоров'я", 8049: "Здоров'я", 5122: "Здоров'я",
	// Освіта.
	8220: "Освіта", 8241: "Освіта", 8299: "Освіта", 8211: "Освіта",
	// Житло (utilities, telecom, home improvement).
	4900: "Житло", 4814: "Житло", 4816: "Житло", 4899: "Житло", 5968: "Житло",
	1520: "Житло", 1711: "Житло", 1731: "Житло", 5200: "Житло", 5211: "Житло",
	5231: "Житло", 5251: "Житло", 5712: "Житло", 5719: "Житло", 5722: "Житло",
	5732: "Житло", 5734: "Житло",
}

// CategoryName returns the default category name for an MCC code, or ""
// if there's no mapping (caller should fall back to uncategorized/"Інше").
//
// This can never cover P2P transfers or card-to-card top-ups (they carry
// no MCC at all — that's a Monobank/card-network limitation, not a gap in
// this table) or a personal expense-tracking category like "Зарплата"
// (nothing in a card transaction says whose money is arriving or why).
func CategoryName(code int) string {
	return categoryByMCC[code]
}
