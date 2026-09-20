// Package dbtrace counts the SQL statements executed during a unit of work.
//
// The count is the primary quality measure for the invoice flow: unlike
// latency it is a deterministic integer, identical on every machine, so it can
// be asserted in an ordinary test rather than eyeballed on a dashboard.
package dbtrace

import (
	"context"
	"sync/atomic"

	"github.com/jackc/pgx/v5"
)

type ctxKey struct{}

// Counter tallies SQL statements executed under a context it is attached to.
// It is safe for concurrent use.
type Counter struct{ n atomic.Int64 }

// Count returns the number of statements observed so far.
func (c *Counter) Count() int { return int(c.n.Load()) }

// WithCounter derives a context carrying a fresh Counter, and returns both.
// Statements executed with the derived context are tallied by the Counter.
func WithCounter(ctx context.Context) (context.Context, *Counter) {
	c := &Counter{}
	return context.WithValue(ctx, ctxKey{}, c), c
}

// FromContext returns the Counter attached to ctx, or nil when counting is not
// enabled. A nil Counter is the normal case in production paths.
func FromContext(ctx context.Context) *Counter {
	c, _ := ctx.Value(ctxKey{}).(*Counter)
	return c
}

// Tracer implements pgx.QueryTracer. Attach it to a pool's connection config
// and every statement run under a counted context is tallied automatically,
// so call sites need no instrumentation of their own.
type Tracer struct{}

var _ pgx.QueryTracer = Tracer{}

// TraceQueryStart tallies one statement.
func (Tracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	if c := FromContext(ctx); c != nil {
		c.n.Add(1)
	}
	return ctx
}

// TraceQueryEnd is required by pgx.QueryTracer; nothing is recorded on completion.
func (Tracer) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}
