// Package legacy preserves the invoice implementation exactly as it stood at
// tag `baseline`, before DEFECT-1 was fixed.
//
// It is deliberately retained and deliberately NOT wired into the HTTP route.
// Two things need it:
//
//   - the equivalence test, which asserts that the batched implementation
//     returns byte-identical invoices to this one across the whole dataset;
//   - the benchmark, so that `make verify` can report before and after from a
//     single checkout rather than requiring the reviewer to check out two tags.
//
// It holds no copy of the tax rule: it calls invoice.TaxCents, so the rule
// lives in exactly one place. Duplicating it here would reproduce DEFECT-2.
//
// See docs/decision-record.md, ADR-001.
package legacy

import (
	"context"
	"fmt"

	"github.com/pasipiya/invoice-service-quest/internal/invoice"
	"github.com/pasipiya/invoice-service-quest/internal/store"
)

// Repository is the persistence surface the baseline implementation used:
// single-row lookups only.
type Repository interface {
	GetOrder(ctx context.Context, id int64) (store.Order, error)
	ListLineItems(ctx context.Context, orderID int64) ([]store.LineItem, error)
	GetProductByID(ctx context.Context, id int64) (store.Product, error)
	GetTaxRuleForRegion(ctx context.Context, region string) (store.TaxRule, error)
}

// Build is the pre-fix invoice flow: two statements plus two more per line
// item, so 2+2N in total. Preserved verbatim in behaviour.
func Build(ctx context.Context, repo Repository, orderID int64) (invoice.Invoice, error) {
	order, err := repo.GetOrder(ctx, orderID)
	if err != nil {
		return invoice.Invoice{}, err
	}

	items, err := repo.ListLineItems(ctx, orderID)
	if err != nil {
		return invoice.Invoice{}, err
	}

	inv := invoice.Invoice{
		OrderID:      order.ID,
		CustomerName: order.CustomerName,
		Lines:        make([]invoice.Line, 0, len(items)),
	}

	for _, it := range items {
		product, err := repo.GetProductByID(ctx, it.ProductID) // one per line
		if err != nil {
			return invoice.Invoice{}, fmt.Errorf("line %d: %w", it.ID, err)
		}

		rule, err := repo.GetTaxRuleForRegion(ctx, it.Region) // one per line
		if err != nil {
			return invoice.Invoice{}, fmt.Errorf("line %d: %w", it.ID, err)
		}

		net := product.UnitPriceCents * int64(it.Quantity)
		tax := invoice.TaxCents(net, rule.RateBP)

		inv.Lines = append(inv.Lines, invoice.Line{
			ProductID:      product.ID,
			SKU:            product.SKU,
			Name:           product.Name,
			Quantity:       it.Quantity,
			UnitPriceCents: product.UnitPriceCents,
			Region:         it.Region,
			TaxRateBP:      rule.RateBP,
			NetCents:       net,
			TaxCents:       tax,
			GrossCents:     net + tax,
		})

		inv.NetCents += net
		inv.TaxCents += tax
		inv.GrossCents += net + tax
	}

	return inv, nil
}
