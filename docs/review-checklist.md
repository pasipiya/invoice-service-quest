# Review checklist — changes to the invoice flow

For reviewing any change to `internal/invoice`, whether written by a person or
an agent. Ordered by how often each item actually catches something. Derived
from [`quality-yardstick.md`](quality-yardstick.md) and from the review in
[`../review/ai-correction-01.md`](../review/ai-correction-01.md).

## Before reading the code

- [ ] **Does the diff touch only what it was supposed to?** List the files, then
      check them against the change's stated scope. This is first because it is
      what an AI agent gets wrong most often, and the code in the extra files
      usually looks fine — which is what makes it easy to wave through.
- [ ] **Is DEFECT-2 or DEFECT-3 quietly fixed?** Both are deliberate non-goals.
      `git diff -- internal/invoice/summary.go internal/store/taxrules.go`.
      Removing a defect as a side effect of another change is still an
      unreviewed behaviour change.

## Cost

- [ ] **Does anything inside a loop touch the database?** Read every loop body
      and follow the calls. A query per iteration is the defect this flow
      already had once.
- [ ] **Does the statement count still hold?** `make verify`, or run
      `TestStatementCountIsConstant`. A number in a commit message is not
      evidence.
- [ ] **If the count went up, is there a reason in the commit message?** Four is
      a ceiling set deliberately, not an accident.

## Correctness

- [ ] **Is tax still resolved per line, from that line's own region?** Resolving
      once per order is faster and wrong. `TestPerLineTaxRegionIsPreserved`.
- [ ] **Does the equivalence test still pass?** Output must be identical to the
      preserved baseline across all 1,000 orders unless the change is explicitly
      about behaviour.
- [ ] **Any positional indexing of batch results?** `ANY($1)` gives no ordering
      guarantee. Results must be keyed by id, never zipped against the input
      slice. This is the bug that a small test dataset will not catch.
- [ ] **Are duplicate products or regions on one order handled?** An order with
      the same product on fifty lines must ask once and price fifty.
- [ ] **Integer cents everywhere, rounding half up?** No floats in money, ever.
- [ ] **Do errors name the offending line?** `line %d:` — a failure that does not
      say which record costs someone an hour.

## Tests

- [ ] **Does a new property come with a test that guards it?** Fixing without
      guarding means fixing it again later.
- [ ] **Would the new test actually fail?** Revert the change, run the test,
      watch it fail, restore. Do not assume — a guard that cannot fail is
      decoration.
- [ ] **Do tests skip cleanly without a database**, rather than failing?

## Claims

- [ ] **Does every number have a method attached?** How many samples, on what
      machine, against what data.
- [ ] **Is measured distinguished from estimated?**
- [ ] **Are regressions reported as loudly as improvements?** The small-order
      allocation cost in ADR-001 is the standard to hold to.

## Before approving

- [ ] `make lint` and `make test` pass.
- [ ] Anything found but not fixed is written down in `docs/defects.md`, not
      left in a review comment that disappears when the PR is merged.
