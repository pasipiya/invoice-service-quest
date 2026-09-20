package invoice

import (
	"context"
	"fmt"

	"github.com/pasipiya/invoice-service-quest/internal/store"
)

// Repository is the persistence surface the invoice flow depends on.
//
// Both the batched and the single-row lookups are declared. Build uses the
// batched pair; Summarise still uses the single-row pair, because
// internal/invoice/summary.go is a declared non-goal of this change and was
// left untouched. See docs/defects.md.
type Repository interface {
	GetOrder(ctx context.Context, id int64) (store.Order, error)
	ListLineItems(ctx context.Context, orderID int64) ([]store.LineItem, error)

	// Batched — used by Build.
	GetProductsByIDs(ctx context.Context, ids []int64) (map[int64]store.Product, error)
	GetTaxRulesForRegions(ctx context.Context, regions []string) (map[string]store.TaxRule, error)

	// Single-row — used by Summarise, and by the preserved baseline
	// implementation in the legacy subpackage.
	GetProductByID(ctx context.Context, id int64) (store.Product, error)
	GetTaxRuleForRegion(ctx context.Context, region string) (store.TaxRule, error)
}

// Service builds invoices.
type Service struct{ repo Repository }

// New returns a Service backed by repo.
func New(repo Repository) *Service { return &Service{repo: repo} }

// Build assembles the invoice for one order.
//
// The statement cost is constant in the number of line items — four
// statements: the order, its lines, every distinct product, every distinct tax
// region. It was previously 2+2N; see docs/decision-record.md for why four
// rather than the three the initial directive asked for.
//
// Tax is still resolved per line from that line's own ship-to region, which may
// differ from the order's region. Batching changes how the rules are fetched,
// not which rule applies to which line.
func (s *Service) Build(ctx context.Context, orderID int64) (Invoice, error) {
	order, err := s.repo.GetOrder(ctx, orderID) // statement 1
	if err != nil {
		return Invoice{}, err
	}

	items, err := s.repo.ListLineItems(ctx, orderID) // statement 2
	if err != nil {
		return Invoice{}, err
	}

	products, rules, err := s.load(ctx, items) // statements 3 and 4
	if err != nil {
		return Invoice{}, err
	}

	inv := Invoice{
		OrderID:      order.ID,
		CustomerName: order.CustomerName,
		Lines:        make([]Line, 0, len(items)),
	}

	for _, it := range items {
		product, ok := products[it.ProductID]
		if !ok {
			return Invoice{}, fmt.Errorf("line %d: product %d: %w",
				it.ID, it.ProductID, store.ErrNotFound)
		}
		rule, ok := rules[it.Region]
		if !ok {
			return Invoice{}, fmt.Errorf("line %d: tax rule %q: %w",
				it.ID, it.Region, store.ErrNotFound)
		}

		net := product.UnitPriceCents * int64(it.Quantity)
		tax := TaxCents(net, rule.RateBP)

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

// load fetches every product and tax rule the lines refer to, in two
// statements. Ids and regions are deduplicated first, so an order with the same
// product on fifty lines still asks for it once.
func (s *Service) load(ctx context.Context, items []store.LineItem) (
	map[int64]store.Product, map[string]store.TaxRule, error,
) {
	productIDs := make([]int64, 0, len(items))
	seenProduct := make(map[int64]struct{}, len(items))
	regions := make([]string, 0, len(items))
	seenRegion := make(map[string]struct{}, len(items))

	for _, it := range items {
		if _, ok := seenProduct[it.ProductID]; !ok {
			seenProduct[it.ProductID] = struct{}{}
			productIDs = append(productIDs, it.ProductID)
		}
		if _, ok := seenRegion[it.Region]; !ok {
			seenRegion[it.Region] = struct{}{}
			regions = append(regions, it.Region)
		}
	}

	products, err := s.repo.GetProductsByIDs(ctx, productIDs)
	if err != nil {
		return nil, nil, err
	}
	rules, err := s.repo.GetTaxRulesForRegions(ctx, regions)
	if err != nil {
		return nil, nil, err
	}
	return products, rules, nil
}
