---
name: run
description: Runs layered editing on a manuscript (fiction, design docs, or any domain redliner has been configured for) — developmental assessment, revision support, re-checks, then line editing. Use when the author asks to edit, assess, or review a manuscript, to work or resolve a finding, or types /redliner:run.
---

# redliner:run

Phase-aware editing pipeline. Subcommands:

| Command | Does |
|---|---|
| `/redliner:run status` | Where this manuscript stands |
| `/redliner:run assess` | Developmental pass (round 1, or a fresh full read) |
| `/redliner:run work <id>` | Talk through one finding and how to revise it |
| `/redliner:run resolve <id>` | Mark a finding addressed (author's claim) |
| `/redliner:run recheck` | Re-read after revision; verify claims, find new issues |
| `/redliner:run line` | Line-editing phase (gated — see below) |
| `/redliner:run continuity` | Extract facts, find collisions, adjudicate — see below |

Default with no subcommand: run `status` and recommend the next step.

Manuscript directory comes from the argument, or defaults to the current
directory — the intended usage is `cd` into the author's manuscript
directory, then run `/redliner:run`. State lives in
`<manuscript_dir>/.redliner/`.

## Deterministic operations

Everything below refers to a handful of deterministic operations —
checking state, diffing against the last snapshot, checking which
sections need continuity re-extraction, reconciling the canon, and
validating everything under `.redliner/` against the domain's schema.
These are described here by what they do, not by exact syntax: use
whichever concrete tool this session actually has for each one (a bare
`redliner state status`/`redliner state diff`/`redliner canon stale`/
`redliner canon reconcile`/`redliner validate` command on the CLI
variant, or the matching MCP tool — `state_status`, `state_diff`,
`canon_stale`, `canon_reconcile`, `validate_findings`, and so on — on
the Cowork/MCP variant). Don't guess at exact command syntax if you're
unsure which mechanism is available; check what's actually offered in
this session and use that.

## Which subagent to Task

Agent files are named `agents/<domain>-<role>.md` and registered under
the plugin namespace as `redliner:<domain>-<role>` (e.g.
`redliner:fiction-developmental-editor`) — never a bare or undomained
name. Determine `<domain>` by checking the manuscript's current state
(fall back to `fiction` only if that's somehow absent from old state)
and substitute it into every `redliner:<role>` reference below. So on a
`design-doc` manuscript, "Task `redliner:<domain>-developmental-editor`"
means Task `redliner:design-doc-developmental-editor`. This holds for
all five roles: `developmental-editor`, `line-editor`,
`editorial-aggregator`, `continuity-extractor`, `continuity-adjudicator`.

## Why phases are sequential

Developmental editing comes first and iterates until structure settles;
line editing comes after. This mirrors real editorial practice for a
concrete reason: **line-editing prose that structural work will cut or
rewrite is wasted effort**, and a combined pass can produce contradictory
advice about the same paragraph — recommending it be deleted while also
suggesting sentence-level rewrites for it.

Everything below enforces that ordering.

## Preconditions (all subcommands)

Check the manuscript's current state. If there's no state, or no
`.redliner/brief.md`, stop and tell the author to run `/redliner:intake`
first — every pass depends on the brief.

**Get your bearings in one call, not four.** Where this session has a
composite orientation operation — `redliner context <manuscript_dir>` on
the CLI, or the matching MCP tool — use it instead of separately asking
for state, the domain config, the section list, the diff verdict, and
canon staleness. It returns all of them as one JSON object.

This is not a style preference. Every extra command is a full round trip
that re-reads the whole accumulated context, so it costs both tokens and
a latency cycle — measured at ~50K cache-read per call on a real run,
where the orientation phase alone burned four calls and asked for the
domain config *twice*. Read the result once and keep it; don't re-run a
command to re-read a field you already have.

## Say what's about to happen, before starting a long pass

Applies to `assess`, `recheck`, `line`, and `continuity` — the
subcommands that Task subagents. **Not** `status`, `work`, or `resolve`,
which are fast; announcing those is just noise.

These passes take **minutes to hours**, and almost all of it is spent
inside subagents with no output. The author sees a ticking timer and a
token count, which says *something is happening* but not *how long this
should take* or *how far along it is*. Without that, a normal eight-minute
stretch is indistinguishable from a hang — this caught out the tool's own
author, so assume it will catch out a novelist.

Before step 1, tell them, briefly:

- **The steps this pass will run**, counted for this manuscript — e.g.
  for `assess` on 12 sections: "one developmental read of the whole
  manuscript, continuity extraction on 12 sections, then reconcile,
  adjudication if collisions are found, and the editorial letter."
- **A rough duration**, computed from the real section count.
- **That long silent stretches are expected**, and that the timer
  ticking means it's alive.

**Estimating.** `assess` runs roughly **N + 3** model steps for N
sections (one whole-manuscript developmental read, N continuity
extractions, adjudication only if a collision is found, one letter).
`line` is about **N + 1**; standalone `continuity` about **N + 1**;
`recheck` varies with how much changed, and is usually smaller.

Budget **~2–3 minutes per step** as a rough figure. Say it as a range,
never a countdown — and be honest that it's an estimate. Its basis is a
single measurement: **~13 minutes for 3 short sections (~250 words
each)**, so longer sections run longer and a full manuscript is
comfortably into hours. Don't present that number as more precise than
it is, and don't re-derive a smaller number to sound better.

If the estimate exceeds roughly half an hour, say so plainly and offer
the alternative — a `continuity`-only pass, or assessing a subset — so
the author can choose rather than discovering the cost at minute forty.

**Then report each step as it finishes**, in one short line naming the
step and its position — "developmental read done (1/6); extracting
continuity facts from section_01 (2/6)". You are the coordinator
between Task calls, so this costs nothing and needs no status-line
configuration: it is simply text emitted between steps, and it turns an
opaque wait into visible progress.

Keep it to one line per step. Don't summarize findings mid-pass — the
letter does that at the end, and a running commentary of findings
invites the author to start reacting to a half-finished picture.

## `/redliner:run assess`

1. Move the manuscript to the developmental phase (this increments the
   round counter).
2. Prepare `<manuscript_dir>/.redliner/findings/`; clear stale files from
   a previous round.
3. Task the `redliner:<domain>-developmental-editor` subagent with the manuscript directory,
   the round number, and output path `.redliner/findings/developmental.json`.
4. Validate everything currently under `.redliner/` — stop and report
   errors rather than aggregating bad data. (This checks the whole
   manuscript directory in one pass, not just the one file you just
   wrote.)
5. Run the **continuity** steps below now — **before the snapshot in the
   next step**, not after. Continuity's reconcile step diffs against
   whatever baseline is currently in the manuscript's state to decide
   `likely_unpropagated_revision`; if the snapshot runs first, that
   baseline already matches the current text and the diff always comes
   back empty, silently disabling the flag. This only looks harmless on
   a first-ever assess (empty baseline either way); it breaks the moment
   `assess` is re-run later as "a fresh full read" with a real prior
   baseline. Don't reorder this.
6. Record the current text as the assessed baseline — this is what lets
   a later `recheck` tell what changed.
7. Task `redliner:<domain>-editorial-aggregator` for the **developmental** letter.
8. Validate again, then read and show the developmental letter, then the
   continuity summary from step 5.

Do **not** run line editing here, whatever the author asked for.

## `/redliner:run work <id>`

Revision support — the conversational one. Do this **in the main session,
not via a subagent**: it's a back-and-forth, not a batch job.

1. Read the brief, the finding by id, and the sections it touches.
2. Talk it through with the author: why it's a problem, what options
   exist, what each would cost elsewhere in the book. Offer approaches
   rather than prescribing one; they're the author.
3. If they draft a fix, react to it honestly — including saying it
   doesn't solve the problem when it doesn't.
4. Don't edit the manuscript unless asked directly. Their book.

## `/redliner:run resolve <id>`

Set that finding's `status` to `claimed` in `developmental.json` (not
`addressed` — the author's claim isn't verification). Confirm, and note
that `/redliner:run recheck` will verify it.

## `/redliner:run recheck`

1. Compare the manuscript's current text against the last assessed
   snapshot, and branch on the verdict — this comparison is
   deterministic, so trust it over any impression of how much changed:

   - **`unchanged`** — no section text differs from the last assessment.
     Any `claimed` findings can't be verified; say so plainly and stop.
     Something's off: either the revision wasn't saved, or the author
     marked things resolved without revising.
   - **`targeted`** — specific sections edited, none added or removed,
     no large swings. Task `redliner:<domain>-developmental-editor` to verify the `claimed`
     findings against those sections and check whether the edits created
     new problems. Pass the existing findings file so ids carry forward.
   - **`restructured`** — sections added, removed, or heavily rewritten.
     A full re-read: task `redliner:<domain>-developmental-editor` over the whole
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
   That signal comes from diffing against the baseline recorded at the
   last assessment — if the snapshot (next step) runs first, the
   baseline already matches the current text, the diff comes back empty,
   and every collision looks like a standing issue instead of fresh
   fallout from this revision. Checking which sections need
   re-extraction first tells you the real scope; a `targeted`
   developmental verdict usually means continuity only has one or two
   sections to redo, not the whole manuscript.
4. Record the current text as the new assessed baseline, aggregate a
   fresh developmental letter, show it, then show the continuity summary
   from step 3.
5. Then say plainly whether structure looks settled enough for line
   editing — count open `major`/`critical` findings and give a real
   recommendation, not a hedge.

## `/redliner:run line`

1. Read `developmental.json`. Count open findings at `major` or
   `critical`.
2. **If any exist, warn before proceeding** — name them and explain that
   line editing now risks polishing prose that structural revision will
   change. Then, if the author still wants to continue, continue. This is
   a soft gate by design: it's their manuscript and there are legitimate
   reasons to line-edit a section that's structurally settled even while
   other parts aren't.
3. Move the manuscript to the line phase.
4. For each section, task `redliner:<domain>-line-editor` with the manuscript directory,
   that section's path, any `deferred_to_line` developmental findings for
   it, and output path `.redliner/findings/line_<section_stem>.json`.
   Sequential keeps the transcript readable; parallel is fine too —
   sections share no state.
5. Validate.
6. Task `redliner:<domain>-editorial-aggregator` for the **line** letter.
7. Validate, then read and show the letter.

## `/redliner:run continuity`

Callable directly, and also what `assess` and `recheck` run at the end
of their own flow (see above) — this section is the one definition both
refer to.

Unlike the other two layers, this one is not phase-gated: extraction is
judgment-free (it doesn't need the developmental pass to have run first)
and it tracks its own staleness per section, independent of the
developmental round counter. It's safe to run any time after intake.

1. Check which sections need (re-)extraction. The result drives
   everything below:
   - `needs_extraction` — sections to (re-)extract this run.
   - `current_hashes` — each of those sections' current SHA-256, keyed by
     stem. The extractor needs this exact value; don't compute it
     yourself or reuse a stale one from state.
   - `orphaned_observations` — observation files whose section no longer
     exists (deleted or renamed). Delete
     `.redliner/canon/observations/<stem>.json` for each before
     reconciling — a cut section's facts shouldn't still count toward
     canon or collisions.
2. If `needs_extraction` is empty, skip to step 5 — nothing changed
   since the last extraction.
3. For each section in `needs_extraction`, Task the
   `redliner:<domain>-continuity-extractor` subagent with: the manuscript
   directory, that section's file path, its hash from `current_hashes`,
   and output path `.redliner/canon/observations/<section_stem>.json`.
   Sections share no state — parallel is fine, sequential keeps the
   transcript readable.
4. Validate — stop and report errors rather than reconciling from bad
   extraction data.
5. Reconcile the canon — this deterministically rebuilds
   `.redliner/canon/canon.json` (merged facts) and
   `.redliner/canon/collisions.json` (entity+attribute pairs with more
   than one asserted value) from every current observations file, not
   just the ones just (re-)extracted.
6. Read the resulting collisions.
   - **Empty `collisions` list** — write
     `.redliner/canon/continuity.json` as `{"contradictions": []}`
     yourself. There's nothing to adjudicate, so don't spend a Task call
     on it.
   - **Non-empty** — Task the `redliner:<domain>-continuity-adjudicator`
     subagent with the manuscript directory and output path
     `.redliner/canon/continuity.json`.
7. Validate again.
8. Report a short summary: entity/fact counts from the canon, and
   contradiction counts broken out by `kind` (`contradiction` vs.
   `unverified`) and by `severity`. Name any
   `likely_unpropagated_revision` collisions explicitly — those are the
   ones the author can act on fastest ("section_02 changed since the
   last pass, section_07 didn't").

Contradiction ids and status don't yet carry forward across runs the way
developmental findings do — a collision that was open, gets fixed, then
recurs elsewhere will get a new id rather than reusing the old one. This
is a known limitation, not an oversight; see `TODO.md`.

## `/redliner:run status`

Show phase, developmental round, open findings by severity, and the
diff verdict if the text has moved since the last assessment. Also show
open contradiction counts from `.redliner/canon/continuity.json` (if it
exists) and whether any sections need re-extraction for continuity. End
with one concrete recommended next command.
