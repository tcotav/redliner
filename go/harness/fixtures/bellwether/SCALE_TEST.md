# Full-corpus agent-join grading rule — fixed 2026-08-14, BEFORE any output was seen

Third pre-registered rule in this fixture's line of work, after
`GRADING.md` (pipeline) and `MORNING_EDIT.md` (section-scoped agent
join). Written before the run, so the bar cannot drift to fit the
result.

**This test does not run on `bellwether`.** It runs on the private
real-prose corpus measured on 2026-08-13 — 330 facts, 87 entities, 5
sections. It lives here because it settles a question this fixture
opened. **Nothing about that manuscript is recorded here**: no prose, no
names, no genre, no plot. Only method, thresholds and counts. See
TODO.md's recording rule.

## The question, and why it gates

TODO.md's "Is deterministic collision-finding the right architecture?"
lists four options and forbids building any of them until this is
measured. The morning-edit result (4/4 where the pipeline scores 0/4)
was obtained at **section scale** — 84 prior facts. Every option that
would reduce the deterministic layer's role depends on the agent join
still working at **corpus scale**, where signal-to-noise is far worse and
nothing has been measured.

The honest expectation on record is that it degrades.

## Method

Two arms, identical input shape, one agent call each. The agent is given
the corpus's extracted facts and asked to report contradictions; it sees
no section prose, no ground truth, and no other repo file.

- **Control arm** — the 330 facts exactly as extracted.
- **Seeded arm** — the same 330 facts plus **4 planted contradictions**.

**`excerpt` is stripped from the fact bundle in both arms.** A seeded
contradiction mutates a fact's *value*; leaving the original verbatim
excerpt beside it would let the agent detect every seed by
value-vs-excerpt mismatch, with no entity join required, and return a
falsely high recall. Stripping also matches what a scale-conscious design
would really send — excerpts are the bulk of the token cost.

### How the seeds are chosen

**Target selection is mechanical, not hand-picked**, because a human who
has been reading this corpus all session will unconsciously pick findable
ones — the flaw `README.md` warns about in "spent" fixtures.

Each seed is a *new* fact added under a **different existing surface form
of an entity already in the corpus**, in a section non-adjacent to the
fact it contradicts, carrying an incompatible value for the same
attribute. That is `bellwether`'s Class A shape — the class the matcher
scores 0/3 on — embedded in real-prose noise instead of a 1,260-word
fixture.

Surface-form pairs come from the corpus's own containment groups (the
same measurement that found 28 alias candidates on 2026-08-13), reduced
to those where co-reference is unambiguous. Which fact within a group
gets contradicted is chosen by script in fact-id order, not by reading.
The one judgment retained is the *replacement value*, which has to be
genuinely incompatible rather than merely different; that is recorded as
judgment rather than dressed up as mechanical.

The seed manifest stays out of this repo — it is manuscript detail.

## Scoring

**Seeded arm, recall.** Same rule `GRADING.md` fixes: a seed is FOUND
only if surfaced as a contradiction or a question naming both halves.
Surfaced-then-dismissed is a MISS. Denominator 4.

**Control arm, precision.** The corpus has no planted contradictions.
Two items are known and pre-declared as **hits, not false positives** —
the pipeline's adjudicated output already flagged them as `unverified`
and they are real observations about the text. Anything else asserted
with `kind: contradiction` counts as a false positive; anything raised as
`kind: question` counts as noise but not error, the same bucket
`GRADING.md` allows.

### Thresholds

| Outcome | Condition |
| --- | --- |
| **Holds up at scale** | seeded recall ≥ 3/4 **and** control asserts ≤ 3 items as `contradiction` |
| **Degrades** | seeded recall ≤ 2/4 **or** control asserts ≥ 8 items as `contradiction` |
| **Mixed** | anything between — diagnose which half failed before building, do not average |

"Holds up" makes options 2 and 3 buildable on evidence. "Degrades" gives
the deterministic prefilter a defined job and keeps option 1 alive.

**Cost** is recorded as measured subagent tokens per arm, with the
extrapolation stated: 330 facts is five sections, and a full novel is
several times that in a single call.

## Result, 2026-08-14 — holds up. 4/4 seeded, 0 false contradictions.

Run after the rule above was committed (`7083f45`), scored against it.

| Arm | Facts | Seeded recall | Items raised | Asserted `contradiction` | Tokens |
| --- | --- | --- | --- | --- | --- |
| Seeded | 334 | **4/4** | 5 | 4 (all four seeds) | 80,269 |
| Control | 330 | — | 1 | **0** | 83,213 |

Both threshold conditions for **holds up at scale** are met: recall ≥3/4,
and the control asserted zero contradictions. The honest expectation on
record — that it would degrade — was wrong.

**Recall did not decay from section scale to corpus scale.** All four
seeds were found, each as `kind: contradiction`, each citing both halves
by fact id, including the three requiring an alias join across
non-adjacent sections. Noise went up by a factor of four (84 facts → 330)
and recall stayed at 4/4.

**Precision is the surprise.** On 330 real facts with nothing planted,
the agent asserted **no** contradictions and raised exactly one question
— a district-naming inconsistency it correctly flagged as needing author
confirmation rather than asserting. For scale: the deterministic matcher
produced **69 collisions** on this same corpus, 87% of them
mixed-attribute artifacts. The comparison is 1 item to read versus 69.
The seeded arm raised the same district question and nothing else beyond
its four seeds, so the seeds did not perturb baseline behaviour.

### The result that complicates the story

**The agent missed both items the pipeline's adjudicator surfaced.**
`cont-001` and `cont-002` — pre-declared here as hits, not false
positives — were raised by neither arm, and both were reachable from the
facts alone (both concern an attribute recorded more than once). They are
subtler than the seeds: same entity, no alias join, a thing *described
twice* rather than asserted two incompatible ways.

So neither approach is a superset of the other. The agent join wins
decisively on cross-section, alias-bridging contradictions and on noise;
the deterministic pass surfaced two same-entity re-description issues the
agent walked past. **That argues for options 2 and 3, and against option
4** — the deterministic layer has a demonstrated job, it is just not the
job it currently has.

### Cost

~82K tokens per arm for 330 facts (an 88KB bundle), one call. Facts are
almost all of the payload. A full-length novel is order 2,000 facts,
roughly 6× this, which does not fit one call at this shape — so a real
implementation needs either the fact bundle trimmed or the corpus
partitioned. Note the partition cannot be by entity name: that is the
cut `MORNING_EDIT.md` measured dead at 1/4.

## What this will NOT prove

- **It measures the join, not extraction plus the join.** The seeds
  mutate *facts*, not prose. That is the right scope — `ComputeReconcile`
  is a fact-level operation and the question is whether an agent beats it
  at fact-level joining — but the result says nothing about whether
  extraction would have produced those facts from prose in the first
  place.
- **One corpus, one model, one prompt.** A prompt written to ask for
  contradictions is not a product; a real implementation would carry the
  brief, the author's declared aliases, and prior decisions.
- **Nothing about cost at true novel scale**, which is extrapolated here,
  not measured.

---

# Compression arm — grading rule fixed 2026-08-14, BEFORE any output was seen

Follow-up to the run above, same discipline, same corpus, same four
seeds, same thresholds. Only the *representation* changes.

## Why

The run above settled accuracy and moved the constraint to cost: ~82K
tokens for 330 facts, against a novel's order 2,000. Partitioning the
corpus is the obvious cut and is already dead three ways — by entity
name (measured 1/4 in `MORNING_EDIT.md`), by multi-section entities (one
seed's earlier surface form appears in a single section, so that filter
drops it), and by section window (one seed spans sections 01↔05, and any
window loses long-range joins, which is the class continuity exists to
catch).

That leaves making each fact cheaper rather than sending fewer of them.
The bundle above was **88,192 bytes for 330 facts — 267 bytes per fact**,
most of it JSON scaffolding, repeated key names, and fields the join may
never have used.

## What changes

One line per fact, `id | entity | attribute | value`, with a compact id
(`s{section}f{number}`) carrying the section. **Dropped:**
`entity_type`, `source`, `confidence`, and all JSON structure.

Everything else is held constant: same 330 facts, same 4 seeds, same
prompt wording, same two arms, same scoring rule and thresholds as the
run above.

## Predictions, recorded so they can be wrong

1. **Compression is recall-neutral** — 4/4 retained. The seeds are
   contradictions of *value* between two entity surface forms; none of
   the dropped fields carries information needed to make that join.
2. **`confidence` is the riskiest drop for precision.** An inferred fact
   conflicting with an explicit one is weaker evidence than two explicit
   facts conflicting, and without the field the agent cannot weigh that.
   If the control arm's asserted-contradiction count rises above zero,
   this is the first place to look.
3. **Bundle drops to roughly 80 bytes/fact**, making order 2,000 facts
   about 160KB — a single call at novel scale.

## Scoring

Identical to the run above, including the thresholds table and the
pre-declaration that `cont-001`/`cont-002` are hits rather than false
positives. Recorded per arm: seeded recall out of 4, items raised,
items asserted as `contradiction`, bundle bytes, measured tokens.

A recall drop here does not reopen the accuracy question settled above —
it localizes which fields the join was actually using.

## Compression result, 2026-08-14 — 4/4 retained at 86 bytes/fact

Run after the rule above was committed (`984f972`).

| Arm | Bundle | B/fact | Seeded recall | Items | Asserted `contradiction` | Tokens |
| --- | --- | --- | --- | --- | --- | --- |
| Seeded, JSON | 89,290 B | 267 | 4/4 | 5 | 4 | 80,269 |
| Seeded, compact | 28,902 B | **86** | **4/4** | 4 | 4 | **45,347** |
| Control, JSON | 88,192 B | 267 | — | 1 | 0 | 83,213 |
| Control, compact | 28,500 B | **86** | — | 1 | **0** | **56,070** |

**All three predictions held.** Compression is recall-neutral (4/4
retained, every seed still `kind: contradiction`, still citing both
halves). Precision is unchanged — the control asserted zero
contradictions and raised exactly one question, so dropping `confidence`
cost nothing measurable here, contrary to the risk flagged in prediction
2. Bundle fell to **86 bytes/fact against a predicted ~80**, a **68%
reduction**.

**Cost fell further than the bundle did**: 80,269 → 45,347 tokens on the
seeded arm, a 44% drop, more than the bundle's share alone would explain.
Less scaffolding to read appears to mean less work done per fact, not
merely fewer input tokens.

### What this makes possible

At 86 bytes/fact, an order-2,000-fact novel is a **~172KB bundle**, which
fits a single call. Whole-corpus agent join at novel scale is viable
without partitioning — which matters because every partitioning scheme
examined is dead: by entity name (1/4 measured), by multi-section entity,
and by section window, each for the same reason.

### Two observations, neither scored

- **The control arm raised a different question in each representation** —
  a district-naming question from the JSON bundle, an entity-identity
  question from the compact one. Both are legitimate, both correctly
  hedged as `question` rather than asserted. But it means the single
  control item is not a *stable* finding, and one run per arm cannot
  distinguish "reliably finds one real thing" from "surfaces one of
  several plausible things at random". Variance is unmeasured here.
- **The compact seeded arm dropped the district question** the JSON arm
  raised, reporting only its four seeds. Consistent with the above.

### Still not proven

Everything in "What this will NOT prove" above still applies unchanged —
this measures the join and not extraction, on one corpus, one model, one
prompt, with a single run per arm. The novel-scale figure is an
extrapolation from 330 facts, not a measurement at 2,000.

---

# Variance arm — grading rule fixed 2026-08-14, BEFORE any output was seen

## Why

Every number in this file and in `MORNING_EDIT.md` is **n=1 per
condition**. Recall has come back 4/4 three times, across three different
input shapes, which is reassuring but not the same as knowing the spread.
And one instability is already visible: the control arm raised a
*different* question in the JSON representation than in the compact one,
so at least one reported quantity is known to move between runs.

Building the hybrid on a number means knowing whether the number is 4/4
or 4/4-most-of-the-time.

## Method

Re-run the **compact** arms — the representation the build would use —
holding everything constant: same two bundles, same prompt wording, same
model, same instructions. Only the run differs.

**n=5 per arm**, counting the compact run already recorded above as run 1
of 5, since it was produced under identical conditions. Four further runs
per arm.

Nothing about the corpus, the seeds, or the prompt changes. This measures
run-to-run variance and nothing else.

## What gets recorded

Per arm, across the five runs:

- **Seeded recall** per run, and the distribution.
- **Asserted-`contradiction` count** per run.
- **Non-seed items** raised, by subject, so the union and the
  intersection across runs can be compared. This is the number that
  settles the open observation above: an empty intersection means the
  single control item is noise that happens to be plausible; a stable
  core means it is a real finding the agent reliably reaches.

## Thresholds

| Outcome | Condition |
| --- | --- |
| **Stable enough to build on** | every run ≥ 3/4 recall, **and** every control run asserts ≤ 3 as `contradiction` |
| **Unstable** | any run ≤ 2/4 recall, **or** any control run asserts ≥ 8 |
| **Mixed** | anything between — report the spread, do not average it into a single figure |

**Prediction, recorded so it can be wrong:** recall is stable at 4/4
across all five seeded runs (the seeds are flat contradictions of value
between two surface forms, not marginal judgment calls), while the
non-seed items are **not** stable — the intersection across the five
control runs will be smaller than the union, and may be empty. If that
holds, the honest way to describe precision is "asserts nothing false"
rather than "finds one real thing".

## What this will NOT prove

Five runs is a spread, not a distribution. One model, one prompt, one
corpus, unchanged. It says nothing about variance under a different
model, a re-worded prompt, or a manuscript with different noise.

## Variance result, 2026-08-14 — recall is stable, the questions are not

Run after the rule above was committed (`8327b9c`). Five runs per arm,
compact representation, everything else held constant. Run 1 of each arm
is the compression run already recorded.

| Arm | Run 1 | 2 | 3 | 4 | 5 |
| --- | --- | --- | --- | --- | --- |
| Seeded, recall | 4/4 | 4/4 | 4/4 | 4/4 | 4/4 |
| Seeded, asserted `contradiction` | 4 | 4 | 4 | 4 | 4 |
| Control, asserted `contradiction` | **0** | **0** | **0** | **0** | **0** |
| Control, items raised | 1 | 1 | 1 | 2 | 2 |

**Stable enough to build on**, by both threshold conditions: every run
4/4, every control run zero asserted contradictions. Tokens: seeded mean
45,678 (range 42,958–47,888), control mean 52,043 (48,168–56,070).

**Both predictions held.** Recall is stable at 4/4 across all five —
these seeds are flat contradictions of value, not marginal judgment
calls, and nothing about them is borderline. And the non-seed items are
**not** stable: seven items across five control runs, and the
intersection across all five is **empty**.

### But it is not random noise — it samples from a real pool

The rotating items are not arbitrary. Across the five control runs they
cover **four distinct subjects**, two of which recur:

| Subject | Runs raising it |
| --- | --- |
| An entity's identity — whether two names denote the same figure | 1, 3, 5 |
| A place named three ways across two sections | 2, 4 |
| A demographic term used one way once and another way elsewhere | 4 |
| A character's sense reporting nothing where something is shown | 5 |

Every one is a legitimate question about the corpus, every one was
correctly hedged as `question` rather than asserted, and one of them
corresponds to a real error already confirmed against this manuscript
independently. The agent is sampling one or two items from a pool of
genuine ambiguities, not inventing different things each time.

### What that changes

- **Precision should be described as "asserts nothing false", not "finds
  one real thing".** Zero false contradictions in five runs is a strong,
  stable claim. "It finds the district problem" is not — it found it
  twice in five.
- **A single run under-reports.** If the product is the question set, one
  pass returns a sample of it. That is a design input for the hybrid:
  either accept partial recall of the soft-question class per run, or
  run the join more than once, or prompt for exhaustiveness explicitly
  and re-measure. **None of those is tested here.**
- **The hard class and the soft class behave differently.** Flat
  contradictions are reproduced perfectly; judgment-call ambiguities are
  sampled. Any future number should say which class it is measuring.

### Still not proven

Five runs is a spread, not a distribution, and the sampling behaviour
above is characterized from seven items — too few to describe the pool's
size or the per-run sampling rate. One model, one prompt, one corpus.
Nothing here measures variance under a re-worded prompt, which is the
most likely thing to change the soft-question behaviour.

---

# Full-flow run, 2026-08-15 — and a correction to the section above

First run of the *complete* continuity flow — reconcile, adjudicator,
bundle, joiner, merge, validate — through the shipped commands and the
shipped agent files. Two manuscripts.

## bellwether: 4/4, both id ranges coexisting

| Stage | Output |
| --- | --- |
| `canon reconcile` | 4 collisions |
| adjudicator | 2 `unverified` (`cont-001`, `cont-002`), 2 dismissed as general-vs-specific phrasing |
| `canon bundle` | 111 facts |
| joiner | 4 findings (3 `contradiction`, 1 `unverified`) |
| `canon merge` | 4 added, 0 duplicate → `cont-501`–`cont-504` |
| `validate` | exit 0 |

**Planted recall 4/4**, with the adjudicator's `cont-0NN` and the
joiner's `cont-5NN` sitting in one validated file — the coexistence that
until now had only ever been unit-tested.

## Real corpus: the result that corrects this document

| Stage | Old pipeline (2026-08-13) | Now |
| --- | --- | --- |
| Collisions to adjudicate | 69 | **9** |
| Adjudicator kept | 2 `unverified` | **0** |
| Joiner | — | 1 `unverified` |
| **Author-facing total** | 2 | **1** |

**The adjudicator dismissed all nine collisions — including the two it
had previously kept.** `cont-001` and `cont-002` are the items this
document cites above as proof that "neither approach subsumes the other",
and a fresh run of the same agent on the same data judged both to be
general-vs-specific phrasing rather than conflicts.

**So that claim was weaker than it was stated.** It rested on one
adjudicator run, and adjudicator output turns out to vary run-to-run in
the same way the joiner's soft findings do — which the variance arm
measured for the joiner and nobody measured for the adjudicator. The
honest version: the deterministic pass surfaces a *class* the joiner did
not raise (same entity, same attribute, described twice), and whether any
given member of that class survives judgment is not stable.

What this does **not** change: the narrowing. That rested on 87%
artifacts and a facts^1.4 growth curve, both independent of this. Nine
collisions rather than 69 is the same result either way.

What it does change: **option 4 (drop the matcher) is no longer clearly
ruled out.** It should be reopened as an open question rather than
treated as settled, and settling it needs the adjudicator's own variance
measured the way the joiner's was.

## Two things that worked, worth recording

- **Brief-based suppression already works.** Both agents read the brief's
  deliberate-choices list, recognized an error the author had already
  recorded there as known, and declined to re-report it. That is the
  mechanism TODO.md's "author-declared alias table" proposes to extend —
  it is not missing, it is narrower than proposed.
- **The joiner's single finding was the kind an author can use**: not
  just "these two names may be the same thing", but a warning that a fix
  already pending on one of them, applied broadly, would erase a
  deliberate choice elsewhere. That reasoning is not available to a
  string matcher at any threshold.
