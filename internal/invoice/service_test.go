package invoice_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/pasipiya/invoice-service-quest/internal/invoice"
	"github.com/pasipiya/invoice-service-quest/internal/invoice/legacy"
	"github.com/pasipiya/invoice-service-quest/internal/store"
)

// TestEquivalenceWithBaseline is the safety net for the whole change: the
// batched implementation must return exactly what the preserved baseline
// returns, for every order in the dataset. Fewer queries is worthless if the
// answer changes.
func TestEquivalenceWithBaseline(t *testing.T) {
	st := requireDB(t)
	svc := invoice.New(st)
	ctx := context.Background()

	const totalOrders = 1000
	for id := int64(1); id <= totalOrders; id++ {
		want, err := legacy.Build(ctx, st, id)
		if err != nil {
			t.Fatalf("baseline build order %d: %v", id, err)
		}
		got, err := svc.Build(ctx, id)
		if err != nil {
			t.Fatalf("build order %d: %v", id, err)
		}
		if !reflect.DeepEqual(want, got) {
			t.Fatalf("order %d: batched invoice differs from baseline\n want %+v\n  got %+v", id, want, got)
		}
	}
	t.Logf("verified %d orders identical to baseline", totalOrders)
}

// TestPerLineTaxRegionIsPreserved guards the rule that made this flow shaped
// the way it is: a line's ship-to region decides its tax, and that region may
// differ from the order's. Resolving tax once per order would be faster still
// and would be wrong.
func TestPerLineTaxRegionIsPreserved(t *testing.T) {
	st := requireDB(t)
	svc := invoice.New(st)

	inv, err := svc.Build(context.Background(), 1)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	rates := make(map[string]int)
	for _, l := range inv.Lines {
		if prev, seen := rates[l.Region]; seen && prev != l.TaxRateBP {
			t.Fatalf("region %q resolved to two different rates: %d and %d", l.Region, prev, l.TaxRateBP)
		}
		rates[l.Region] = l.TaxRateBP
	}
	if len(rates) < 2 {
		t.Fatalf("order 1 should span several regions, found %d", len(rates))
	}
	t.Logf("order 1 spans %d distinct tax regions, each consistently rated", len(rates))
}

func TestEmptyOrderReturnsEmptyInvoice(t *testing.T) {
	repo := &fakeRepo{
		order:    store.Order{ID: 42, CustomerName: "Nobody", Region: "uk-gb"},
		items:    nil,
		products: map[int64]store.Product{},
		rules:    map[string]store.TaxRule{},
	}

	inv, err := invoice.New(repo).Build(context.Background(), 42)
	if err != nil {
		t.Fatalf("empty order should not error: %v", err)
	}
	if len(inv.Lines) != 0 || inv.GrossCents != 0 {
		t.Errorf("want empty invoice, got %d lines totalling %d", len(inv.Lines), inv.GrossCents)
	}
	if inv.CustomerName != "Nobody" {
		t.Errorf("customer name lost: %q", inv.CustomerName)
	}
}

func TestMissingProductNamesTheLine(t *testing.T) {
	repo := &fakeRepo{
		order:    store.Order{ID: 1, CustomerName: "X", Region: "uk-gb"},
		items:    []store.LineItem{{ID: 77, OrderID: 1, ProductID: 999, Quantity: 1, Region: "uk-gb"}},
		products: map[int64]store.Product{}, // 999 absent
		rules:    map[string]store.TaxRule{"uk-gb": {Region: "uk-gb", RateBP: 2000}},
	}

	_, err := invoice.New(repo).Build(context.Background(), 1)
	if err == nil {
		t.Fatal("want an error when a line references a missing product")
	}
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("error should wrap store.ErrNotFound, got %v", err)
	}
	if !strings.Contains(err.Error(), "line 77") {
		t.Errorf("error must name the offending line, got %q", err)
	}
}

func TestMissingTaxRuleNamesTheLine(t *testing.T) {
	repo := &fakeRepo{
		order:    store.Order{ID: 1, CustomerName: "X", Region: "uk-gb"},
		items:    []store.LineItem{{ID: 88, OrderID: 1, ProductID: 5, Quantity: 1, Region: "atlantis"}},
		products: map[int64]store.Product{5: {ID: 5, SKU: "S", Name: "N", UnitPriceCents: 100}},
		rules:    map[string]store.TaxRule{}, // atlantis absent
	}

	_, err := invoice.New(repo).Build(context.Background(), 1)
	if err == nil {
		t.Fatal("want an error when a line references an unknown region")
	}
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("error should wrap store.ErrNotFound, got %v", err)
	}
	if !strings.Contains(err.Error(), "line 88") {
		t.Errorf("error must name the offending line, got %q", err)
	}
}

// TestRepeatedProductAndRegionAreRequestedOnce is where a careless batched
// implementation breaks: an order with the same product on many lines must ask
// for it once, and must still price every line.
func TestRepeatedProductAndRegionAreRequestedOnce(t *testing.T) {
	const lines = 50
	items := make([]store.LineItem, 0, lines)
	for i := range lines {
		items = append(items, store.LineItem{
			ID: int64(i + 1), OrderID: 1, ProductID: 5, Quantity: 2, Region: "uk-gb",
		})
	}

	repo := &fakeRepo{
		order:    store.Order{ID: 1, CustomerName: "Repeat", Region: "uk-gb"},
		items:    items,
		products: map[int64]store.Product{5: {ID: 5, SKU: "S", Name: "N", UnitPriceCents: 100}},
		rules:    map[string]store.TaxRule{"uk-gb": {Region: "uk-gb", RateBP: 2000}},
	}

	inv, err := invoice.New(repo).Build(context.Background(), 1)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	if len(repo.gotProductIDs) != 1 {
		t.Errorf("product asked for %d times, want 1: %v", len(repo.gotProductIDs), repo.gotProductIDs)
	}
	if len(repo.gotRegions) != 1 {
		t.Errorf("region asked for %d times, want 1: %v", len(repo.gotRegions), repo.gotRegions)
	}
	if repo.batchCalls != 2 {
		t.Errorf("made %d batch calls, want 2", repo.batchCalls)
	}
	if len(inv.Lines) != lines {
		t.Fatalf("priced %d lines, want %d", len(inv.Lines), lines)
	}
	// 50 lines x 2 units x 100c = 10000c net, 20% tax = 2000c.
	if inv.NetCents != 10000 || inv.TaxCents != 2000 || inv.GrossCents != 12000 {
		t.Errorf("totals wrong: net=%d tax=%d gross=%d", inv.NetCents, inv.TaxCents, inv.GrossCents)
	}
}

func TestTaxCentsRoundsHalfUp(t *testing.T) {
	cases := []struct {
		net  int64
		rate int
		want int64
	}{
		{100, 2000, 20},
		{0, 2000, 0},
		{1, 2000, 0},    // 0.2c rounds down
		{25, 2000, 5},   // exactly 5c
		{3, 5000, 2},    // 1.5c rounds half up to 2
		{333, 1900, 63}, // 63.27c rounds down
	}
	for _, c := range cases {
		if got := invoice.TaxCents(c.net, c.rate); got != c.want {
			t.Errorf("TaxCents(%d, %d) = %d, want %d", c.net, c.rate, got, c.want)
		}
	}
}
