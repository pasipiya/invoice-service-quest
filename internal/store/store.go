// Package store is the persistence layer for the invoice flow. Queries are
// hand-written SQL over pgx; there is no ORM, so the number of round trips a
// code path makes is visible at the call site.
package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/pasipiya/invoice-service-quest/internal/dbtrace"
)

// Store owns the connection pool.
type Store struct{ pool *pgxpool.Pool }

// Open parses dsn, attaches the statement-counting tracer and verifies the
// connection. The caller must Close the returned Store.
func Open(ctx context.Context, dsn string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}

	// Counting is wired in at the pool, so every query made through this
	// Store is tallied without each call site having to opt in.
	cfg.ConnConfig.Tracer = dbtrace.Tracer{}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return &Store{pool: pool}, nil
}

// Close releases the pool.
func (s *Store) Close() { s.pool.Close() }

// Pool exposes the underlying pool for the seeder, which bulk-loads with COPY.
func (s *Store) Pool() *pgxpool.Pool { return s.pool }
