# directive.md — final

**Quest:** Make AI-Assisted Code Easier to Trust and Change
**Author:** Pasindu Piyathilaka
**Repository:** https://github.com/pasipiya/invoice-service-quest
**Frozen at tag:** `submission-v1`

Final working instructions for the change, followed by a results and handoff
appendix. Developed from [intent.md](https://github.com/pasipiya/invoice-service-quest/blob/submission-v1/docs/intent.md).
The initial version, written before any fix existed, is preserved at
[directive-v1.md](https://github.com/pasipiya/invoice-service-quest/blob/submission-v1/docs/directive-v1.md);
what changed between them, and why, is in §11.

---

# Part 1 — The directive

## 1. Objective

Remove the N+1 query pattern from `GET /orders/{id}/invoice` so that the number
of SQL statements per request is **constant in the number of line items**,
without changing the invoice output.

## 2. Context

- **Flow:** `Service.Build` in `internal/invoice/service.go`.
- **Baseline cost:** `2 + 2N` statements. Measured: 402 for the 200-line
  order 1; 8.3 on average across the dataset; median 3.13 lines per order.
- **Why it is shaped this way:** a line's ship-to `region` may differ from the
  order's region, so tax is resolved per line. Any fix must preserve this —
  resolving tax once per order would be faster and wrong.
- **Measurement:** `internal/dbtrace` attaches a `pgx.QueryTracer` to the pool
  and counts statements per context; every response carries `X-Query-Count`.
- **Money:** integer cents, rounding half up. No floats anywhere.
- **Data:** synthetic, fixed PRNG seed. `make reset` reproduces it exactly.

## 3. Scope

**In:** `internal/invoice/service.go`; batched lookups added *alongside* the
existing single-row ones in `internal/store/`; new tests; benchmarks and the
verification harness.

**Out, and not to be modified:**

| Out of scope | Reason |
|---|---|
| `internal/invoice/summary.go` | DEFECT-2 lives here; declared non-goal |
| The retry loop in `internal/store/taxrules.go` | DEFECT-3; declared non-goal |
| `migrations/` | The fix must come from the access pattern, not an index |
| `internal/httpapi/` | The HTTP contract does not change |
| `go.mod` | No new dependencies |
| Caching of any kind | Improves the symptom, leaves the cause and adds staleness risk |

**Boundaries:** one defect only. Finding another means recording it in
`docs/defects.md`, not fixing it. No concurrency introduced to make the existing
query pattern faster — fewer queries, not parallel ones.

## 4. Requirements

1. Statement count is **constant** in line count.
2. Per-line tax resolution preserved.
3. Invoice output byte-identical to the baseline for every order.
4. A line referencing a missing product or unknown region still errors, naming
   the line.
5. An order with no lines returns a valid empty invoice.
6. Batch results keyed by id — never indexed positionally against the request,
   because `ANY($1)` gives no ordering guarantee.

## 5. Tests required

Guard test (count constant across order sizes); equivalence against the
preserved baseline; empty order; missing product; missing tax rule; duplicate
product across many lines; benchmarks for both implementations. All under
`go test -race`.

## 6. Completion criteria

- [x] `make test` passes, race clean
- [x] `make lint` passes
- [x] `X-Query-Count` constant across order sizes
- [x] Equivalence passes across the full dataset
- [x] **Reverting the fix makes the guard test fail — verified by doing it**
- [x] `make verify` writes before-and-after to `results/`
- [x] `benchstat` reports the delta with a p-value over n=10
- [x] Diff touches no out-of-scope file

## 7. Review responsibilities

**The agent decides:** SQL formulation, how results are keyed, naming, test
structure.

**The human decides, non-delegable:** whether scope held; whether the fix
addresses cause or symptom; whether measurements are honest; whether the guard
would actually fail on regression.

---

# Part 2 — Results and handoff appendix

> Everything below was written after the work. All figures are **measured**
> unless labelled otherwise.

## 8. Artifacts

| Artifact | Link |
|---|---|
| Repository | https://github.com/pasipiya/invoice-service-quest/tree/submission-v1 |
| **Focused diff** | https://github.com/pasipiya/invoice-service-quest/compare/baseline...improved |
| Pull request | https://github.com/pasipiya/invoice-service-quest/pull/3 |
| Why this problem | [intent.md](https://github.com/pasipiya/invoice-service-quest/blob/submission-v1/docs/intent.md) |
| Initial directive | [directive-v1.md](https://github.com/pasipiya/invoice-service-quest/blob/submission-v1/docs/directive-v1.md) |
| Quality yardstick | [quality-yardstick.md](https://github.com/pasipiya/invoice-service-quest/blob/submission-v1/docs/quality-yardstick.md) |
| Decision record | [decision-record.md](https://github.com/pasipiya/invoice-service-quest/blob/submission-v1/docs/decision-record.md) |
| **Code review of AI output** | [ai-correction-01.md](https://github.com/pasipiya/invoice-service-quest/blob/submission-v1/review/ai-correction-01.md) |
| Raw AI output, unedited | [01-naive-agent.diff](https://github.com/pasipiya/invoice-service-quest/blob/submission-v1/review/raw/01-naive-agent.diff) |
| The three planted defects | [defects.md](https://github.com/pasipiya/invoice-service-quest/blob/submission-v1/docs/defects.md) |
| Handoff + exercise | [handoff.md](https://github.com/pasipiya/invoice-service-quest/blob/submission-v1/docs/handoff.md) |
| Review checklist | [review-checklist.md](https://github.com/pasipiya/invoice-service-quest/blob/submission-v1/docs/review-checklist.md) |
| Measured results | [results/](https://github.com/pasipiya/invoice-service-quest/tree/submission-v1/results) |

## 9. Reproducing the results

Requirements: Go 1.25+, Docker Compose. Roughly two minutes.

```bash
git clone https://github.com/pasipiya/invoice-service-quest
cd invoice-service-quest
make up && make seed
make verify        # before-and-after table, writes results/
make test          # correctness, equivalence, guard
```

For the statistical comparison (needs
`go install golang.org/x/perf/cmd/benchstat@latest`):

```bash
make compare       # n=10 per implementation, benchstat with p-values
```

To see the fix and the untouched non-goal side by side in one command:

```bash
make run           # in another terminal
for id in 1 2 7; do
  echo "order $id invoice=$(curl -s -D- -o/dev/null localhost:8080/orders/$id/invoice | grep -i x-query-count | awk '{print $2}') \
summary=$(curl -s -D- -o/dev/null localhost:8080/orders/$id/summary | grep -i x-query-count | awk '{print $2}')"
done
```

### Reproduction verified

These steps were run against a **fresh clone** on 2026-09-20 — not the author's
working tree — from a cold database, to confirm the instructions above are
complete rather than assumed:

| | Author's working tree | Fresh clone |
|---|---|---|
| Statements, 200-line order | 402 → 4 | **402 → 4** (identical) |
| p50, 200-line order | 57.29 → 1.97 ms | 58.42 → 2.00 ms |
| `benchstat` time, 200 lines | −97.06% (p=0.000) | **−97.14%** (p=0.000) |
| `benchstat` time, 1 line | ~ (p=0.280) | ~ (p=0.631) |
| Allocation regression, 1 line | +40.88% | **+40.92%** |
| `make lint`, `make test` | pass | pass |

The seeder produced identical row counts in both (3,129 line items), confirming
the fixed PRNG seed makes the dataset reproducible. Statement counts are
identical because they are exact; latencies differ by roughly 2%, which is the
expected variation for a timing measurement and the reason query count is the
primary measure.

## 10. Checks and actual results

**Correctness**

| Check | Result |
|---|---|
| `go test -race ./...` | pass |
| `gofmt` + `go vet` | clean |
| Equivalence vs preserved baseline, all 1,000 orders | **0 mismatches** |
| HTTP-level equivalence, 2,000 responses across both endpoints | **0 mismatches** |
| Error paths (404 unknown order, 400 bad id) | unchanged |
| Guard test fails when the fix is reverted | **verified by reverting** |

**Cost** — statement counts are exact and machine-independent; latencies are
local measurements, 100 samples per case after 10 warm-up runs.

| Order | Statements before | after | p50 before | p50 after |
|---|---:|---:|---:|---:|
| 1 line | 4 | 4 | 0.62 ms | 0.64 ms |
| 5 lines (p95) | 12 | 4 | 1.58 ms | 0.79 ms |
| **200 lines** | **402** | **4** | **57.29 ms** | **1.97 ms** |

```
Statements per invoice request — bars to scale, 402 at full width

  1 line      before  █ 4
              after   █ 4

  5 lines     before  █ 12
              after   █ 4

  200 lines   before  ███████████████████████████████████████████ 402
              after   █ 4
```

```
Median latency — bars to scale, 57.29 ms at full width

  1 line      before  █ 0.62 ms
              after   █ 0.64 ms      ← marginally slower, see the trade-off

  5 lines     before  █ 1.58 ms
              after   █ 0.79 ms

  200 lines   before  ███████████████████████████████████████████ 57.29 ms
              after   ██ 1.97 ms
```

**`benchstat`**, n=10 per implementation:

| | 200 lines | 1 line |
|---|---|---|
| Time | **−97.06%** (p=0.000) | no significant change (p=0.280) |
| Bytes/op | −49.54% (p=0.000) | **+40.88%** (p=0.000) |
| Allocs/op | −62.25% (p=0.000) | **+31.58%** (p=0.000) |

```
benchstat, n=10 per implementation — bar length = size of the change

  200-line order                                        improvement
    time         −97.06%  ██████████████████████████████████████   p=0.000
    bytes/op     −49.54%  ███████████████████                      p=0.000
    allocs/op    −62.25%  ████████████████████████                 p=0.000

  1-line order                                            REGRESSION
    time               ~  (no significant change)                  p=0.280
    bytes/op     +40.88%  ████████████████ worse                   p=0.000
    allocs/op    +31.58%  ████████████ worse                       p=0.000

  Reported in both directions. The small-order cost is real and accepted.
```

**Steps to change the flow safely:** before this change, none — the repository
had no tests at all. After it, `make test` covers equivalence, the statement
count, and five edge cases, and `make verify` produces the numbers.

**A regression, reported as loudly as the improvement:** one-line invoices
allocate 40.88% more memory, statistically significant. Two maps a short order
does not need. Accepted — mean order size is 3.13 lines and the absolute cost is
about a kilobyte — and recorded in ADR-001 rather than omitted.

## 11. What changed between directive v1 and this version

**The acceptance criterion was wrong.** v1 required 3 statements. Four is the
honest number for this shape; reaching three would mean a bespoke join that
couples the store layer to one caller for no measurable gain. The criterion is
now *"constant in the number of line items"* — a property rather than an
integer. It specified an implementation detail as though it were an outcome.

**The predicted failure mode did not occur.** v1 anticipated an agent would
parallelise the N queries with `errgroup`. It did not; it went straight to the
correct fix. The prediction was wrong and the real risk was subtler — see §12.

## 12. AI contribution, and what was corrected

**How AI was used.** The service, tests, harness and documentation were
substantially AI-generated in an interactive session, under the author's
direction, with the author making the scoping and prioritisation decisions,
reviewing every diff, and independently re-verifying every number reported.
A second, isolated agent was used as a subject of study, below.

**The deliberate experiment.** A fresh agent was given a copy of the repository
with `docs/` deleted, every `DEFECT-n` comment stripped, the README replaced and
the module renamed — verified by grep — so it could not see the directive or
know this was an exercise. Its prompt was the ordinary lazy version:

> *"Our invoice endpoint is too slow. `GET /orders/1/invoice` takes about 65ms…
> Customers with big orders are complaining. Please make it faster."*

Never mentioning queries, N+1 or batching.

**What it got right.** It diagnosed the N+1 unprompted and fixed it correctly:
batched lookups, deduplicated inputs, results keyed by id rather than indexed
positionally. No goroutines, no caching, no index-tuning. Independently
re-measured: 4 statements flat, 3.98 ms median, 1,000/1,000 orders identical.

**What was rejected, and the risk in each case.**

1. **It modified `summary.go`**, out of scope, where the unfixed DEFECT-2 lives.
   *Risk:* any future disagreement between the two endpoints would then have two
   candidate causes, and a before-and-after measurement would be attributable to
   neither change.
2. **It deleted the retry loop**, DEFECT-3, as a side effect of removing an
   unused method. *Risk:* the most dangerous of the three. A defect disappeared
   with no decision taken, no test covering the behaviour change, and only a
   passing remark in its summary. Failure semantics should change deliberately
   or not at all.
3. **It deleted both single-row store methods** as unused. *Risk:* those are the
   baseline implementation. Removing them destroys the ability to prove
   equivalence and to benchmark the two side by side — it deleted the evidence
   that its own diff was safe.

**The transferable finding.** The risk was not an agent doing something stupid.
It was an agent doing genuinely good work in places it had been told not to
touch — a diff where every line looks competent is far harder to reject in
review than one that is obviously wrong. It had no way to know the boundaries,
because they lived in a document it could not see. **Scope has to travel with
the code**, which is why every defect in this repository is labelled in source
with its disposition.

**What shipped** is the agent's approach with the scope restored, plus the tests
it did not write. It is not the agent's diff.

## 13. Limitations

- **Synthetic data**, fixed seed. Realistic in shape, invented in substance. No
  real customer, product or pricing data anywhere.
- **The distribution is constructed.** The 200-line order is the only large
  order and was placed there by the author to create the tail. The typical order
  is 3.13 lines and renders in under 4 ms — this is a tail fix, not an average
  one, and it is presented as such.
- **One machine, one operator.** Latency figures are from a single laptop
  (linux/amd64, 13th Gen Intel i5-13420H, 12 CPU). Statement counts are exact
  and machine-independent; latencies are not.
- **No CI.** All checks were run locally by the author. A reviewer can reproduce
  them with the commands in §9, but no third-party machine has verified them.
- **No production or team-wide claim is made.** Nothing here is evidence about
  real customers, real traffic, or the effect on any team.
- **The handoff exercise has not been performed by another engineer.** It was
  written and walked through by the author only. No observed feedback from a
  second person is available, and none is claimed.
- **DEFECT-3 was never demonstrated at runtime** — it was assessed by reading
  the code, not by fault injection.
- **Two known defects remain in the codebase on purpose.** See `defects.md`.
