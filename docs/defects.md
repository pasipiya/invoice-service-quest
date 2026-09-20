# Deliberately planted defects

This repository is a purpose-built exercise. Three defects were introduced on
purpose in the baseline, each labelled in the source with its ID. **Only
DEFECT-1 is fixed.** The other two are explicit non-goals, left in place to
demonstrate scope discipline rather than through oversight.

Everything here is synthetic. No real customer, product or pricing data appears
in this repository.

---

## DEFECT-1 — N+1 product and tax-rule lookups *(FIXED)*

**Where:** `internal/invoice/service.go`, the loop in `Service.Build`.

The flow loads the order header and its line items, then issues **two further
queries per line item** — one for the product, one for the tax rule of that
line's ship-to region.

```
1        SELECT order
1        SELECT line_items
N        SELECT product      ← per line
N        SELECT tax_rule     ← per line
-------
2 + 2N   statements for one HTTP request
```

For order 1 in the seed data (200 lines) that is **402 statements**.

```
Statements as the order grows — nothing caps this

     1 line    █ 4
     5 lines   █ 12
    25 lines   █ 52
    50 lines   ███ 102
   100 lines   ██████ 202
   200 lines   ███████████ 402   ← the order in the seed data
   500 lines   ████████████████████████████ 1002
  1000 lines   ████████████████████████████████████████████████████████ 2002

  Cost = 2 + 2N. The largest customers get the worst page. That is backwards.
```

The cost grows linearly with order size, so the largest customers get the worst
latency — the inverse of what the business would want.

**Why this one is worth fixing:** its primary measure is a deterministic
integer, identical on every machine, and therefore assertable in an ordinary
test rather than observed on a dashboard. Ranking and alternatives:
`docs/intent.md`.

---

## DEFECT-2 — Duplicated tax rule that has drifted *(NON-GOAL, not fixed)*

**Where:** `taxCents` in `internal/invoice/types.go` versus `summaryTaxCents`
in `internal/invoice/summary.go`.

The same business rule exists twice and the two copies no longer agree:

| Copy | Used by | Rounding |
|---|---|---|
| `taxCents` | `GET /orders/{id}/invoice` | half up |
| `summaryTaxCents` | `GET /orders/{id}/summary` | truncates |

They differ by up to one cent per line, so the total in a customer's order list
can disagree with the invoice for the same order. A rate or rounding change must
be made in two places, and will be made in one.

**Why it is not fixed here:** it is a genuine correctness and maintainability
problem, and arguably the more interesting one. It is excluded because its
natural measure — "how many places must change" — is a judgement call, whereas
DEFECT-1 yields a hard number a reviewer can reproduce. Keeping the Quest to one
flow and one measurable change was the higher priority.

---

## DEFECT-3 — Unguarded retry loop *(NON-GOAL, not fixed)*

**Where:** `Store.GetTaxRuleForRegion` in `internal/store/taxrules.go`.

The retry loop is wrong in three ways:

1. It retries `ErrNotFound`. A missing row will not appear on attempt two.
2. It retries a cancelled context instead of returning immediately.
3. It has no backoff or jitter, so a struggling database is hit five times as
   hard at exactly the wrong moment.

**Why it is not fixed here:** demonstrating it properly needs fault injection,
which would cost more of the time budget than the Quest's scope justifies. It is
recorded rather than silently ignored.

---

## Interaction worth noting

DEFECT-1 and DEFECT-3 compound. Five attempts against a failing database, per
line item, on a 200-line order is up to 1,000 tax-rule statements from a single
request. Fixing DEFECT-1 reduces that blast radius from `5N` to `5` even though
DEFECT-3 itself is untouched — a side effect claimed here only as reasoning, not
as a measured result.
