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
| [`docs/plan.md`](docs/plan.md) | Execution plan for the exercise |
| [`docs/defects.md`](docs/defects.md) | The three planted defects and their disposition |
| `docs/intent.md` | Why this problem: alternatives, scoring, evidence, non-goals |
| `docs/directive.md` | Working instructions, completion criteria, results and handoff |

## Configuration

| Variable | Default |
|---|---|
| `DATABASE_URL` | `postgres://postgres:postgres@localhost:5433/invoices?sslmode=disable` |
| `HTTP_ADDR` | `:8080` |

The Postgres credentials are throwaway values bound to localhost by
`docker-compose.yml`. They are not secrets and are not used anywhere else.
