# Grading rule — fixed 2026-08-12, BEFORE any output was seen

This fixture exists to answer one question TODO.md:1431-1434 leaves open:
does the continuity recall fix generalize, or does it only work on the
manuscript it was tuned against (`saltmarsh`)?

The bar is set here, in advance, so it cannot drift to fit the result.

## What is being measured

The manuscript was authored by an agent that never saw redliner's
extractor output, fact schema, collision matcher, or the saltmarsh
fixture. It planted **4 contradictions** and **3 reconcilable decoys**.
Ground truth is in `GROUND_TRUTH.md`, written by that same agent.

Every planted pair sits in **non-adjacent sections** and uses
**different wording/naming** across its two halves. That is deliberate:
identical phrasing in both halves is the case the old (broken) matcher
already handled, so a fixture without naming variation would pass
trivially and prove nothing.

## Two stages, scored separately

A planted contradiction has to survive two gates, and conflating them
would hide which one broke:

1. **Matcher recall** — did `canon reconcile` surface the pair as a
   collision at all? This is the gate the recall bug lived behind.
2. **End-to-end recall** — did the adjudicator then keep it?

**A collision the matcher surfaced but the adjudicator dismissed counts
as a MISS**, not a partial credit. The failure class this whole line of
work exists to eliminate is *silent* misses, and from the author's seat
a dismissal and a non-detection are indistinguishable — both mean the
contradiction never reaches them.

## Thresholds

| Result | Reading |
| --- | --- |
| 4/4 end-to-end | Fix generalizes. Caveat at TODO.md:1431 is discharged. |
| 3/4 | Partial. The miss gets diagnosed before anything else is built; do not average it away. |
| ≤2/4 | Fix does not generalize. It was tuned to saltmarsh. Reopen. |

Decoys (precision):

- Dismissed outright → correct.
- Kept as `unverified` → acceptable. That is the "needs author
  confirmation" bucket, and saltmarsh showed the adjudicator using it
  correctly for on-page dialogue disagreement.
- Kept as `contradiction` → precision regression. Record it; it is not a
  reason to fail the recall verdict, since cost-not-correctness was the
  standing conclusion on false positives (TODO.md:1413).

## What a pass here does NOT prove

Recorded up front, in the same discipline as saltmarsh's "spent" note,
because this fixture will get cited later:

- The prose is **synthetic and written to contain contradictions**. It
  tests whether the matcher survives naming variation it has never seen.
  It does **not** test recall on prose written with no contradictions in
  mind, where the signal-to-noise is far worse.
- It is one more manuscript, not a corpus. Two data points is not a
  generalization curve.
- Like saltmarsh, this stays **out** of `capture_baseline.py`'s
  `FIXTURE_SCRIPTS`. The differential harness proves Go-matches-Python;
  this tests a behavioural property of the current matcher. Conflating
  them means regenerating goldens whenever these expectations change.
