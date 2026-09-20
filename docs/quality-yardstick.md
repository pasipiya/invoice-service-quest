# Quality yardstick

What "good" means in this repository. Short on purpose: a yardstick nobody
remembers is not a yardstick. Applies to the invoice flow and to any change an
AI agent proposes in it.

## 1. Cost is visible at the call site

A reader must be able to tell how many database round trips a function makes by
reading it. A loop that hides a query per iteration fails this test. If the cost
is not obvious from the code, the code is wrong even when it is fast.

## 2. Cost per request does not grow with input size

Work per request should be bounded by something other than "how big the user's
order happens to be". Linear growth with no ceiling is a defect, not a
performance characteristic.

## 3. Every quality claim has a number attached

"Faster" and "cleaner" are not claims. A claim is a measure, a before value, an
after value, and a statement of how it was obtained. Prefer measures that are
deterministic and machine-independent over measures that vary by hardware.

## 4. A property worth fixing is worth guarding

Any quality property restored by a change must be asserted by a test that fails
if it regresses. Fixing without guarding means fixing it again in six months.

## 5. Behaviour is preserved unless the change is explicitly about behaviour

A refactor that alters output is a bug, however much faster it is. Equivalence
must be demonstrated by a test, not asserted in a commit message.

## 6. Money is integer cents

No floating point in any monetary calculation, anywhere. Rounding is half up.

## 7. Errors are wrapped with the identifier that caused them

`fmt.Errorf("line %d: %w", it.ID, err)`, not a bare return. A log line that
doesn't say which record failed costs someone an hour.

## 8. Scope is declared before work starts, and shrinks rather than grows

Finding a second problem mid-change means recording it, not fixing it. Two
fixes in one diff make the before-and-after attributable to neither.

---

## How an agent's output is judged against this

| Yardstick | How it is checked |
|---|---|
| 1, 7, 8 | Human review — judgement, not automatable |
| 2 | `internal/invoice/querycount_test.go` — statement count must not scale with N |
| 3 | `make verify` must produce the numbers; no number, no claim |
| 4 | The guard test must exist and must fail when the fix is reverted |
| 5 | Equivalence test across both implementations |
| 6 | Code review, plus `go vet` |
