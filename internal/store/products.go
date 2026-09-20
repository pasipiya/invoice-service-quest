package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// Product is a sellable item.
type Product struct {
	ID             int64
	SKU            string
	Name           string
	UnitPriceCents int64
}

// GetProductByID loads one product.
//
// DEFECT-1 (deliberate, see docs/defects.md): the invoice flow calls this once
// per line item, which is one of the two N+1 patterns in that flow. The method
// itself is fine; the defect is the loop at the call site and the absence of a
// batched alternative to reach for.
func (s *Store) GetProductByID(ctx context.Context, id int64) (Product, error) {
	const q = `SELECT id, sku, name, unit_price_cents FROM products WHERE id = $1`

	var p Product
	err := s.pool.QueryRow(ctx, q, id).Scan(&p.ID, &p.SKU, &p.Name, &p.UnitPriceCents)
	if errors.Is(err, pgx.ErrNoRows) {
		return Product{}, fmt.Errorf("product %d: %w", id, ErrNotFound)
	}
	if err != nil {
		return Product{}, fmt.Errorf("get product %d: %w", id, err)
	}
	return p, nil
}
