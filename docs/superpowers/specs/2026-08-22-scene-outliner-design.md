# Scene outliner — design

**Date:** 2026-08-22
**Status:** approved, not yet implemented

## The problem

An author working on a long manuscript loses the shape of it. Deciding
whether a scene can move, or should be cut, currently means rereading
the book. There is no compact view of what each scene is *for*.

Redliner has two layers that both read the manuscript — developmental
(judges structure) and continuity (records facts) — but neither answers
"what happens, in order, and does each piece change anything."

## What this adds

A third layer that **records** the manuscript's scene structure: one
row per scene, holding what the driving character was trying to achieve,
what opposed it, and what changed as a result.

Two consumers, both first-class:

1. **The author.** A rendered `Outline.md` they read to decide what
   moves and what gets cut.
2. **The developmental editor.** The outline is handed to it as a
   structural spine, alongside the prose.

### Why goal / conflict / outcome specifically

These three fields are chosen for the stated use case, not out of
convention:

- **Outcome** is what makes cuttability visible. A scene whose outcome
  is "nothing changed" is the cut candidate. A summary-only outline
  cannot show this.
- **Goal plus outcome across consecutive scenes** is what makes
  reorderability visible. If scene N's outcome is not a precondition of
  scene N+1, the two are movable.

A prettier prose summary would be useless for both jobs.

## Shape: it is the continuity layer, not a phase

The outliner is modeled on `continuity`, deliberately:

- **Not phase-gated.** Recording is judgment-free, so it is safe any
  time after intake — including on a partial draft nowhere near a
  developmental pass.
- **Tracks its own staleness per section** by SHA-256, independent of
  the developmental round counter. Re-running is idempotent and cheap:
  unchanged sections are skipped, changed ones re-recorded, deleted ones
  have their rows dropped. This is what makes "run it repeatedly as you
  write" real rather than a full re-read each time.
- **One agent per section**, joined deterministically.

### Pure recorder

The outliner has the same enforced purity as
`*-continuity-extractor`: no `note`, no `severity`, no `concern`, and
the validator rejects extra keys. If a scene's outcome is "nothing
changed," the recorder writes that flatly and the developmental pass
decides what it means.

This is the split that keeps the layer from becoming a second
developmental editor. It is the same recorder/judge boundary the
continuity layer already enforces between extractor and adjudicator.

## Storage

| Path | Audience |
|---|---|
| `.redliner/outline/sections/<stem>.json` | tool — per-section rows |
| `.redliner/outline/outline.json` | tool — deterministic join, section order |
| `<manuscript_dir>/Outline.md` | **author** — rendered view |

The Markdown file sits in the manuscript folder, not inside
`.redliner/`, following the rule `run/SKILL.md` already enforces for
editorial letters: hidden storage for machine state, visible files for
anything a human reads. That rule has a real user report attached — *"I
don't know where the .redliner directory you linked is."* Section
discovery globs `section_*`, so `Outline.md` is never mistaken for
manuscript text.

The join rebuilds `outline.json` from **every** current per-section
file, not just the ones re-recorded this run — same as `canon
reconcile`.

## Schema

Per-section file:

```json
{
  "section": "section_03",
  "section_sha256": "<the hash the recorder was given>",
  "leaves_open": "Whether the guard reports her. (serial-fiction only)",
  "scenes": [
    {
      "order": 1,
      "pov": "Mira",
      "anchor": "The gate was already open when she",
      "goal": "Get inside the compound before the shift change.",
      "conflict": "The guard rotation ran early; she has no cover story.",
      "outcome": "She gets in, but is seen — the guard now knows her face."
    }
  ]
}
```

Domain-configured section-level fields (currently only
serial-fiction's `leaves_open`) sit at the top level of the file,
outside the `scenes` array. Domains without them omit the key entirely.

**Rows are scene-level, not section-level.** The unit redliner tracks is
`section`, but a section usually holds several scenes, and the author's
use case — moving and cutting scenes — is a sub-section operation. A
section-level row cannot express "cut the middle scene." One file per
section holding an array of scene rows keeps the SHA-skip working
per-file while giving scene granularity.

**`order` is positional, not a durable id.** Scene boundaries are the
recorder's judgment and can shift between runs even on unchanged text.
Nothing downstream may assume `section_03` scene 2 denotes the same
scene as it did last week.

**`anchor` is the scene's first few words, verbatim.** It is how the
author locates a scene in the prose when ids cannot be trusted, and it
is cheap for the recorder to emit.

## Per-domain configuration

An **optional** `outline` block in `domain.json` declares the row
fields. It drives both the generated agent prompt and the validator.

- **fiction** — `goal`, `conflict`, `outcome`.
- **serial-fiction** — those three, plus one *section-level* field,
  `leaves_open`: what question the chapter ends on. This is a recording,
  not a rating: not "this hook is weak" but "the chapter ends with the
  guard having seen her face, unresolved." The domain already treats
  `chapter_hook` and `episodic_pacing` as first-class, and cutting a
  scene from a serial has a consequence a novel does not have — it can
  gut the beat the installment ends on. A novel-shaped outline would let
  the author walk into exactly that mistake.
- **design-doc** — block absent. Nothing generated, no subcommand. A
  design-doc outline (claim / objection / conclusion per section) is a
  coherent idea later; it is out of scope here.

The block must be genuinely optional — `domain_loader.go` and the
validators handle its absence rather than erroring.

## The published boundary

Serial fiction has a constraint novels do not: once a chapter goes out,
it is fixed. Authors do not revise published installments except when
re-editing for a book version. A scene above that line cannot be moved
or cut, which makes the boundary the single most load-bearing fact in a
view whose whole purpose is deciding what to move and what to cut.

`state.json` gains an optional `published_through` field naming the last
published section stem (e.g. `"section_11"`), set at intake for
serial-fiction manuscripts and changeable with a command as chapters go
out. Absent or null means nothing is published — the correct default for
`fiction`, and for a serial being drafted before launch.

**`Outline.md` renders the boundary as a visible line**, with everything
above it marked as shipped. That is the whole of this spec's use of the
field: a rendering cue, not a behavior change. Recording is unaffected —
a published chapter is already never re-recorded, because SHA-skip means
an unchanged section is skipped whether or not anyone declared it
frozen.

The field is deliberately a *section* boundary rather than a scene one.
Publication happens per installment; there is no such thing as half a
chapter being live.

## Cost model

Worth stating plainly, because "re-run the outline" sounds expensive and
is not.

A re-run costs: a deterministic staleness check (free), **one agent call
per section whose hash changed**, then a deterministic join and render
(free). Writing chapter 12 and re-running is one call; chapters 1–11 are
never opened. A full re-record happens only if every section's hash
changed — a global find-replace or a reformat, not ordinary writing.

So "outline after every chapter" and "outline after five chapters" cost
the same in total; they differ only in when the author sees the view.
This is the reason the layer is built on the continuity layer's
staleness machinery rather than as a one-shot pass, and it is what makes
the "run it repeatedly as you write" workflow viable rather than
aspirational.

Inside `assess`, the developmental editor additionally reads
`outline.json`. That is a compact structural file, and it replaces work
the editor was already doing by re-deriving scene structure from prose.

## Commands and wiring

Two entry points, matching the continuity precedent:

### `/redliner:run outline`

Standalone. Safe any time after intake.

1. Determine which sections need re-recording (SHA comparison), plus
   current hashes and orphaned files.
2. Delete orphaned per-section files (a cut section's scenes must not
   still appear in the outline).
3. For each stale section, Task `redliner:<domain>-outliner` with the
   manuscript directory, the section path, its current hash, and the
   output path.
4. Validate.
5. Join deterministically into `outline.json`, render `Outline.md`.
6. Report: scene count, and which sections were re-recorded.

### Inside `assess`

**Outline refresh happens inside the assess flow — not as a
precondition the author is trusted to have satisfied.** It is
hash-driven and idempotent, so on an unchanged manuscript it costs
nearly nothing, and on a changed one it is the difference between a
spine that matches the prose and one that does not.

**Its exact position matters, because refresh overwrites
`outline.json`.** The existing flow is: step 1 moves to the
developmental phase and increments the round counter; step 2 archives
the previous round *before* clearing anything; step 3 tasks the
developmental editor. The outline refresh goes **after step 2's archive
and before step 3** — never before step 2.

Put it earlier and round N-1's recorded scene structure is destroyed.
Unlike `continuity.json`, which is deterministically rebuildable from
per-section observations, `outline.json` is a join of agent output: once
overwritten without an archive, that round's outline is gone, and the
deferred diff tool inherits a hole. This is verbatim the failure step 2
exists to prevent — *"every pass rewrites findings in place, so clearing
without archiving left nothing to compare a later round against."*

This mirrors the `--snapshot-after` lesson recorded in `run/SKILL.md`:
splitting two ordered operations means "one order works and the other
silently disables the flag, with identical output either way." A stale
outline fails exactly that way — the developmental output looks fine and
is reasoning from the wrong structure. Putting the refresh inside the
flow removes the ordering the author could get wrong.

### The developmental editor reads the outline alongside the prose

Never instead of it. `TODO.md`'s B1 decision records that the agent
files are the most carefully engineered part of this system, and
`developmental-editor.md`'s craft is built on reading actual prose.
Substituting an outline to save context would hollow that out. The
outline is a spine the editor reasons *with* — it removes the need to
re-derive scene structure from prose every round, and makes arc-level
questions legible.

## Archiving

The outline is archived per round alongside developmental findings.
`rounds.go` gains an `outline` archive kind and a case archiving
`.redliner/outline/outline.json` into
`.redliner/rounds/outline-round<N>/`.

**It archives at both points developmental findings do** — in assess
step 2 (before the refresh overwrites it) and again in the closing step.
That is not redundant: the closing archive preserves each completed
round, and the step 2 archive is the safety net for a round that ended
without one. `freeArchiveDir` already suffixes `.2`, `.3` for exactly
this case.

**`outline` is an archive kind, not a pass kind.** `passKinds` is
currently shared by `rounds archive` and `state pass`, and that has to
split: `state pass` records "the author ran this pass," which drives
`status`. The outline refreshes automatically inside every assess, so
recording it there would make `status` report "outline: run" permanently
— a constant, and therefore no signal at all. The informative report is
the per-section staleness `status` already shows for continuity. So
`rounds.go` validates against a new `archiveKinds` list (developmental,
line, continuity, outline) while `state pass` keeps `passKinds`
unchanged, and `state pass outline` stays deliberately unavailable.

Archives are numbered by the developmental round counter even though
the layer is not phase-gated. That is the continuity precedent exactly —
`continuity` archives under `DevelopmentalRound` too — and it is what
makes an outline archive line up with the developmental findings it
should be read beside.

`assess` archives the outline where it already archives continuity: in
the closing step, alongside `rounds archive <dir> developmental` and
`... continuity`.

An outline diff across rounds is the clearest available answer to "what
did my revision actually change," which is why the archive is worth
having even before a tool reads it.

## Implementation constraints

- `domain_loader.go` and validators must handle the `outline` block's
  absence cleanly, so `design-doc` opts out.
- `validate_findings` walks everything under `.redliner/`. The new file
  types need a schema there, or they will error or be silently skipped.
- Adding the block changes `domain show` output:
  `go/harness/golden/happy/02_domain_show_fiction.json` and its siblings
  need regenerating.
- `new-domain` generates concrete per-domain agent files (the B1
  decision). It needs a seventh template, plus a regeneration story for
  existing domains.
- `state.json` gains optional `published_through`; `project_state.go`
  and its validator must accept its absence (every existing manuscript,
  and every `fiction` one, lacks it). `intake` asks for it on
  serial-fiction manuscripts only, and a command sets it as chapters
  ship.
- `run/SKILL.md` needs the new subcommand section, the assess-flow
  insertion between steps 2 and 3, and `status` should report whether any
  sections need re-recording — as it already does for continuity.
- `Outline.md` in the manuscript folder is confirmed safe:
  `schemas.SectionFiles` globs `section_*` with a `.txt`/`.md`
  extension, non-recursively, so nothing else in the directory is
  discovered as manuscript text.
- MCP surface: the deterministic operations (staleness check, join)
  need tool equivalents alongside the CLI ones, per the "use whichever
  concrete tool this session has" contract in `run/SKILL.md`.

## Deferred (TODO, not this spec)

**An outline diff tool.** Given two archived outlines, show what moved,
what was cut, what was added, and where a scene's outcome changed. The
archive above makes this possible; the tool itself is future work.

Open sub-questions it would need to answer: with `order` positional and
ids non-durable, matching scenes across rounds has to be done by
content similarity or `anchor`, not by position — a naive positional
diff would report every scene after an insertion as changed.

**Teaching the developmental pass about the locked prefix.** This spec
adds `published_through` and uses it only to draw a line in
`Outline.md`. The larger use is that
`serial-fiction-developmental-editor` currently reasons about
"everything released so far" — the phrase appears throughout its prompt
— with no way to know which chapters those are, so it can emit findings
against text the author cannot change.

The actionable form of a finding about a published chapter is different,
not absent: not "restructure chapter 4's midpoint" but "chapter 4 left
this unpaid — pay it in chapter 13." Same observation, forward-directed
advice.

That implies a mode rather than a filter: **serial mode** (locked
prefix, findings must be actionable forward) versus **book-version
re-edit** (nothing locked, restructure anything). Which one applies is a
fact only the author has.

Scope: the agent prompt, the findings schema's `scope` field, and
intake. Its own spec, not this one.
