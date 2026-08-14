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
