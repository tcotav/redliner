---
name: run
description: Runs layered editing on a manuscript (fiction, design docs, or any domain edaitor has been configured for) — developmental assessment, revision support, re-checks, then line editing. Use when the author asks to edit, assess, or review a manuscript, to work or resolve a finding, or types /edaitor:run.
---

# edaitor:run

Phase-aware editing pipeline. Subcommands:

| Command | Does |
|---|---|
| `/edaitor:run status` | Where this manuscript stands |
| `/edaitor:run assess` | Developmental pass (round 1, or a fresh full read) |
| `/edaitor:run work <id>` | Talk through one finding and how to revise it |
| `/edaitor:run resolve <id>` | Mark a finding addressed (author's claim) |
| `/edaitor:run recheck` | Re-read after revision; verify claims, find new issues |
| `/edaitor:run line` | Line-editing phase (gated — see below) |
| `/edaitor:run continuity` | Extract facts, find collisions, adjudicate — see below |

Default with no subcommand: run `status` and recommend the next step.

Manuscript directory comes from the argument, or defaults to the current
directory — the intended usage is `cd` into the author's manuscript
directory, then run `/edaitor:run`. State lives in
`<manuscript_dir>/.edaitor/`.

The scripts below (`edaitor_state.py`, `edaitor_canon.py`,
`validate_findings.py`) run as bare commands, no `python3` prefix — the
plugin's `bin/` directory is on the Bash tool's PATH while this plugin is
enabled, and each script is executable with its own shebang.

## Which subagent to Task

Agent files are named `agents/<domain>-<role>.md` and registered under
the plugin namespace as `edaitor:<domain>-<role>` (e.g.
`edaitor:fiction-developmental-editor`) — never a bare or undomained
name. Get `<domain>` from `edaitor_state.py status <manuscript_dir>`'s
`domain` field (fall back to `fiction` only if that key is somehow
absent from old state) and substitute it into every `edaitor:<role>`
reference below. So on a `design-doc` manuscript, "Task
`edaitor:<domain>-developmental-editor`" means Task
`edaitor:design-doc-developmental-editor`. This holds for all five
roles: `developmental-editor`, `line-editor`, `editorial-aggregator`,
`continuity-extractor`, `continuity-adjudicator`.

## Why phases are sequential

Developmental editing comes first and iterates until structure settles;
line editing comes after. This mirrors real editorial practice for a
concrete reason: **line-editing prose that structural work will cut or
rewrite is wasted effort**, and a combined pass can produce contradictory
advice about the same paragraph — recommending it be deleted while also
suggesting sentence-level rewrites for it.

Everything below enforces that ordering.

## Preconditions (all subcommands)

Run `edaitor_state.py status <manuscript_dir>`. If there's no state, or
no `.edaitor/brief.md`, stop and tell the author to run `/edaitor:intake`
first — every pass depends on the brief.

## `/edaitor:run assess`

1. `edaitor_state.py phase <manuscript_dir> developmental` (this
   increments the round counter).
2. Prepare `<manuscript_dir>/.edaitor/findings/`; clear stale files from
   a previous round.
3. Task the `edaitor:<domain>-developmental-editor` subagent with the manuscript directory,
   the round number, and output path `.edaitor/findings/developmental.json`.
4. `validate_findings.py <manuscript_dir>` — stop and report errors
   rather than aggregating bad data. (Takes the manuscript directory
   itself, not a findings/ or canon/ subpath — it checks everything
   under `<manuscript_dir>/.edaitor/` in one pass.)
5. Run the **continuity** steps below now — **before the snapshot in the
   next step**, not after. Continuity's `reconcile` diffs against
   whatever baseline is currently in `state.json` to decide
   `likely_unpropagated_revision`; if `snapshot` runs first, that
   baseline already matches the current text and the diff always comes
   back empty, silently disabling the flag. This only looks harmless on
   a first-ever assess (empty baseline either way); it breaks the moment
   `assess` is re-run later as "a fresh full read" with a real prior
   baseline. Don't reorder this.
6. `edaitor_state.py snapshot <manuscript_dir>` — records what the text
   looked like when assessed, so `recheck` can tell what changed.
7. Task `edaitor:<domain>-editorial-aggregator` for the **developmental** letter.
8. Validate again, then read and show the developmental letter, then the
   continuity summary from step 5.

Do **not** run line editing here, whatever the author asked for.

## `/edaitor:run work <id>`

Revision support — the conversational one. Do this **in the main session,
not via a subagent**: it's a back-and-forth, not a batch job.

1. Read the brief, the finding by id, and the sections it touches.
2. Talk it through with the author: why it's a problem, what options
   exist, what each would cost elsewhere in the book. Offer approaches
   rather than prescribing one; they're the author.
3. If they draft a fix, react to it honestly — including saying it
   doesn't solve the problem when it doesn't.
4. Don't edit the manuscript unless asked directly. Their book.

## `/edaitor:run resolve <id>`

Set that finding's `status` to `claimed` in `developmental.json` (not
`addressed` — the author's claim isn't verification). Confirm, and note
that `/edaitor:run recheck` will verify it.

## `/edaitor:run recheck`

1. `edaitor_state.py diff <manuscript_dir>` and branch on the verdict —
   this is deterministic, so trust it over any impression of how much
   changed:

   - **`unchanged`** — no section text differs from the last assessment.
     Any `claimed` findings can't be verified; say so plainly and stop.
     Something's off: either the revision wasn't saved, or the author
     marked things resolved without revising.
   - **`targeted`** — specific sections edited, none added or removed,
     no large swings. Task `edaitor:<domain>-developmental-editor` to verify the `claimed`
     findings against those sections and check whether the edits created
     new problems. Pass the existing findings file so ids carry forward.
   - **`restructured`** — sections added, removed, or heavily rewritten.
     A full re-read: task `edaitor:<domain>-developmental-editor` over the whole
     manuscript with the prior findings file. Findings the restructure
     invalidated should come back `stale`, not `addressed` — the author
     didn't fix them, the text moved. Tell the author which findings went
     stale and why; that's the case they can't assess themselves.

2. Validate.
3. Run the **continuity** steps below now — **before the snapshot in the
   next step, not after.** This ordering matters more here than anywhere
   else: revision is exactly when facts get out of sync (an edit in one
   section not yet propagated to another), and `likely_unpropagated_
   revision` is *the* signal continuity exists to surface on a recheck.
   That signal comes from diffing against the baseline still sitting in
   `state.json` from the last assessment — if `snapshot` (next step) runs
   first, the baseline already matches the current text, the diff comes
   back empty, and every collision looks like a standing issue instead of
   fresh fallout from this revision. `edaitor_canon.py stale` figures out
   which sections actually need re-extraction; a `targeted` developmental
   verdict usually means continuity only has one or two sections to redo,
   not the whole manuscript.
4. `edaitor_state.py snapshot <manuscript_dir>`, aggregate a fresh
   developmental letter, show it, then show the continuity summary from
   step 3.
5. Then say plainly whether structure looks settled enough for line
   editing — count open `major`/`critical` findings and give a real
   recommendation, not a hedge.

## `/edaitor:run line`

1. Read `developmental.json`. Count open findings at `major` or
   `critical`.
2. **If any exist, warn before proceeding** — name them and explain that
   line editing now risks polishing prose that structural revision will
   change. Then, if the author still wants to continue, continue. This is
   a soft gate by design: it's their manuscript and there are legitimate
   reasons to line-edit a section that's structurally settled even while
   other parts aren't.
3. `edaitor_state.py phase <manuscript_dir> line`.
4. For each section, task `edaitor:<domain>-line-editor` with the manuscript directory,
   that section's path, any `deferred_to_line` developmental findings for
   it, and output path `.edaitor/findings/line_<section_stem>.json`.
   Sequential keeps the transcript readable; parallel is fine too —
   sections share no state.
5. Validate.
6. Task `edaitor:<domain>-editorial-aggregator` for the **line** letter.
7. Validate, then read and show the letter.

## `/edaitor:run continuity`

Callable directly, and also what `assess` and `recheck` run at the end
of their own flow (see above) — this section is the one definition both
refer to.

Unlike the other two layers, this one is not phase-gated: extraction is
judgment-free (it doesn't need the developmental pass to have run first)
and it tracks its own staleness per section, independent of the
developmental round counter. It's safe to run any time after intake.

1. `edaitor_canon.py stale <manuscript_dir>`. Its JSON output drives
   everything below:
   - `needs_extraction` — sections to (re-)extract this run.
   - `current_hashes` — each of those sections' current SHA-256, keyed by
     stem. The extractor needs this exact value; don't compute it
     yourself or reuse a stale one from `state.json`.
   - `orphaned_observations` — observation files whose section no longer
     exists (deleted or renamed). Delete
     `.edaitor/canon/observations/<stem>.json` for each before
     reconciling — a cut section's facts shouldn't still count toward
     canon or collisions.
2. If `needs_extraction` is empty, skip to step 5 — nothing changed
   since the last extraction.
3. For each section in `needs_extraction`, Task the
   `edaitor:<domain>-continuity-extractor` subagent with: the manuscript
   directory, that section's file path, its hash from `current_hashes`,
   and output path `.edaitor/canon/observations/<section_stem>.json`.
   Sections share no state — parallel is fine, sequential keeps the
   transcript readable.
4. `validate_findings.py <manuscript_dir>` — stop and report errors
   rather than reconciling from bad extraction data.
5. `edaitor_canon.py reconcile <manuscript_dir>` — deterministically
   rebuilds `.edaitor/canon/canon.json` (merged facts) and
   `.edaitor/canon/collisions.json` (entity+attribute pairs with more
   than one asserted value) from every current observations file, not
   just the ones just (re-)extracted.
6. Read `collisions.json`.
   - **Empty `collisions` list** — write
     `.edaitor/canon/continuity.json` as `{"contradictions": []}`
     yourself. There's nothing to adjudicate, so don't spend a Task call
     on it.
   - **Non-empty** — Task the `edaitor:<domain>-continuity-adjudicator`
     subagent with the manuscript directory and output path
     `.edaitor/canon/continuity.json`.
7. `validate_findings.py <manuscript_dir>` again.
8. Report a short summary: entity/fact counts from `canon.json`, and
   contradiction counts from `continuity.json` broken out by `kind`
   (`contradiction` vs. `unverified`) and by `severity`. Name any
   `likely_unpropagated_revision` collisions explicitly — those are the
   ones the author can act on fastest ("section_02 changed since the
   last pass, section_07 didn't").

Contradiction ids and status don't yet carry forward across runs the way
developmental findings do — a collision that was open, gets fixed, then
recurs elsewhere will get a new id rather than reusing the old one. This
is a known limitation, not an oversight; see `TODO.md`.

## `/edaitor:run status`

Show phase, developmental round, open findings by severity, and the
`diff` verdict if the text has moved since the last assessment. Also
show open contradiction counts from `.edaitor/canon/continuity.json` (if
it exists) and whether `edaitor_canon.py stale` reports any sections
needing re-extraction. End with one concrete recommended next command.
