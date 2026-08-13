# saltmarsh — a **spent** regression fixture

Three sections of literary fantasy, written 2026-08-12 specifically to be
a manuscript the tool had never seen, with two issues planted in it:

1. **A crisp factual contradiction.** The tide clock in Ferris's shop is
   "not worked in eleven years" (section_01) and "stopped these fifteen
   years" (section_03). Same object, same attribute, incompatible values.
2. **An ambiguous in-dialogue disagreement.** Ilse says Ferris has had
   the instrument "three weeks"; Ferris answers "Two." Both in dialogue,
   staged as an on-page disagreement — a judgment call, not an error.

## Why it exists

Running `/redliner:intake` → `/redliner:run assess` on it produced the
first end-to-end pass on a fresh manuscript, and it immediately found a
real defect that `sample_manuscript` could never have surfaced: the
continuity layer **extracted both tide-clock facts and never collided
them**, because independent per-section extractions named the entity
`tide clock` vs `the tide clock` and the attribute `duration_not_working`
vs `stopped_duration`, and matching was exact on both. See `TODO.md`,
"Continuity misses contradictions when extractions name things
differently".

`sample_manuscript` was authored alongside the tool, so its vocabulary
was consistent for free. That is exactly the blind spot this fixture was
written to escape.

## ⚠️ Why it is *spent*, and what that means

**The fix was tuned against this manuscript.** It is therefore useful for
catching **regressions** and useless as evidence that the fix
generalizes. Do not cite a passing run here as proof the recall bug is
solved in general — that claim needs a *different* fresh manuscript,
written without looking at the current matcher. `TODO.md` still carries
that as an open item, deliberately.

The same caveat applies to `expected/continuity.json`: it records one
real adjudication run, which is far better than the assumption it
replaced, but it is one run on one manuscript.

## Contents

| Path | What it is |
| --- | --- |
| `section_0{1,2,3}.txt` | the manuscript |
| `.redliner/brief.md` | the brief `/redliner:intake` produced |
| `.redliner/state.json` | `manuscript_dir` relativized to `saltmarsh`, matching how `happy` stores it |
| `.redliner/canon/observations/*.json` | 60 extracted facts across 16 entities — **the valuable part**: they let `canon reconcile` be re-run deterministically with no model calls |
| `expected/collisions.json` | 12 collisions the current matcher finds, including the planted one |
| `expected/continuity.json` | what the adjudicator kept from those 12: exactly 2 |

## Using it

Copy to a scratch directory (reconcile writes into `.redliner/canon/`),
then:

```
redliner canon reconcile <copy>
```

Compare against `expected/collisions.json`. The one that must never
disappear:

```
tide clock.duration_not_working: ['eleven years', 'fifteen years']
```

If that stops being reported, the recall fix has regressed — and note the
failure is silent, since a missed contradiction produces no artifact and
looks exactly like a clean manuscript.

## Deliberately NOT wired into the differential harness

This fixture is **not** in `capture_baseline.py`'s `FIXTURE_SCRIPTS` and
has no golden files. The harness exists to prove the Go port matches the
frozen Python oracle; this fixture tests a *behavioral* property of the
current matcher instead. Adding it there would conflate the two, and
would mean regenerating goldens every time this fixture's expectations
change.
