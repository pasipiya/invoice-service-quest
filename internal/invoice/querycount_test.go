package invoice_test

import (
	"context"
	"testing"

	"github.com/pasipiya/invoice-service-quest/internal/dbtrace"
	"github.com/pasipiya/invoice-service-quest/internal/invoice"
	"github.com/pasipiya/invoice-service-quest/internal/invoice/legacy"
)

// maxStatements is the contract: building an invoice costs at most this many
// SQL statements, whatever the order looks like. Four are expected — order,
// line items, products, tax rules — and the ceiling is set at the exact figure
// so that adding a fifth is a deliberate act with a test to update.
const maxStatements = 4

// TestStatementCountIsConstant is the guard for DEFECT-1.
//
// It asserts the property, not the symptom: the cost of building an invoice
// must not depend on how many lines the order has. A 200-line order and a
// 1-line order must cost the same. Reintroducing a per-line query fails this
// test long before anyone notices the latency.
func TestStatementCountIsConstant(t *testing.T) {
	st := requireDB(t)
	svc := invoice.New(st)

	count := func(orderID int64) int {
		t.Helper()
		ctx, counter := dbtrace.WithCounter(context.Background())
		if _, err := svc.Build(ctx, orderID); err != nil {
			t.Fatalf("build order %d: %v", orderID, err)
		}
		return counter.Count()
	}

	const (
		bulkOrder  = 1 // 200 lines
		smallOrder = 7 // 1 line
	)

	bulk, small := count(bulkOrder), count(smallOrder)

	if bulk != small {
		t.Errorf("statement count scales with line count: order %d cost %d, order %d cost %d; "+
			"they must be equal",
			bulkOrder, bulk, smallOrder, small)
	}
	if bulk > maxStatements {
		t.Errorf("order %d cost %d statements, want at most %d", bulkOrder, bulk, maxStatements)
	}

	t.Logf("statements: 200-line order %d, 1-line order %d", bulk, small)
}

// TestLegacyStatementCountScales documents the baseline the guard protects
// against. If this ever stops scaling, the preserved implementation has drifted
// and the before-and-after comparison is no longer meaningful.
func TestLegacyStatementCountScales(t *testing.T) {
	st := requireDB(t)

	count := func(orderID int64) int {
		t.Helper()
		ctx, counter := dbtrace.WithCounter(context.Background())
		if _, err := legacy.Build(ctx, st, orderID); err != nil {
			t.Fatalf("legacy build order %d: %v", orderID, err)
		}
		return counter.Count()
	}

	bulk, small := count(1), count(7)

	if bulk <= small {
		t.Fatalf("baseline should scale with line count, got %d for 200 lines and %d for 1", bulk, small)
	}
	if want := 2 + 2*200; bulk != want {
		t.Errorf("baseline cost %d statements for the 200-line order, want %d (2+2N)", bulk, want)
	}

	t.Logf("baseline statements: 200-line order %d, 1-line order %d", bulk, small)
}
