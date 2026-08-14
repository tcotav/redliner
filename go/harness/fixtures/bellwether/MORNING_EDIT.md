# Morning-edit grading rule — fixed 2026-08-14, BEFORE any output was seen

Same discipline as `GRADING.md`: the bar is written down first so it
cannot drift to fit the result. `GRADING.md` grades the **pipeline**
(`canon reconcile` + adjudicator). This grades a **different shape** on
the same planted contradictions, so the two numbers are comparable.

## The question

The pipeline scores **0/4** on this fixture, and the root cause is
`ComputeReconcile` partitioning facts by `normEntity`
(`go/internal/cli/canon.go:455-461`) so `Renata Sowa`/`Ren`,
`Emil`/`her father`, and `Lyman`/`the boat` never meet. That is a
deterministic string matcher losing at aliasing.

**Does an agent-in-the-loop "morning edit" route around that component,
or does it inherit the same 0/4?**

Morning edit = the author finished a section yesterday and, before
starting the next one, asks what it contradicts in what came before.
One agent, one section of new text, the facts already extracted from
prior sections. No `reconcile` call in the path.

This is a test of the *idea*, not of a built feature. Nothing is being
tuned against this fixture — see README.md on why that would destroy
its only value.

## Method

Two simulated mornings, using the observation files already committed at
`.redliner/canon/observations/` (the real extractor's output from the
2026-08-12 run — **not re-extracted**, so extraction quality is held
constant and only the join is under test):

| Morning | New section | Prior facts supplied | Planted targets | Decoys in play |
| --- | --- | --- | --- | --- |
| A | `section_03` (25 facts) | sections 01–02 (59 facts) | #1 absence length, #2 father's age | A (two boats) |
| B | `section_04` (27 facts) | sections 01–03 (84 facts) | #3 hull length, #4 deathbed presence | B (water level), C (never asked) |

Denominator is **4**, the same four planted contradictions
`GRADING.md` scores. Every planted pair is reachable: #1 and #2 are
sections 1↔3, #3 is 1↔4, #4 is 2↔4.

The agent is given the new section's full text, the prior facts as JSON,
and no other repo file. `GROUND_TRUTH.md`, `GRADING.md`, `README.md`,
and `expected/` are withheld — inputs are copied to a scratch directory
so the fixture directory is never listed.

## Two arms, scored separately

1. **All-prior-facts.** Every fact from every prior section. Best case,
   and the ceiling for this approach. If it fails here the idea is dead
   and no scoping scheme saves it.
2. **Entity-scoped.** Only prior facts whose entity name matches (by the
   same lowercase + strip-one-leading-article rule `normEntity` uses) an
   entity named in the new section. This is the obvious cut for scale —
   a full novel is order 2,000 facts, too many to hand an agent every
   morning.

**Prediction recorded in advance:** arm 2 fails on the three Class A
aliased pairs, because name-based scoping reintroduces the exact
matching step that produced the 0/4 — it filters out `Lyman` before the
agent can connect it to `the boat`. If that prediction holds, the
finding is *the scoping cut cannot be name-based*, which is worth more
than arm 1's score. If arm 2 does better than predicted, the prediction
was wrong and gets recorded as wrong.

## Scoring

Per planted contradiction, inheriting `GRADING.md`'s rule verbatim:

- **FOUND** — the agent surfaces it as a contradiction or a question to
  the author, naming both halves.
- **MISS** — not surfaced, *or* surfaced and then dismissed. A
  dismissal and a non-detection are indistinguishable from the author's
  seat; both mean it never reaches them.

Partial credit does not exist. Identifying the right pair but
mis-describing why still counts as FOUND — the author sees the two
halves and can judge; that is the whole product.

| Result | Reading |
| --- | --- |
| 4/4 arm 1 | The morning edit routes around the matcher entirely. Build it; entity-matching is not its prerequisite. |
| 2–3/4 arm 1 | Real signal, real gap. Diagnose which class is missed before building. Do not average it away. |
| ≤1/4 arm 1 | The idea inherits the pipeline's ceiling. The framing that the agent "routes around `normEntity`" is wrong, and the entity-matching fix is the prerequisite after all. |

Decoy precision, same buckets as `GRADING.md`: dismissed → correct;
raised as a question needing author confirmation → acceptable; asserted
as a contradiction → precision regression, recorded but not a reason to
change the recall verdict.

## Result, 2026-08-14 — arm 1: 4/4. arm 2: 1/4.

Run after the rule above was committed (`6de13d5`), against the grading
it fixes. The pipeline's score on the same four contradictions is 0/4.

| # | Planted | Class | Pipeline | Arm 1 (all facts) | Arm 2 (entity-scoped) |
| --- | --- | --- | --- | --- | --- |
| 1 | absence 19 winters vs 23 years | A | MISS | **FOUND** | MISS |
| 2 | father's age 81 vs ~76 | A | MISS | **FOUND** | MISS |
| 3 | hull 26ft vs 31ft | A | MISS | **FOUND** | MISS |
| 4 | deathbed present vs alone | B | MISS | **FOUND** | **FOUND** |

**Arm 1 found all four, and named the aliased halves explicitly** — it
connected `twenty-three years` to `Renata Sowa`'s `nineteen winters`,
`her father` to `Emil`, and identified the 31-foot hull as the Lyman by
reasoning from shared construction details ("white oak and mahogany...
so it reads as the same boat") rather than from the name. That is the
exact join `normEntity` cannot make, and it is the reason the 0/4
happened.

**Precision: clean.** No decoy was asserted as a contradiction in any
arm — the two boats, the harbor water level, and Kaja's "he never asked
about you" were all correctly left alone. Arm 2 raised one extra item as
a `question` (time of death against section 1's "the morning he died"),
correctly noting the earlier line is Ren's supposition; that is the
acceptable bucket, not a regression.

**The arm 2 prediction held exactly, including which ones it would
miss.** Name-based scoping missed precisely the three Class A pairs and
caught the one Class B pair. The mechanism is visible in what survived
the filter: morning A retained facts for only `Ren` and `the shop`,
having dropped `Renata Sowa` and `Emil` because section 3 calls them
`Ren` and `Ren's father`; morning B retained `Emil`/`Kaja` (so #4 was
reachable) but dropped `Lyman`, because section 4 only ever says `the
boat`. **So the scoping cut cannot be name-based** — it reintroduces the
partition that caused the 0/4, one step earlier in the pipeline.

Note the inversion worth keeping: Class B, the propositional mismatch
`GROUND_TRUTH.md` calls "not reachable by name matching at all" and
"materially harder", is the *easy* one for an agent — it is the only one
arm 2 got. The hard class for a string matcher and the hard class for a
reader are not the same class.

## What a pass here will NOT prove

- **Nothing about scale.** Four sections and 84 prior facts is not a
  novel. Arm 2 probes the scoping question but does not settle it.
- **Nothing about real prose.** This manuscript was written to contain
  contradictions. The 2026-08-13 real-prose run is the evidence class
  this fixture explicitly cannot supply, and it is where signal-to-noise
  actually gets hard.
- **Nothing about cost.** Two agent calls on 1,260 words says nothing
  about what this costs per morning on a real manuscript.
- **It is one fixture.** Two data points was not a generalization curve
  when `GRADING.md` said so, and this does not change that.
