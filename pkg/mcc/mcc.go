// Package mcc maps merchant category codes (from card transactions) to the
// default category names seeded in migrations/00001_init.sql, so
// auto-imported Monobank transactions land in a sensible category instead
// of always "Інше".
package mcc

var categoryByMCC = map[int]string{
	// Продукти.
	5411: "Продукти", 5422: "Продукти", 5441: "Продукти",
	5451: "Продукти", 5462: "Продукти", 5499: "Продукти",
	// Розваги (includes cafes/restaurants — no dedicated category for those yet).
	5812: "Розваги", 5813: "Розваги", 5814: "Розваги",
	7832: "Розваги", 7922: "Розваги", 7994: "Розваги",
	// Транспорт.
	4111: "Транспорт", 4121: "Транспорт", 4131: "Транспорт",
	5541: "Транспорт", 5542: "Транспорт", 7523: "Транспорт",
	// Одяг.
	5651: "Одяг", 5661: "Одяг", 5691: "Одяг", 5699: "Одяг",
	// Здоров'я.
	5912: "Здоров'я", 8011: "Здоров'я", 8021: "Здоров'я", 8062: "Здоров'я",
	// Освіта.
	8220: "Освіта", 8241: "Освіта", 8299: "Освіта",
}

// CategoryName returns the default category name for an MCC code, or ""
// if there's no mapping (caller should fall back to uncategorized/"Інше").
func CategoryName(code int) string {
	return categoryByMCC[code]
}
