# bellwether — the **unspent** fixture the recall fix failed

Four sections of contemporary literary fiction (~1,260 words), written
2026-08-12 to answer the question `TODO.md` had carried open since the
recall fix shipped: **does the fix generalize, or was it tuned to
`saltmarsh`?**

Answer: it does not generalize. **0 of 4 planted contradictions were
found.**

## Why this fixture is worth more than saltmarsh

`saltmarsh` is *spent* — the fix was tuned against it, so it catches
regressions and proves nothing about generalization. This one is not.
**Nothing has been tuned against it**, and that is precisely its value.
Keep it that way: if you fix the matcher and then adjust this fixture's
expectations to match the new output, you have converted it into another
spent fixture and thrown away the only unspent evidence in the repo.

Blindness was enforced, not assumed. The manuscript was written by an
agent given no repo path, no redliner vocabulary, and an explicit
instruction to read no file on the machine. A scan of the resulting
fixture for redliner terms (`canon`, `extractor`, `collision`,
`continuity`, `saltmarsh`) found none.

## What it plants

`GROUND_TRUTH.md` is authoritative — written by the authoring agent,
before any pipeline run. Four contradictions, each split across
**non-adjacent** sections with deliberate naming variation, plus three
reconcilable decoys.

## What the run showed (2026-08-12)

**Two failure classes, 3 + 1. Don't collapse them into one bug.**

**Class A — entity-name variation (3 of 4).** Both halves extracted
correctly; only the matcher failed.

| Planted | Section A | Section B |
| --- | --- | --- |
| absence length | `Renata Sowa.years_since_last_visit = nineteen winters` | `Ren.years_away_from_town = twenty-three years` |
| father's age | `Emil.age_at_death = eighty-one` | `Ren's father.age_at_death = two months short of seventy-seven` |
| hull length | `Lyman.length = twenty-six feet` | `the boat.length = thirty-one feet` |

Rows 2 and 3 are the sharpest evidence in the repo: **the attribute
matches exactly** and they still never meet.

Root cause, verified in source: `ComputeReconcile` partitions facts with
`byEntity[normEntity(rec.Entity)]` (`go/internal/cli/canon.go:455-461`),
and `linkByAttribute` (`canon.go:309`) only runs *inside* one partition.
`normEntity` (`canon.go:273`) lowercases and strips one leading article,
nothing more. The 2026-08-12 fix made **attribute** names fuzzy and left
**entity** names exact-match — and saltmarsh's failure varied the
attribute (`duration_not_working` vs `stopped_duration`) on an entity
that differed only by an article (`tide clock` vs `the tide clock`),
which is the one shape that fix handles.

**Class B — propositional mismatch (1 of 4).** `Kaja.attended_deathbed =
"sat with him all four days and held his hand the last four hours"` vs
`the two sisters.presence_at_death = "neither present"`. Same underlying
claim, different entity *and* different attribute sharing no significant
token. **No amount of name matching collides these** — resolving `Kaja`
to `the two sisters` would not do it. Don't let a future entity-matching
fix take credit for this class.

**Precision failed too.** Of 16 collisions reported, **15 mix two
different attributes** and 6 are entirely within a single section.
Exactly 1 is a clean same-attribute cross-section pair, and it is not one
of the planted four.

> **Correction.** An earlier version of this README called single-section
> collisions structurally invalid. They are not — saltmarsh's `cont-002`
> is a real `unverified` finding with both fact ids in `section_03`. A
> manuscript can contradict itself within one section. The 6 here are
> noise on their merits, not by construction. `Emil` alone produced 6 spurious collisions:
`activity_at_death`, `age_at_death`, `death_date`, and `place_of_death`
all share the token `death`, so pairwise linking emits all C(4,2)
combinations, yielding items like `Emil.age_at_death: ['eighty-one',
'hospice']`. That is quadratic in attributes-per-entity, so it worsens on
a full novel.

## The adjudicator saw three of them and could not report them

The most actionable finding here, and an unexpected one.

Adjudication of the 16 collisions produced **1 `unverified` item and 0
contradictions**. But the adjudicator's reply to the coordinator named
all three Class A contradictions in prose — it went looking despite being
told *"Do not go looking for more contradictions"*
(`agents/fiction-continuity-adjudicator.md:11-16`), and it found them.

It could not write them down. `fact_ids` is a required field, and a
contradiction the matcher never paired has no fact-id pair to cite. In
its own words: *"no collision was raised, and the schema needs fact
IDs."* The one component that noticed was blocked by the output contract,
and `SKILL.md:293-298` has the coordinator summarize from
`continuity.json` — where they do not appear.

It also **misdiagnosed the cause**, on two independent runs: it called
them "genuine extraction gaps worth fixing upstream." Every one of those
facts was extracted correctly (see `observations/`). The adjudicator
never sees `observations/`, only `collisions.json`, so
absent-from-collisions looks identical to absent-from-extraction. Anyone
reading only that report would go fix the wrong component.

## Contents

| Path | What it is |
| --- | --- |
| `section_0{1..4}.txt` | the manuscript |
| `GROUND_TRUTH.md` | the planted contradictions and decoys, authoritative |
| `.redliner/brief.md` | a hand-written brief in the shape `/redliner:intake` produces |
| `.redliner/state.json` | `manuscript_dir` relativized to `bellwether`, matching `happy` and `saltmarsh` |
| `.redliner/canon/observations/*.json` | 111 facts across 30 entities — lets `canon reconcile` re-run deterministically with **no model calls** |
| `expected/collisions.json` | what the current matcher finds — **a record of failure, not a target**. Was 16 when the run above was measured; **19 since the `protect-exact` fix** (2026-08-12), which surfaced three clean same-attribute collisions that had been hidden inside contaminated merged supersets. Recall is unchanged at 0/4 — `protect-exact` does not touch the entity partition. |
| `expected/planted_pairs.json` | **the actual assertion.** Never regenerate this |
| `expected/continuity.json` | what the adjudicator kept from those 16: one `unverified` item, zero contradictions |

## Using it

```
cp -r go/harness/fixtures/bellwether /tmp/bw && redliner canon reconcile /tmp/bw
```

The check that matters is not a diff against `expected/collisions.json` —
that file records broken output. It is: **do the four pairs appear as
collisions?** `expected/planted_pairs.json` holds them as fact-id pairs
so this is mechanical rather than a prose instruction:

```python
import json

cols = json.load(open("/tmp/bw/.redliner/canon/collisions.json"))["collisions"]
groups = [{f["id"] for f in c["facts"]} for c in cols]
for p in json.load(open("expected/planted_pairs.json"))["pairs"]:
    a, b = p["fact_ids"]
    print(p["label"], p["failure_class"], any(a in g and b in g for g in groups))
```

Today all four print `False`. A matcher fix works when Class A's three
print `True`; Class B is a separate, harder problem and should not be
counted toward an entity-matching fix.

Do **not** edit those ids to match new output. Fixing the matcher should
change what the script prints, not what the fixture asserts.

## Caveats on the evidence

- The prose is **synthetic and written to contain contradictions**. It
  tests whether the matcher survives naming variation it has never seen.
  It does not test recall on prose written with no contradictions in
  mind, where signal-to-noise is far worse.
- Two data points is not a generalization curve.
- **Extraction fidelity note:** the first `section_02` extraction emitted
  4 excerpts (of 27) that were not verbatim substrings — three
  ellipsis-joined across separated phrases — and `validate` correctly
  rejected the file, which in a real run would halt `continuity` at step
  4. The committed `section_02.json` is a re-extraction using an added
  instruction the shipped prompt lacks: *"every `excerpt` must appear
  verbatim in the section text — a contiguous substring... do not join
  two separated phrases with an ellipsis."* Sections 1, 3, and 4 are
  pure shipped-prompt output. The 0/4 does not depend on any of this;
  none of the planted pairs involves a rejected fact.

## Deliberately NOT in the differential harness

Like `saltmarsh`, this is **not** in `capture_baseline.py`'s
`FIXTURE_SCRIPTS` and has no goldens. The harness proves the Go port
matches the frozen Python oracle; this tests a behavioural property of
the current matcher. Conflating the two would mean regenerating goldens
whenever these expectations change — and here the expectations are
*supposed* to change, once the matcher is fixed.
