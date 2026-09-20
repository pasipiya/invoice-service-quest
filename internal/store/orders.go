package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// ErrNotFound is returned when a requested row does not exist.
var ErrNotFound = errors.New("not found")

// Order is a customer order header.
type Order struct {
	ID           int64
	CustomerName string
	Region       string
}

// LineItem is one line of an order. Region is the ship-to region for this
// line and may differ from the order's region, which is why tax is resolved
// per line rather than once per order.
type LineItem struct {
	ID        int64
	OrderID   int64
	ProductID int64
	Quantity  int
	Region    string
}

// GetOrder loads a single order header.
func (s *Store) GetOrder(ctx context.Context, id int64) (Order, error) {
	const q = `SELECT id, customer_name, region FROM orders WHERE id = $1`

	var o Order
	err := s.pool.QueryRow(ctx, q, id).Scan(&o.ID, &o.CustomerName, &o.Region)
	if errors.Is(err, pgx.ErrNoRows) {
		return Order{}, fmt.Errorf("order %d: %w", id, ErrNotFound)
	}
	if err != nil {
		return Order{}, fmt.Errorf("get order %d: %w", id, err)
	}
	return o, nil
}

// ListLineItems loads every line of an order in a stable order.
func (s *Store) ListLineItems(ctx context.Context, orderID int64) ([]LineItem, error) {
	const q = `SELECT id, order_id, product_id, quantity, region
	           FROM line_items WHERE order_id = $1 ORDER BY id`

	rows, err := s.pool.Query(ctx, q, orderID)
	if err != nil {
		return nil, fmt.Errorf("list line items for order %d: %w", orderID, err)
	}
	defer rows.Close()

	var items []LineItem
	for rows.Next() {
		var li LineItem
		if err := rows.Scan(&li.ID, &li.OrderID, &li.ProductID, &li.Quantity, &li.Region); err != nil {
			return nil, fmt.Errorf("scan line item: %w", err)
		}
		items = append(items, li)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate line items: %w", err)
	}
	return items, nil
}
