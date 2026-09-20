# Quest execution plan

Working plan for the Quest "Make AI-Assisted Code Easier to Trust and Change".
Living document: updated as the work proceeds. Not a required submission item.

## Chosen problem

An N+1 query pattern in a single user-facing flow: `GET /orders/{id}/invoice`.

The flow loads an order, then issues one product query and one tax-rule query
**per line item**. For a 200-line bulk order that is `2 + 2N = 402` queries for
one HTTP request. Fixed by batching to 3 queries.

Selected over two alternatives on the grounds that its primary measure — query
count — is a deterministic integer, identical on any machine and assertable in a
test. Latency is reported as a secondary, machine-specific measure.
Full reasoning, scoring matrix and non-goals: `docs/intent.md`.

## Deliberately planted defects

Three defects are introduced in the baseline and clearly labelled in code and in
`docs/defects.md`. **Only DEFECT-1 is fixed.** The other two are named non-goals
and are left in place on purpose, to demonstrate scope discipline.

| ID | Defect | Disposition |
|----|--------|-------------|
| DEFECT-1 | N+1 product and tax-rule lookups in the invoice flow | **Fixed** |
| DEFECT-2 | Duplicated total/tax calculation that has drifted | Non-goal |
| DEFECT-3 | Unguarded retry loop (no backoff, retries non-transient errors) | Non-goal |

## Stack

Go 1.25 · stdlib `net/http` · pgx/v5 + pgxpool · PostgreSQL 16 via Docker Compose.

Query counting uses `pgx.QueryTracer` attached to the pool config, tallying
statements into a per-request counter carried on the context. Roughly 30 lines,
no magic, auditable by a reviewer in one sitting.

Postgres rather than SQLite deliberately: N+1 costs a network round trip per
query, which is where the real damage is. In-process SQLite would understate the
problem and make the demonstration misleading.

## Measures

| Measure | Kind | Why |
|---|---|---|
| SQL statements per invoice request | **measured**, deterministic | Machine-independent; assertable in a guard test |
| p50 / p95 latency | **measured**, machine-specific | `go test -bench`, n=10, compared with `benchstat` |
| Regression guard present | binary | Prevents silent reintroduction |

All figures are local single-machine measurements against synthetic data. No
team-wide or production claim is made anywhere in the submission.

## Branch and tag sequence

History order is itself evidence: the directive must demonstrably predate the fix.

| # | Branch | Contents | Ends with |
|---|--------|----------|-----------|
| 1 | `chore/scaffold-baseline` | service, schema, seeder, tracer, 3 labelled defects | tag `baseline` |
| 2 | `docs/intent-and-directive` | `intent.md`, `directive-v1.md`, quality yardstick | — |
| 3 | `fix/invoice-n1-batched-lookups` | the fix, guard test, benchmarks — **opened as a PR** | tag `improved` |
| 4 | `docs/results-and-handoff` | results, ADR-001, review example, handoff, final `directive.md` | tag `submission-v1` |

Merges use `--no-ff`. Squash merging is avoided: it would collapse the commit
ordering that evidences the process.

## Deliverables

Three items are submitted: the Loom video, `docs/intent.md` and
`docs/directive.md`. Every other artifact is reached through links in the
directive's results/handoff appendix, pinned to the `submission-v1` commit.

## Working agreement

- Branches, commits and tags are created by the author, not by the AI agent.
- Commits whose content was substantially AI-generated carry a
  `Co-Authored-By:` trailer, giving a machine-readable AI-contribution record
  in `git log`.
- At least one AI proposal is recorded as rejected, with the risk explained:
  `review/ai-correction-01.md`.
