# ADR-001 — Batched lookups for the invoice flow

**Status:** accepted · **Date:** 2026-09-20
**Context:** [`intent.md`](intent.md) · **Instructions:** [`directive-v1.md`](directive-v1.md)
**Review that shaped it:** [`../review/ai-correction-01.md`](../review/ai-correction-01.md)

## Context

`Service.Build` issued two queries per line item — one for the product, one for
the tax rule of that line's ship-to region — giving `2 + 2N` statements per
request. Measured: 402 statements and a 57.3 ms median for the 200-line order.

Tax must stay resolved per line. A line's ship-to region can differ from the
order's region, so resolving tax once per order would be faster and wrong.

## Decision

Collect the distinct product IDs and distinct ship-to regions from the line
items, fetch each set in one `WHERE … = ANY($1)` statement, and resolve every
line from the resulting maps. Four statements per request, constant in line
count.

Results are keyed into maps rather than returned as ordered slices, because
PostgreSQL makes no promise about the order of rows returned by `ANY($1)`.
Indexing results positionally against the requested ids would be a latent
corruption bug that tests on a small dataset would not catch.

## Alternatives considered

### A. One JOIN across all four tables — 1 to 2 statements

`orders ⋈ line_items ⋈ products ⋈ tax_rules` in a single query.

Rejected. It would buy two statements over the chosen approach — noise beside
the 402 saved — and would cost real maintainability: the store layer would gain
a query shaped for exactly one caller's needs, and the product row would be
repeated for every line that references it. The batched lookups are generic and
reusable; a bespoke join is not. **The difference between 402 and 4 is the
result. The difference between 4 and 2 is not worth coupling for.**

### B. Concurrent per-line queries — `errgroup` fan-out

Run the N product and tax queries in parallel.

Rejected on principle before measurement. Latency would improve while the
access pattern stayed exactly as broken: still `2 + 2N` statements, now arriving
as 400 concurrent queries from a single request. Under real traffic that
exhausts the connection pool and moves the failure from "one slow page" to
"the database is unavailable for everyone". It optimises the symptom and makes
the cause worse.

Worth recording: [`directive-v1.md`](directive-v1.md) predicted an AI agent
would propose exactly this. It did not — see the review. The prediction was
wrong, and the real risk was subtler.

### C. Caching products and tax rules

Rejected. It improves the second request and leaves the first one unchanged,
while introducing invalidation as a new correctness surface — a stale price or
tax rate is a worse defect than a slow page. It also hides the access pattern
instead of fixing it, so the next person reading `Build` still cannot see what
it costs.

### D. Index tuning

Rejected as a non-answer. Every index the flow needs already exists; the cost is
round-trip overhead, not query execution. Faster individual queries would still
leave 402 of them.

### E. Denormalising or materialising invoice totals

Rejected as disproportionate. It would trade a bounded, local change for a
consistency problem that has to be maintained forever, to solve a problem that
four statements already solve.

### F. Fixing the same N+1 in `Summarise` at the same time

Rejected on scope. `internal/invoice/summary.go` is where DEFECT-2 lives, a
declared non-goal. Changing it would leave two candidate causes for any future
disagreement between the endpoints, and would make the before-and-after
measurement attributable to neither change. An AI agent did exactly this
unprompted; the reasoning for rejecting it is in the review.

## Consequences

### Measured, 100 samples per case after warm-up, single machine

| Order | Statements | p50 latency |
|---|---|---|
| 1 line | 4 → 4 | 0.62 → 0.64 ms |
| 5 lines | 12 → 4 | 1.58 → 0.79 ms |
| 200 lines | **402 → 4** | **57.29 → 1.97 ms** |

`benchstat`, n=10 per implementation:

| | 200 lines | 1 line |
|---|---|---|
| Time | **−97.06%** (p=0.000) | no significant change (p=0.280) |
| Bytes/op | −49.54% (p=0.000) | **+40.88%** (p=0.000) |
| Allocs/op | −62.25% (p=0.000) | **+31.58%** (p=0.000) |

### The cost, stated plainly

**Small orders allocate more.** A one-line invoice now builds two maps it does
not need, costing 12 extra allocations and about 1 KiB. This is a real
regression, it is statistically significant, and it is accepted: the dataset's
mean order is 3.13 lines, the absolute cost is around a kilobyte, and the
alternative — branching to a different code path for short orders — would add
a second implementation to keep correct in exchange for nothing measurable.

If invoice building ever appears in an allocation profile, this is the line to
revisit. It is not there today.

### Other consequences

- The flow's cost is now legible. A reader sees four statements without tracing
  a loop.
- The property is guarded, not merely restored:
  `TestStatementCountIsConstant` asserts a 200-line order costs the same as a
  1-line order. Verified by reverting the fix and confirming the test fails.
- `store` gained two methods and kept the two it had, so callers can choose.
- DEFECT-2 and DEFECT-3 are untouched, exactly as declared.

## Decision: the acceptance criterion was wrong, and was changed

[`directive-v1.md`](directive-v1.md) required **3 statements**. Four is the
honest number for this shape — order, lines, products, tax rules — and reaching
three would mean adopting alternative A for no real gain.

The criterion has been changed to *"constant in the number of line items"*. It
was a poorly chosen target: it specified an implementation detail as though it
were an outcome. The property that matters is that the cost stops scaling, and
the test now asserts that property rather than an integer.

This change is recorded here rather than edited quietly into the directive.

## Decision: the pre-fix implementation is kept in the tree

`internal/invoice/legacy` holds the baseline implementation. It is not wired to
any route. It exists so that `make verify` can report before and after from one
checkout, and so the equivalence test can assert the two produce identical
invoices — which it does, across all 1,000 orders.

The alternative was to rely on `git checkout baseline`, which would force a
reviewer to check out two tags and run each separately, and would give no
automated equivalence check at all. Keeping dead-looking code in the tree is a
real cost, mitigated by the package comment stating why it is there and by the
test that would fail if it drifted.

It holds no copy of the tax rule — it calls `invoice.TaxCents` — so the rule
still lives in exactly one place. Duplicating it there would have reproduced
DEFECT-2 while fixing DEFECT-1.
