# invoice-service-quest

A deliberately small Go service with one user-facing flow —
`GET /orders/{id}/invoice` — used to demonstrate a repeatable human-and-AI
workflow for finding, fixing and *proving* a maintainability problem.

Three defects are planted on purpose and labelled in the source. **One is
fixed.** The other two are recorded non-goals. See [`docs/defects.md`](docs/defects.md).

All data is synthetic and generated from a fixed PRNG seed. No real customer,
product or pricing data appears anywhere in this repository.

## Requirements

Go 1.25+, Docker with Compose v2. Nothing else.

## Run it

```bash
make env      # create .env from .env.example (optional; defaults work without it)
make up       # start Postgres 16 on localhost:5433, wait until healthy
make seed     # load the deterministic dataset
make run      # serve on :8080
```

Then, in a second terminal:

```bash
# The 200-line bulk order. The X-Query-Count header is the headline measure.
curl -s -D - -o /dev/null http://localhost:8080/orders/1/invoice | grep -i x-query-count

# A small order, for comparison — the cost scales with line count.
curl -s -D - -o /dev/null http://localhost:8080/orders/7/invoice | grep -i x-query-count

# The full invoice body.
curl -s http://localhost:8080/orders/1/invoice | jq .
```

Tear down with `make down` (deletes the database volume). `make reset` does
down, up and seed in one step.

## The measure

Every request carries an `X-Query-Count` response header, counting the SQL
statements that request cost. It comes from a `pgx.QueryTracer` attached to the
pool ([`internal/dbtrace`](internal/dbtrace/tracer.go)), about thirty lines with
no magic in them.

Query count is the primary measure rather than latency, because it is a
deterministic integer: identical on every machine, and therefore assertable in
an ordinary test instead of eyeballed on a dashboard. Latency is reported as a
secondary, machine-specific measure.

## Endpoints

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/healthz` | Liveness |
| `GET` | `/orders/{id}/invoice` | The flow under study |
| `GET` | `/orders/{id}/summary` | Order-list total; exists to expose DEFECT-2 |

## Layout

```
cmd/api          HTTP server
cmd/seed         deterministic synthetic data loader
internal/invoice the flow — Build() is where DEFECT-1 lives
internal/store   hand-written SQL over pgx; no ORM, so round trips are visible
internal/dbtrace statement counter (pgx.QueryTracer)
internal/httpapi routing and handlers
migrations       schema, applied automatically on first container start
docs             plan, defects, intent, directive, decision record, handoff
```

## Documents

| Document | What it covers |
|---|---|
| [`docs/intent.md`](docs/intent.md) | **Why this problem** — alternatives, scoring, evidence, non-goals |
| [`docs/directive.md`](docs/directive.md) | **Final directive** — scope, criteria, results and handoff appendix |
| [`docs/handoff.md`](docs/handoff.md) | How to change this code without the author, plus an exercise |
| [`docs/review-checklist.md`](docs/review-checklist.md) | What to check when reviewing a change to the flow |
| [`docs/decision-record.md`](docs/decision-record.md) | ADR-001: alternatives, trade-offs, the measured regression |
| [`docs/defects.md`](docs/defects.md) | The three planted defects and their disposition |
| [`docs/quality-yardstick.md`](docs/quality-yardstick.md) | What "good" means here, in eight points |
| [`docs/directive-v1.md`](docs/directive-v1.md) | The initial directive, written before the fix |
| [`review/ai-correction-01.md`](review/ai-correction-01.md) | An AI fix reviewed, adopted in approach, rejected as submitted |
| [`docs/plan.md`](docs/plan.md) | Execution plan for the exercise |

## Results

Measured locally, 100 samples per case. Statement counts are exact and
machine-independent; latencies are not. Reproduce with `make verify`.

| Order | Statements | p50 latency |
|---|---|---|
| 1 line | 4 → 4 | 0.62 → 0.64 ms |
| 5 lines | 12 → 4 | 1.58 → 0.79 ms |
| **200 lines** | **402 → 4** | **57.29 → 1.97 ms** |

```
Statements per invoice request — bars to scale, 402 at full width

  1 line      before  █ 4
              after   █ 4

  5 lines     before  █ 12
              after   █ 4

  200 lines   before  ███████████████████████████████████████████ 402
              after   █ 4
```

`benchstat` n=10: **−97.06%** time for the 200-line order (p=0.000); no
significant change for a 1-line order (p=0.280); small orders allocate
**+40.88%** more (p=0.000), an accepted trade-off recorded in ADR-001.

Only the invoice endpoint was fixed. The summary endpoint still has the same
N+1, deliberately — DEFECT-2 lives in that file and is a declared non-goal:

    order 1  invoice=4  summary=401
    order 2  invoice=4  summary=11
    order 7  invoice=4  summary=3

## Configuration

| Variable | Default |
|---|---|
| `DATABASE_URL` | `postgres://postgres:postgres@localhost:5433/invoices?sslmode=disable` |
| `HTTP_ADDR` | `:8080` |

The Postgres credentials are throwaway values bound to localhost by
`docker-compose.yml`. They are not secrets and are not used anywhere else.
