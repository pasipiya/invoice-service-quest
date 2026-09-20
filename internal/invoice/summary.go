package invoice

import "context"

// Summary is the lightweight order total shown in the account order list.
type Summary struct {
	OrderID    int64 `json:"order_id"`
	LineCount  int   `json:"line_count"`
	NetCents   int64 `json:"net_cents"`
	TaxCents   int64 `json:"tax_cents"`
	GrossCents int64 `json:"gross_cents"`
}

// summaryTaxCents applies a basis-point rate to a net amount.
//
// DEFECT-2 (deliberate, NON-GOAL — not fixed in this Quest): this is a second,
// drifted copy of the tax rule in types.go. That one rounds half up; this one
// truncates. The two therefore disagree by up to one cent per line, so the
// total a customer sees in their order list can differ from the total on the
// invoice for the same order. The rule also exists in two places, so any rate
// or rounding change has to be made twice and will be made once.
//
// Left in place on purpose. See docs/defects.md and the non-goals in
// docs/intent.md.
func summaryTaxCents(netCents int64, rateBP int) int64 {
	return netCents * int64(rateBP) / 10000 // truncates; types.go rounds half up
}

// Summarise totals an order for the account order list.
func (s *Service) Summarise(ctx context.Context, orderID int64) (Summary, error) {
	items, err := s.repo.ListLineItems(ctx, orderID)
	if err != nil {
		return Summary{}, err
	}

	sum := Summary{OrderID: orderID, LineCount: len(items)}
	for _, it := range items {
		product, err := s.repo.GetProductByID(ctx, it.ProductID)
		if err != nil {
			return Summary{}, err
		}
		rule, err := s.repo.GetTaxRuleForRegion(ctx, it.Region)
		if err != nil {
			return Summary{}, err
		}

		net := product.UnitPriceCents * int64(it.Quantity)
		tax := summaryTaxCents(net, rule.RateBP) // DEFECT-2: disagrees with Build

		sum.NetCents += net
		sum.TaxCents += tax
		sum.GrossCents += net + tax
	}
	return sum, nil
}
