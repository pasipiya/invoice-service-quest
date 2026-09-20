# Handoff

Everything another engineer needs to change this flow without asking me.

---

## 1. Get it running

```bash
make up      # Postgres 16 on :5433, waits until healthy
make seed    # deterministic synthetic data
make verify  # before-and-after numbers, writes results/
```

Three commands, no other setup. `make run` serves on `:8080`; `make reset`
rebuilds the database from scratch. Requirements: Go 1.25+, Docker Compose.

## 2. What this service is

One flow: `GET /orders/{id}/invoice`. It loads an order, its line items, the
products those lines refer to and the tax rule for each line's ship-to region,
then prices every line in integer cents.

**The one rule that shapes everything:** a line's ship-to `region` may differ
from the order's region, so tax is resolved *per line*, not once per order.
Resolving it once per order would be faster and would produce wrong invoices.

**The cost contract:** building an invoice costs a constant **4** SQL
statements, whatever the order looks like. This is enforced by
`TestStatementCountIsConstant`, not by convention.

## 3. Where things are

| Path | What it is |
|---|---|
| `internal/invoice/service.go` | The flow. `Build` is the function you will change. |
| `internal/invoice/types.go` | `Invoice`, `Line`, and `TaxCents` — the single copy of the tax rule |
| `internal/invoice/legacy/` | The pre-fix implementation. Not routed. Exists for the equivalence test and benchmark. Do not wire it up. |
| `internal/invoice/summary.go` | **Known broken.** DEFECT-2 lives here. Read `docs/defects.md` before touching it. |
| `internal/store/` | Hand-written SQL over pgx. No ORM, so round trips are visible at the call site. |
| `internal/dbtrace/` | The statement counter. ~30 lines. |
| `cmd/verify/` | Measures both implementations, writes `results/` |

## 4. Three things that will bite you

**`ANY($1)` does not preserve order.** The batched lookups return maps keyed by
id for this reason. If you add a batch query, key it — never zip results against
the slice you asked for. A small test dataset will not catch this.

**There are two tax calculations and they disagree.** `TaxCents` rounds half up;
`summaryTaxCents` in `summary.go` truncates. This is DEFECT-2, a deliberate
known bug, and 751 of the 1,000 seeded orders are affected. If you change tax
logic, you must decide which one you are changing and why.

**The retry loop in `GetTaxRuleForRegion` is wrong on purpose.** It retries
errors that can never succeed and has no backoff. That is DEFECT-3. Do not
"tidy" it as part of another change — fix it deliberately with a test, or leave
it.

## 5. Handoff exercise

A real change that touches every layer. It should take under an hour.

> **Add a per-line discount.** Line items gain a `discount_bp` column (basis
> points, applied to the line's net before tax). The invoice response gains
> `discount_cents` per line and a `discount_cents` total.

Work through it in this order:

1. Add the column in a new migration, and a value to the seeder.
2. Add the field to `store.LineItem` and its `SELECT`.
3. Add `DiscountCents` to `invoice.Line` and `invoice.Invoice`.
4. Apply the discount in `Build`, before tax.
5. Update the tests.

**You are done when:**

- [ ] `make test` passes, race detector clean.
- [ ] `TestStatementCountIsConstant` still passes — **the statement count must
      still be 4.** If you added a query per line to fetch the discount, you
      have reintroduced DEFECT-1 and the test will tell you.
- [ ] `make verify` shows a constant statement count across all three order
      sizes.
- [ ] You decided, explicitly, whether `Summarise` should also apply the
      discount — and wrote your decision down either way.

**The point of the exercise:** step 5 is where the guard test earns its keep,
and the last checkbox is where you meet DEFECT-2 and have to make a scope
decision of your own. You will know the equivalence test needs updating, because
the invoice output is now *supposed* to change — that is the one case where
changing it is correct.

> **Limitation, stated plainly:** this exercise has not been performed by
> another engineer. It was written and walked through by the author only. No
> observed feedback from a second person is available, and none is claimed.

## 6. Making a change safely

1. `make verify` first, so you have a before.
2. Make the change.
3. `make test` — equivalence and guard tests are the net.
4. `make verify` again and compare. `make compare` if you want `benchstat`
   with a p-value.
5. Review against `docs/review-checklist.md`.
6. Anything you found but did not fix goes in `docs/defects.md`.

## 7. If you are handing this to an AI agent

Give it `docs/directive.md` and `docs/quality-yardstick.md` in the prompt, not
just the repository. The scope boundaries are the part an agent cannot infer
from the code — that is exactly what went wrong in
`review/ai-correction-01.md`, where an agent produced a technically correct fix
that edited two files it should not have touched and deleted a defect nobody had
decided to remove.

Review its output against the checklist, top section first.
