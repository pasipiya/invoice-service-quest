# intent.md — Why this problem?

**Quest:** Make AI-Assisted Code Easier to Trust and Change
**Repository:** `invoice-service-quest`
**Flow under study:** `GET /orders/{id}/invoice`
**Baseline state:** tag `baseline`

---

## 1. The setting

A small Go service over PostgreSQL with one user-facing flow: rendering a
customer invoice for an order. The repository is purpose-built for this
exercise. Three defects were introduced deliberately and labelled in the source
and in [`docs/defects.md`](https://github.com/pasipiya/invoice-service-quest/blob/submission-v1/docs/defects.md). All data is synthetic, generated from a
fixed PRNG seed so that every measurement below is reproducible by a reviewer.

The exercise is to pick **one** of the three, fix it, and prove the fix.

---

## 2. The three problems considered

### DEFECT-1 — N+1 product and tax-rule lookups

`Service.Build` loops over line items and issues two queries per line: one for
the product, one for the tax rule of that line's ship-to region. Cost is
`2 + 2N` statements for one HTTP request, growing linearly with order size and
bounded by nothing.

### DEFECT-2 — Duplicated tax rule that has drifted

The same tax calculation exists twice. `taxCents` (invoice) rounds half up;
`summaryTaxCents` (order list) truncates. The two endpoints therefore report
different totals for the same order, and any rate or rounding change must be
applied in two places.

### DEFECT-3 — Unguarded retry loop

`GetTaxRuleForRegion` retries five times with no backoff, retrying errors that
can never succeed on a second attempt — a missing row, a cancelled context — and
hammering a struggling database at the worst possible moment.

---

## 3. Baseline evidence

All figures **measured** on 2026-09-20 against the seeded dataset (1,000 orders,
3,129 line items, 5,000 products). Local single-machine measurement; see
§8 for limitations.

**Order size distribution** — the shape that matters:

| | Lines | Statements (`2 + 2N`) |
|---|---|---|
| Minimum | 1 | 4 |
| Mean | 3.13 | 8.3 |
| p95 | 5 | 12 |
| **Maximum (order 1)** | **200** | **402** |

**Latency**, 30 samples after warm-up, `curl` against a local server:

| Order | Lines | Statements | Median | p95 |
|---|---|---|---|---|
| 7 | 1 | 4 | 2.2 ms | 2.6 ms |
| 2 | 5 | 12 | 3.7 ms | 4.6 ms |
| **1** | **200** | **402** | **60.1 ms** | **73.4 ms** |

**This is a tail problem, not an average problem.** The typical order costs 8.3
statements and renders in under 4 ms. The damage is concentrated entirely in
large orders, which is to say the largest customers. Rendering every order in
the dataset once costs 8,258 statements; batching would make it 3,000.

**DEFECT-2 drift**, computed directly in SQL across all 1,000 orders:

| Measure | Value |
|---|---|
| Orders where the two endpoints disagree | **751 of 1,000 (75.1%)** |
| Largest disagreement | **86 cents** (order 1) |

**DEFECT-3** — no runtime evidence gathered. Demonstrating it needs fault
injection, which was judged out of budget. Assessed from code inspection only,
and labelled as such below.

---

## 4. Prioritisation criteria

| Criterion | Weight | What it asks |
|---|---:|---|
| User impact | 30 | How many users are affected, how badly, how visibly? |
| Maintenance effort | 25 | How much does this cost every future engineer who touches the flow? |
| Operating cost | 20 | What does it cost to run, and how does that scale? |
| **Verifiability** | **25** | Can the fix be proven with reproducible, machine-independent evidence? |

The first three come from the Quest brief. **The fourth is mine, and it is
weighted heavily for a reason specific to this exercise:** the deliverable is
not only a fix but a demonstration that a fix can be *trusted*. A problem whose
improvement can only be argued, not measured, is a poor vehicle for that
regardless of its business value.

**Is a self-invented criterion self-serving?** It would be, if it were hidden.
So here is the arithmetic without it: on the brief's three criteria alone,
DEFECT-1 scores 54 and DEFECT-2 scores 54. **They tie exactly.** Verifiability
is not padding a lead — it is breaking a genuine tie, and it is the only
criterion that does any work in this decision. A reader who disagrees with the
criterion can remove it and see the tie for themselves.

---

## 5. Scoring

Each cell scored out of its criterion's weight, with the evidence that drove it.

| | DEFECT-1 N+1 | DEFECT-2 drift | DEFECT-3 retry |
|---|---:|---:|---:|
| User impact (30) | **21** | **27** | **9** |
| Maintenance effort (25) | **15** | **23** | **10** |
| Operating cost (20) | **18** | **4** | **8** |
| Verifiability (25) | **25** | **12** | **10** |
| **Total (100)** | **79** | **66** | **37** |

**User impact** — DEFECT-2 scores highest, and it deserves to: it is a
correctness bug affecting 75.1% of orders, and it shows customers two different
money amounts for the same purchase. DEFECT-1 affects far fewer users, but
affects them severely and affects the wrong ones — a 200-line order is 27× the
median latency. DEFECT-3 is invisible until the database is already in trouble.

**Maintenance effort** — DEFECT-2 again scores highest: a business rule living
in two places will be changed in one. DEFECT-1 costs less per change but makes
the flow's cost illegible; nothing at the call site tells you a loop iteration
is a network round trip.

**Operating cost** — DEFECT-1 dominates. It is the only one of the three with
unbounded growth: database load scales with order size, with no ceiling and no
alarm. DEFECT-2 costs nothing to run.

**Verifiability** — DEFECT-1 is the clear winner and the reason it ranks first
overall. Its measure is a deterministic integer, identical on every machine,
assertable in an ordinary test. DEFECT-2's natural measure is "how many places
must change", which is a judgement call; proving the fix would mean proving an
absence. DEFECT-3 needs fault injection to demonstrate at all.

---

## 6. Why DEFECT-1 ranked first

**On a real team, I would fix DEFECT-2 first.** It is a correctness bug. It
affects 75.1% of orders, it shows customers two different amounts for the same
purchase, and money that disagrees with itself erodes trust in a way a slow page
does not. I would not want to explain to a reviewer, or to a customer, why I
spent the week on latency instead.

I chose DEFECT-1 anyway, for two reasons I can defend.

**The first is the nature of the risk.** The drift in DEFECT-2 is bounded and
static: at most 86 cents, and it will be 86 cents next year. It is a known
quantity — bad, but sized. DEFECT-1 is unbounded and gets worse as the business
succeeds. A 200-line order costs 402 statements today; a 1,000-line order would
cost 2,002, and nothing in the code, the schema or the tests would object. The
customers it punishes hardest are the ones placing the largest orders. A defect
whose cost is proportional to your success is a different class of problem from
one whose cost is fixed, even when the fixed one is larger today.

**The second is what this exercise is for.** The task is not only to improve
code, it is to show that an improvement can be *trusted* — by me, by a reviewer,
and by whoever changes the flow next. DEFECT-1 produces evidence that survives
leaving my laptop: a statement count is an integer, identical everywhere, and it
can be asserted in a test so that the property is *held* rather than merely
restored. Proving DEFECT-2 fixed means proving that two things now agree and
will keep agreeing, which is a harder argument to make well in the time
available, and a weaker demonstration if made badly.

So: DEFECT-2 is the more important bug, and DEFECT-1 is the better vehicle for
what is being asked here. I would rather state that plainly than construct a
scoring matrix that makes the convenient answer look inevitable.

---

## 7. Affected users, and intended value

**Who is affected by DEFECT-1:** customers viewing invoices for large orders.
In the synthetic dataset that is a small minority — one order in a thousand.
What matters is not how many are affected but that the relationship is linear
with no upper bound, so the problem worsens as orders grow rather than staying
constant.

> **Assumption, not evidence:** it is tempting to add that large orders belong
> to the highest-value customers, which would make this a revenue argument
> rather than a latency one. There is no basis for that here. The dataset is
> synthetic and the 200-line order was placed in it by me. The claim is stated
> as an assumption and is not relied on anywhere in this submission.

**Intended value:**

1. Invoice rendering cost becomes constant instead of linear in order size.
2. The cost becomes *guarded* — a test fails if a future change reintroduces
   per-line querying, so the property is maintained rather than merely restored.
3. The flow becomes legible: a reader can see how many round trips it makes.

**What success looks like:** statements per invoice request drop from `2 + 2N`
to a constant 3, invoice output is byte-identical before and after, and a
regression is caught by `make test` rather than by a customer.

---

## 8. Non-goals — what I will not change

| Not changing | Why |
|---|---|
| **DEFECT-2** (drifted tax rule) | Higher business impact, deliberately excluded. Its improvement cannot be evidenced as cleanly within the budget, and fixing two defects at once would make the before-and-after attributable to neither. |
| **DEFECT-3** (unguarded retry) | Needs fault injection to demonstrate; out of budget. Recorded, not silently dropped. |
| The `/summary` endpoint | Exists only to expose DEFECT-2. Untouched. |
| Schema and indexes | The fix must come from the access pattern, not from hiding it behind an index. |
| Caching of any kind | Would improve latency while leaving the underlying access pattern — and its correctness risks — in place. See ADR-001. |
| Framework, ORM, or dependency changes | Out of scope for a one-flow exercise. |

**Also not claimed:** any team-wide, organisation-wide or production impact. All
figures are local, single-machine measurements against synthetic data.

---

## 9. Limitations of this evidence

- Synthetic data with a fixed seed. Realistic in shape, invented in substance.
- One machine, one run of each measurement. Latency figures are indicative;
  the statement counts are exact and deterministic.
- The 200-line order is the *only* large order in the dataset. It was placed
  there deliberately to create the tail, so the distribution is constructed,
  not observed.
- DEFECT-3 was assessed by reading the code, not by running it.
- No user research, no production telemetry, no interviews. Nothing here is
  evidence about real customers.

---

## 10. Related documents

| Document | Contents |
|---|---|
| [`defects.md`](https://github.com/pasipiya/invoice-service-quest/blob/submission-v1/docs/defects.md) | The three defects in detail |
| [`plan.md`](https://github.com/pasipiya/invoice-service-quest/blob/submission-v1/docs/plan.md) | Execution plan for the exercise |
| [`directive.md`](https://github.com/pasipiya/invoice-service-quest/blob/submission-v1/docs/directive.md) | Working instructions, completion criteria, results and handoff |
| [`decision-record.md`](https://github.com/pasipiya/invoice-service-quest/blob/submission-v1/docs/decision-record.md) | ADR-001: alternatives considered for the fix itself |
| [`review/ai-correction-01.md`](https://github.com/pasipiya/invoice-service-quest/blob/submission-v1/review/ai-correction-01.md) | An AI proposal for this fix, reviewed and corrected |
