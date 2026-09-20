package invoice

import (
	"context"
	"fmt"

	"github.com/pasipiya/invoice-service-quest/internal/store"
)

// Repository is the persistence surface the invoice flow depends on.
type Repository interface {
	GetOrder(ctx context.Context, id int64) (store.Order, error)
	ListLineItems(ctx context.Context, orderID int64) ([]store.LineItem, error)
	GetProductByID(ctx context.Context, id int64) (store.Product, error)
	GetTaxRuleForRegion(ctx context.Context, region string) (store.TaxRule, error)
}

// Service builds invoices.
type Service struct{ repo Repository }

// New returns a Service backed by repo.
func New(repo Repository) *Service { return &Service{repo: repo} }

// Build assembles the invoice for one order.
//
// DEFECT-1 (deliberate, see docs/defects.md) — THIS IS THE DEFECT THE QUEST
// FIXES. The loop below issues two queries per line item: one for the product
// and one for the tax rule. Total statements for the request are therefore
// 2 + 2N, where N is the number of line items. For the 200-line bulk order in
// the seed data that is 402 statements to render one page, and it grows
// linearly with order size, so the worst customers get the worst experience.
func (s *Service) Build(ctx context.Context, orderID int64) (Invoice, error) {
	order, err := s.repo.GetOrder(ctx, orderID) // statement 1
	if err != nil {
		return Invoice{}, err
	}

	items, err := s.repo.ListLineItems(ctx, orderID) // statement 2
	if err != nil {
		return Invoice{}, err
	}

	inv := Invoice{
		OrderID:      order.ID,
		CustomerName: order.CustomerName,
		Lines:        make([]Line, 0, len(items)),
	}

	for _, it := range items {
		product, err := s.repo.GetProductByID(ctx, it.ProductID) // DEFECT-1: +1 per line
		if err != nil {
			return Invoice{}, fmt.Errorf("line %d: %w", it.ID, err)
		}

		rule, err := s.repo.GetTaxRuleForRegion(ctx, it.Region) // DEFECT-1: +1 per line
		if err != nil {
			return Invoice{}, fmt.Errorf("line %d: %w", it.ID, err)
		}

		net := product.UnitPriceCents * int64(it.Quantity)
		tax := taxCents(net, rule.RateBP)

		inv.Lines = append(inv.Lines, Line{
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
