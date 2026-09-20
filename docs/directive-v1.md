# directive.md — v1 (initial)

**Status:** initial directive, written before any fix exists.
**Written:** 2026-09-20, against tag `baseline`.
**Supersedes:** nothing. **Superseded by:** `directive.md` (final).

> This version is preserved deliberately. It is the instruction set as it stood
> *before* the work, so that the final directive can be compared against it and
> the difference can be attributed to what was learned. Nothing in this file has
> been edited after the fix was implemented.

Working instructions for an AI coding agent, derived from
[`intent.md`](intent.md). Read alongside [`quality-yardstick.md`](quality-yardstick.md).

---

## 1. Objective

Remove the N+1 query pattern (DEFECT-1) from the invoice flow, so that the
number of SQL statements per request is **constant** rather than linear in the
number of line items — without altering the invoice output in any way.

---

## 2. Context the agent needs

- **Flow:** `GET /orders/{id}/invoice`, served by `Service.Build` in
  `internal/invoice/service.go`.
- **Current cost:** `2 + 2N` statements. Measured: 402 for the 200-line order 1;
  8.3 on average across the dataset.
- **Why it is shaped this way:** tax is resolved per line, because a line's
  ship-to `region` may differ from the order's region. Any fix must preserve
  this. Resolving tax once per order would be wrong.
- **Measurement:** `internal/dbtrace` attaches a `pgx.QueryTracer` to the pool
  and counts statements per context. Every response carries `X-Query-Count`.
- **Money:** integer cents throughout, rounding half up via `taxCents`.
- **Data:** synthetic, fixed PRNG seed, reload with `make reset`.

---

## 3. Scope

### In scope

- `internal/invoice/service.go` — the flow itself.
- `internal/store/products.go`, `internal/store/taxrules.go` — adding **batched**
  lookups alongside the existing single-row ones.
- New tests in `internal/invoice/`.
- Benchmarks and the verification script.

### Out of scope — do not modify

| Out of scope | Reason |
|---|---|
| `internal/invoice/summary.go` and `taxCents` / `summaryTaxCents` | DEFECT-2 is a declared non-goal |
| The retry loop in `GetTaxRuleForRegion` | DEFECT-3 is a declared non-goal |
| `migrations/` — schema and indexes | The fix must come from the access pattern, not from hiding it behind an index |
| `internal/httpapi/` | The HTTP contract does not change |
| `go.mod` — no new dependencies | A one-flow fix does not need a library |
| Caching of any kind | Improves the symptom, leaves the access pattern and its staleness risks in place |

### Boundaries

- One defect. Finding another means **recording it in `docs/defects.md`**, not
  fixing it.
- No change to the JSON response shape, field names, or ordering of lines.
- No concurrency introduced to make the existing query pattern faster. Fewer
  queries, not parallel ones. *(See §7.)*

---

## 4. Requirements

1. `Service.Build` issues a **constant** number of statements, independent of
   line count. Target: 3.
2. Per-line tax resolution is preserved — a line's region determines its rate.
3. Invoice output is **byte-identical** to the baseline for every order in the
   dataset.
4. A line referencing a missing product or an unknown region still produces an
   error naming the offending line, as it does today.
5. An order with zero line items still returns a valid empty invoice.
6. Batched store methods return results that the caller can index by id; the
   caller must not assume the database returned rows in the order requested.

---

## 5. Tests the agent must write

| Test | Asserts |
|---|---|
| `querycount_test.go` | Statement count for a 200-line order is ≤ 3, and **equal** to the count for a 1-line order — this is the guard for yardstick §2 |
| Equivalence test | Baseline and new implementations produce identical `Invoice` values for a sample of orders spanning the size distribution |
| Empty-order test | Zero line items returns an empty invoice, not an error |
| Missing-product test | Error names the offending line |
| Duplicate-product test | An order with the same product on several lines is priced correctly — this is where a naive batched lookup breaks |
| `bench_test.go` | Both implementations, benchmarked for `benchstat` comparison |

Tests must pass under `go test -race ./...`.

---

## 6. Acceptance criteria

The change is complete when **all** of these hold:

- [ ] `make test` passes, race detector clean.
- [ ] `make lint` passes — `gofmt` clean, `go vet` clean.
- [ ] `X-Query-Count` is 3 for order 1, order 2 and order 7 alike.
- [ ] The equivalence test passes across the order-size distribution.
- [ ] Reverting the fix makes the guard test **fail** — verified by actually
      doing it, not by assuming.
- [ ] `make verify` emits before-and-after numbers to `results/`.
- [ ] `benchstat` reports the latency delta with a p-value over n=10 runs.
- [ ] The diff touches no file listed as out of scope.

---

## 7. Review responsibilities

### The agent decides

Implementation shape within the constraints above: SQL formulation, how results
are keyed in memory, naming, test structure.

### The human decides — non-delegable

- Whether the scope boundary held. An agent that fixes DEFECT-2 "while it was in
  there" has failed, regardless of code quality.
- Whether a proposed fix addresses the **cause** or the **symptom**. Anything
  that leaves `2 + 2N` queries in place while making them faster is rejected.
- Whether measurements are honest: measured versus estimated, and whether the
  claim matches what was actually run.
- Whether the guard test would actually fail on regression.

### Expected failure modes, to be watched for explicitly

1. **Concurrency instead of batching.** An agent asked to make this faster is
   likely to reach for `errgroup` and run the N queries in parallel. Latency
   improves; the query count does not. Load on the database gets *worse* —
   200 concurrent queries per request, pool exhaustion under real traffic. This
   must be **rejected**, and the rejection recorded in
   `review/ai-correction-01.md` with the risk stated.
2. **Caching.** Improves the second request and hides the first.
3. **Index-adding.** Makes each of the 402 queries faster; there are still 402.
4. **Scope drift into DEFECT-2**, since the tax code is adjacent.
5. **Ordering assumptions** — `WHERE id = ANY($1)` does not guarantee row order.

---

## 8. Completion criteria for the Quest

- [ ] Fix merged via PR, tagged `improved`.
- [ ] `results/` contains measured before and after figures.
- [ ] ADR-001 records alternatives and trade-offs.
- [ ] `review/ai-correction-01.md` records a rejected AI proposal and its risk.
- [ ] `docs/handoff.md` lets another engineer change the flow unaided.
- [ ] Final `directive.md` carries the results and handoff appendix.
- [ ] Loom recorded, under five minutes.

---

## 9. What this directive does not authorise

Deploying anything, touching data that is not synthetic, adding dependencies,
changing the HTTP contract, or making any claim of impact beyond a local
single-machine measurement.
