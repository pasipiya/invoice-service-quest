// Command verify measures the invoice flow before and after the DEFECT-1 fix
// and writes the results to results/.
//
// Both implementations are exercised in a single run against the same database,
// so the comparison needs no checkout of another tag and no second machine. The
// statement counts it reports are exact; the latencies are local measurements
// and vary by machine.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"time"

	"github.com/pasipiya/invoice-service-quest/internal/dbtrace"
	"github.com/pasipiya/invoice-service-quest/internal/invoice"
	"github.com/pasipiya/invoice-service-quest/internal/invoice/legacy"
	"github.com/pasipiya/invoice-service-quest/internal/store"
)

const (
	warmup  = 10
	samples = 100
)

// cases are the order sizes measured, spanning the dataset's distribution.
var cases = []struct {
	orderID int64
	lines   int
	label   string
}{
	{7, 1, "1 line (smallest)"},
	{2, 5, "5 lines (p95)"},
	{1, 200, "200 lines (largest)"},
}

type measurement struct {
	OrderID    int64   `json:"order_id"`
	Lines      int     `json:"lines"`
	Label      string  `json:"label"`
	Statements int     `json:"statements"`
	P50Ms      float64 `json:"p50_ms"`
	P95Ms      float64 `json:"p95_ms"`
}

type result struct {
	Implementation string        `json:"implementation"`
	Description    string        `json:"description"`
	Measured       string        `json:"measured_at"`
	Machine        string        `json:"machine"`
	Samples        int           `json:"samples_per_case"`
	Note           string        `json:"note"`
	Cases          []measurement `json:"cases"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "verify:", err)
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5433/invoices?sslmode=disable"
	}
	st, err := store.Open(ctx, dsn)
	if err != nil {
		return fmt.Errorf("%w\n\nrun `make up && make seed` first", err)
	}
	defer st.Close()

	svc := invoice.New(st)

	baseline := measure("baseline", "pre-fix: one product and one tax-rule query per line (2+2N)",
		func(ctx context.Context, id int64) error {
			_, err := legacy.Build(ctx, st, id)
			return err
		})

	improved := measure("improved", "batched: order, lines, all products, all tax rules (constant)",
		func(ctx context.Context, id int64) error {
			_, err := svc.Build(ctx, id)
			return err
		})

	if err := write("results/baseline.json", baseline); err != nil {
		return err
	}
	if err := write("results/improved.json", improved); err != nil {
		return err
	}

	report(baseline, improved)
	return nil
}

func measure(name, desc string, build func(context.Context, int64) error) result {
	res := result{
		Implementation: name,
		Description:    desc,
		Measured:       time.Now().Format(time.RFC3339),
		Machine:        fmt.Sprintf("%s/%s, %d cpu", runtime.GOOS, runtime.GOARCH, runtime.NumCPU()),
		Samples:        samples,
		Note: "Statement counts are exact and machine-independent. Latencies are " +
			"local single-machine measurements against synthetic data and will " +
			"differ elsewhere. No production or team-wide claim is made.",
	}

	for _, c := range cases {
		ctx, counter := dbtrace.WithCounter(context.Background())
		if err := build(ctx, c.orderID); err != nil {
			panic(err)
		}

		for range warmup {
			_ = build(context.Background(), c.orderID)
		}

		durations := make([]float64, 0, samples)
		for range samples {
			start := time.Now()
			_ = build(context.Background(), c.orderID)
			durations = append(durations, float64(time.Since(start).Microseconds())/1000)
		}
		sort.Float64s(durations)

		res.Cases = append(res.Cases, measurement{
			OrderID:    c.orderID,
			Lines:      c.lines,
			Label:      c.label,
			Statements: counter.Count(),
			P50Ms:      durations[len(durations)/2],
			P95Ms:      durations[int(float64(len(durations))*0.95)-1],
		})
	}
	return res
}

func write(path string, r result) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

func report(baseline, improved result) {
	fmt.Printf("\nInvoice flow — statements and latency per request\n")
	fmt.Printf("%s\n", baseline.Machine)
	fmt.Printf("%d samples per case, %d warm-up\n\n", samples, warmup)

	fmt.Printf("%-22s %11s %11s   %11s %11s\n", "", "stmts", "stmts", "p50 ms", "p50 ms")
	fmt.Printf("%-22s %11s %11s   %11s %11s\n", "order", "before", "after", "before", "after")
	fmt.Printf("%s\n", "──────────────────────────────────────────────────────────────────────")

	for i := range baseline.Cases {
		b, a := baseline.Cases[i], improved.Cases[i]
		fmt.Printf("%-22s %11d %11d   %11.2f %11.2f\n", b.Label, b.Statements, a.Statements, b.P50Ms, a.P50Ms)
	}

	last := len(baseline.Cases) - 1
	b, a := baseline.Cases[last], improved.Cases[last]
	fmt.Printf("\nLargest order: %d → %d statements (%.0f%% fewer), %.2f → %.2f ms (%.1f× faster)\n",
		b.Statements, a.Statements,
		100*(1-float64(a.Statements)/float64(b.Statements)),
		b.P50Ms, a.P50Ms, b.P50Ms/a.P50Ms)
	fmt.Printf("Written: results/baseline.json, results/improved.json\n\n")
}
