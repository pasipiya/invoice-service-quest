// Command seed loads deterministic synthetic data.
//
// The data is generated from a fixed PRNG seed, so every reviewer who runs this
// gets byte-identical rows and therefore comparable measurements. No real
// customer, product or pricing data is used anywhere.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"os"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/pasipiya/invoice-service-quest/internal/store"
)

const (
	// prngSeed is fixed so the dataset is reproducible.
	prngSeed = 20260920

	numProducts = 5000
	numOrders   = 1000

	// bulkOrderLines is the size of order 1, the order the benchmarks and the
	// demonstration use. It is the case where the N+1 hurts most.
	bulkOrderLines = 200

	// maxOrdinaryLines bounds the other, everyday orders.
	maxOrdinaryLines = 5
)

// regions and their tax rates in basis points.
var regions = []struct {
	name   string
	rateBP int
}{
	{"uk-gb", 2000},
	{"ie", 2300},
	{"de", 1900},
	{"fr", 2000},
	{"es", 2100},
	{"nl", 2100},
	{"se", 2500},
	{"pl", 2300},
}

func main() {
	if err := run(); err != nil {
		slog.Error("seed failed", "err", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5433/invoices?sslmode=disable"
	}

	st, err := store.Open(ctx, dsn)
	if err != nil {
		return err
	}
	defer st.Close()

	pool := st.Pool()

	// Idempotent: a re-run reproduces exactly the same dataset.
	if _, err := pool.Exec(ctx,
		`TRUNCATE line_items, orders, products, tax_rules RESTART IDENTITY CASCADE`); err != nil {
		return fmt.Errorf("truncate: %w", err)
	}

	rng := rand.New(rand.NewPCG(prngSeed, prngSeed))

	// Tax rules.
	taxRows := make([][]any, 0, len(regions))
	for _, r := range regions {
		taxRows = append(taxRows, []any{r.name, r.rateBP})
	}
	if err := copyIn(ctx, pool, "tax_rules", []string{"region", "rate_bp"}, taxRows); err != nil {
		return err
	}

	// Products.
	productRows := make([][]any, 0, numProducts)
	for i := 1; i <= numProducts; i++ {
		productRows = append(productRows, []any{
			fmt.Sprintf("SKU-%05d", i),
			fmt.Sprintf("Synthetic product %d", i),
			int64(500 + rng.IntN(49500)), // 5.00 to 500.00
		})
	}
	if err := copyIn(ctx, pool, "products",
		[]string{"sku", "name", "unit_price_cents"}, productRows); err != nil {
		return err
	}

	// Orders. Order 1 is the bulk order used by the benchmarks.
	orderRows := make([][]any, 0, numOrders)
	orderRows = append(orderRows, []any{"Bulk Wholesale Ltd", regions[0].name})
	for i := 2; i <= numOrders; i++ {
		orderRows = append(orderRows, []any{
			fmt.Sprintf("Synthetic customer %d", i),
			regions[rng.IntN(len(regions))].name,
		})
	}
	if err := copyIn(ctx, pool, "orders",
		[]string{"customer_name", "region"}, orderRows); err != nil {
		return err
	}

	// Line items.
	var lineRows [][]any
	appendLines := func(orderID int64, n int) {
		for j := 0; j < n; j++ {
			lineRows = append(lineRows, []any{
				orderID,
				int64(1 + rng.IntN(numProducts)),
				1 + rng.IntN(20),
				regions[rng.IntN(len(regions))].name,
			})
		}
	}
	appendLines(1, bulkOrderLines)
	for id := int64(2); id <= numOrders; id++ {
		appendLines(id, 1+rng.IntN(maxOrdinaryLines))
	}
	if err := copyIn(ctx, pool, "line_items",
		[]string{"order_id", "product_id", "quantity", "region"}, lineRows); err != nil {
		return err
	}

	slog.Info("seeded",
		"tax_rules", len(taxRows),
		"products", len(productRows),
		"orders", len(orderRows),
		"line_items", len(lineRows),
		"bulk_order_id", 1,
		"bulk_order_lines", bulkOrderLines,
	)
	return nil
}

func copyIn(ctx context.Context, pool interface {
	CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error)
}, table string, cols []string, rows [][]any) error {
	n, err := pool.CopyFrom(ctx, pgx.Identifier{table}, cols, pgx.CopyFromRows(rows))
	if err != nil {
		return fmt.Errorf("copy into %s: %w", table, err)
	}
	slog.Info("loaded", "table", table, "rows", n)
	return nil
}
