package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// TaxRule is the tax rate applying to a region, in basis points.
type TaxRule struct {
	Region string
	RateBP int
}

// maxTaxRuleAttempts bounds the retry loop below.
const maxTaxRuleAttempts = 5

// GetTaxRuleForRegion loads the tax rule for a region.
//
// DEFECT-1 (deliberate): the invoice flow calls this once per line item, the
// second of the two N+1 patterns in that flow.
//
// DEFECT-3 (deliberate, NON-GOAL — not fixed in this Quest): the retry loop
// below is unguarded. It retries every error, including ErrNotFound and a
// cancelled context, neither of which will ever succeed on a second attempt;
// and it retries immediately with no backoff, so a struggling database is hit
// five times as hard at precisely the wrong moment. Left in place on purpose
// to demonstrate scope discipline. See docs/defects.md.
func (s *Store) GetTaxRuleForRegion(ctx context.Context, region string) (TaxRule, error) {
	const q = `SELECT region, rate_bp FROM tax_rules WHERE region = $1`

	var lastErr error
	for attempt := 0; attempt < maxTaxRuleAttempts; attempt++ {
		var tr TaxRule
		err := s.pool.QueryRow(ctx, q, region).Scan(&tr.Region, &tr.RateBP)
		if err == nil {
			return tr, nil
		}
		if errors.Is(err, pgx.ErrNoRows) {
			lastErr = fmt.Errorf("tax rule %q: %w", region, ErrNotFound)
			continue // DEFECT-3: a missing row will never appear on a retry.
		}
		lastErr = err
		// DEFECT-3: no backoff, no jitter, no check of ctx.Err().
	}
	return TaxRule{}, fmt.Errorf("get tax rule %q after %d attempts: %w",
		region, maxTaxRuleAttempts, lastErr)
}
