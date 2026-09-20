package invoice_test

import (
	"context"
	"testing"

	"github.com/pasipiya/invoice-service-quest/internal/invoice"
	"github.com/pasipiya/invoice-service-quest/internal/invoice/legacy"
	"github.com/pasipiya/invoice-service-quest/internal/store"
)

// The benchmarks run both implementations against the same database so that
// `benchstat baseline.txt improved.txt` can report the delta with a p-value.
// Order 1 has 200 lines, order 7 has one.

func BenchmarkBuild_Baseline_200Lines(b *testing.B) { benchLegacy(b, 1) }
func BenchmarkBuild_Batched_200Lines(b *testing.B)  { benchBatched(b, 1) }
func BenchmarkBuild_Baseline_1Line(b *testing.B)    { benchLegacy(b, 7) }
func BenchmarkBuild_Batched_1Line(b *testing.B)     { benchBatched(b, 7) }

func benchLegacy(b *testing.B, orderID int64) {
	st := requireDBBench(b)
	ctx := context.Background()
	b.ReportAllocs()
	for b.Loop() {
		if _, err := legacy.Build(ctx, st, orderID); err != nil {
			b.Fatal(err)
		}
	}
}

func benchBatched(b *testing.B, orderID int64) {
	st := requireDBBench(b)
	svc := invoice.New(st)
	ctx := context.Background()
	b.ReportAllocs()
	for b.Loop() {
		if _, err := svc.Build(ctx, orderID); err != nil {
			b.Fatal(err)
		}
	}
}

func requireDBBench(b *testing.B) *store.Store {
	b.Helper()
	if db == nil {
		b.Skip("no database reachable; run `make up && make seed`")
	}
	return db
}
