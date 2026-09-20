package invoice_test

import (
	"context"
	"fmt"

	"github.com/pasipiya/invoice-service-quest/internal/store"
)

// fakeRepo is an in-memory Repository for cases that are awkward to arrange in
// the database — an order with no lines, a line pointing at a product that does
// not exist. It also records the arguments the batched lookups were called
// with, which is how the deduplication test observes behaviour rather than
// guessing at it.
type fakeRepo struct {
	order    store.Order
	items    []store.LineItem
	products map[int64]store.Product
	rules    map[string]store.TaxRule

	gotProductIDs []int64
	gotRegions    []string
	batchCalls    int
}

func (f *fakeRepo) GetOrder(context.Context, int64) (store.Order, error) {
	return f.order, nil
}

func (f *fakeRepo) ListLineItems(context.Context, int64) ([]store.LineItem, error) {
	return f.items, nil
}

func (f *fakeRepo) GetProductsByIDs(_ context.Context, ids []int64) (map[int64]store.Product, error) {
	f.gotProductIDs = append([]int64(nil), ids...)
	f.batchCalls++
	out := make(map[int64]store.Product, len(ids))
	for _, id := range ids {
		if p, ok := f.products[id]; ok {
			out[id] = p
		}
	}
	return out, nil
}

func (f *fakeRepo) GetTaxRulesForRegions(_ context.Context, regions []string) (map[string]store.TaxRule, error) {
	f.gotRegions = append([]string(nil), regions...)
	f.batchCalls++
	out := make(map[string]store.TaxRule, len(regions))
	for _, r := range regions {
		if tr, ok := f.rules[r]; ok {
			out[r] = tr
		}
	}
	return out, nil
}

func (f *fakeRepo) GetProductByID(_ context.Context, id int64) (store.Product, error) {
	p, ok := f.products[id]
	if !ok {
		return store.Product{}, fmt.Errorf("product %d: %w", id, store.ErrNotFound)
	}
	return p, nil
}

func (f *fakeRepo) GetTaxRuleForRegion(_ context.Context, region string) (store.TaxRule, error) {
	tr, ok := f.rules[region]
	if !ok {
		return store.TaxRule{}, fmt.Errorf("tax rule %q: %w", region, store.ErrNotFound)
	}
	return tr, nil
}
