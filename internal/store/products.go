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

// GetProductsByIDs loads many products in one statement, keyed by id.
//
// This is the batched counterpart to GetProductByID and the fix for DEFECT-1.
// Results are returned as a map rather than a slice on purpose: the database
// makes no promise about the order of rows returned by ANY($1), so callers must
// not index results positionally against the ids they asked for.
//
// Ids not present in the database are simply absent from the map; the caller
// decides whether that is an error, because only the caller knows which line
// item the id came from.
func (s *Store) GetProductsByIDs(ctx context.Context, ids []int64) (map[int64]Product, error) {
	out := make(map[int64]Product, len(ids))
	if len(ids) == 0 {
		return out, nil
	}

	const q = `SELECT id, sku, name, unit_price_cents FROM products WHERE id = ANY($1)`

	rows, err := s.pool.Query(ctx, q, ids)
	if err != nil {
		return nil, fmt.Errorf("get products by ids: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var p Product
		if err := rows.Scan(&p.ID, &p.SKU, &p.Name, &p.UnitPriceCents); err != nil {
			return nil, fmt.Errorf("scan product: %w", err)
		}
		out[p.ID] = p
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate products: %w", err)
	}
	return out, nil
}
