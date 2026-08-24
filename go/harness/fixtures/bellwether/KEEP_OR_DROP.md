# Pre-registration: keep or drop the deterministic collision pass

**Fixed 2026-08-23, BEFORE any output was seen.** Fourth pre-registered
rule in this line of work, after `GRADING.md` (pipeline),
`MORNING_EDIT.md` (section-scoped agent join) and `SCALE_TEST.md`
(corpus-scale agent join). Written before the run so the bar cannot
drift to fit the result.

Companion to TODO.md, "Is deterministic collision-finding the right
architecture?" — specifically its 2026-08-23 status subsection, which
records that option 2 shipped and option 4 is the only open question.

**Most of this test does not run on `bellwether`,** for the same reason
`SCALE_TEST.md` doesn't: it needs corpora this fixture cannot supply.
It lives here because it closes a question this fixture opened, and
because the three pre-registered rules it follows from are all in this
directory.

**Recording rule applies.** If any arm runs on a private manuscript,
nothing about it is recorded here or anywhere in the repo: no prose, no
names, no genre, no plot. Method, thresholds and counts only.

## The question

The deterministic pass has been narrowed to one job: same entity, same
attribute name, two different values. Adjudication then decides which of
those are real. **Does that pass earn its place in the pipeline, or
should it be removed and the whole-corpus agent read left to do the
work alone?**

This is not a question about recall. The pass's 0/4 on cross-name
contradictions is the deliberate boundary of its job, not a defect. The
question is whether the class it *does* reach — same-entity,
same-attribute re-description — contains enough real errors, often
enough, to be worth the adjudicator calls and the author's attention.

## Why it can't be answered from what exists

Two measurements point opposite ways and both are n=1:

- **Real corpus, five identical runs.** Adjudicator kept 0, 1, 0, 2, 0
  collisions. Empty intersection. No run ever asserted a
  `contradiction`. Zero findings the author had not already recorded in
  the brief. On that corpus the pass contributes nothing an author sees.
- **`bellwether`, full-flow run.** Adjudicator kept 2 of 4. One fixture,
  synthetic prose, seeded.

The corpus that would most favour keeping the pass — one whose
collisions are mostly *real* — has never been measured. Dropping on the
strength of the first bullet alone is the same error the recall fix made
in the other direction: generalizing from one manuscript.

## What must be true before the run

These are the conditions that make the result readable. If any fails,
the run does not count and this spec is amended before retrying.

1. **At least three corpora**, not one. Minimum: `bellwether`
   (synthetic, seeded, already characterized), the 2026-08-13 real-prose
   corpus (330 facts, characterized), and **at least one new real
   manuscript never used to tune anything in this repo**.
2. **Each corpus is characterized before adjudication is run on it.** Its
   collisions are hand-labelled `real error` / `legitimate
   re-description` / `artifact` by reading the prose, with the labels
   written down *before* any adjudicator output is looked at. This is the
   ground truth the run is scored against, and it is the expensive part.
3. **Five runs per corpus, identical input.** Variance is the thing being
   measured, not an inconvenience. One run per corpus answers nothing —
   that is exactly the mistake the 2026-08-15 correction caught.
4. **The extraction is held fixed across the five runs.** Re-extracting
   per run confounds adjudicator variance with extractor variance.

## The metrics, and what each decides

Per corpus, across the five runs:

- **Real-error yield** — how many hand-labelled real errors the pass
  surfaced *and* the adjudicator kept, in at least 3 of 5 runs. This is
  the number that justifies keeping the pass.
- **Novel yield** — of those, how many the whole-corpus agent read did
  *not* also find, and how many were not already in the author's brief
  as a known issue. A finding the other half already produces is not a
  reason to keep this half.
- **Stability** — union and intersection of kept collisions across the
  five runs, and the per-collision keep rate. The pre-registered bands
  from the 2026-08-15 work carry over: kept in 5/5 is stable, kept in
  2/5 is unstable.
- **Cost** — adjudicator calls issued and tokens spent per real error
  surfaced.

## Decision rule, fixed in advance

Let *N* be the count of corpora (minimum 3).

- **Drop the pass** if, on a majority of corpora, novel yield is **zero**
  — every real error it surfaced was either also found by the
  whole-corpus read or already known to the author. Zero novel yield
  means it costs adjudicator calls and buys nothing, and that holds
  regardless of how stable it is.
- **Keep the pass** if, on a majority of corpora, it produces **at least
  one stable (≥3/5) novel real error**. One genuinely new error per
  manuscript is worth its cost; the pass is cheap next to the corpus
  read.
- **Neither** — mixed results across corpora — means the answer is
  manuscript-dependent, and the deliverable is not a keep/drop decision
  but a rule for *when* it applies, plus whatever corpus property
  predicts it. Do not force a global decision out of a split result.

Instability alone does not decide it. An unstable pass that surfaces a
real error the other half misses is worth keeping and worth a re-run
policy; a perfectly stable pass that surfaces nothing new is not worth
keeping. **Novel yield is the primary metric; stability governs how the
result is described to the author, not whether the pass survives.**

## What this run does not settle

Recorded so the result can't be cited as more than it is.

- **It says nothing about partitioning or trimming the agent bundle.**
  That is the other open cost problem (~82K tokens for 330 facts, a
  novel is order 6×) and it needs its own pre-registration. The 4/4 from
  `SCALE_TEST.md` was obtained on an un-partitioned bundle; any design
  that changes the input cannot inherit that number. Partition-by-entity-
  name is already measured dead at 1/4 (`MORNING_EDIT.md`) — do not
  re-propose it.
- **It says nothing about extraction quality.** Both halves read the same
  facts. A contradiction neither half finds because it was never
  extracted is invisible to this run entirely.
- **A "keep" result does not restore the "neither subsumes the other"
  claim** at the level of individual findings. That was corrected on
  2026-08-15 to hold only at the level of classes, and this run is
  scoped to yield, not to subsumption.

## Order of work

1. Amend nothing in this file once the first corpus is characterized.
2. Characterize all *N* corpora (step 2 above) before running any
   adjudicator arm.
3. Run five arms per corpus.
4. Score against the pre-written labels.
5. Append the result and the decision to TODO.md's status subsection.
   Whichever way it goes, record the number that decided it.
