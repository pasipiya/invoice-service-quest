// Package invoice builds a customer invoice for one order. This is the single
// user-facing flow the Quest is scoped to.
package invoice

// Line is one priced line of an invoice, with tax resolved for its ship-to
// region. All money is in integer cents; there is no float arithmetic.
type Line struct {
	ProductID      int64  `json:"product_id"`
	SKU            string `json:"sku"`
	Name           string `json:"name"`
	Quantity       int    `json:"quantity"`
	UnitPriceCents int64  `json:"unit_price_cents"`
	Region         string `json:"region"`
	TaxRateBP      int    `json:"tax_rate_bp"`
	NetCents       int64  `json:"net_cents"`
	TaxCents       int64  `json:"tax_cents"`
	GrossCents     int64  `json:"gross_cents"`
}

// Invoice is the response of the invoice flow.
type Invoice struct {
	OrderID      int64  `json:"order_id"`
	CustomerName string `json:"customer_name"`
	Lines        []Line `json:"lines"`
	NetCents     int64  `json:"net_cents"`
	TaxCents     int64  `json:"tax_cents"`
	GrossCents   int64  `json:"gross_cents"`
}

// taxCents applies a basis-point rate to a net amount, rounding half up.
// Half up is the documented convention for this service; see docs/defects.md
// for the second, drifted copy of this rule.
func taxCents(netCents int64, rateBP int) int64 {
	return (netCents*int64(rateBP) + 5000) / 10000
}
