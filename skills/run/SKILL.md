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
| `/redliner:run resolve <id>` | Mark a finding addressed (author's claim) — developmental or line |
| `/redliner:run wontfix <id>` | Decline a finding, with a reason; it won't be re-raised |
| `/redliner:run recheck` | Re-read after revision; verify claims, find new issues |
| `/redliner:run line` | Line-editing phase (gated — see below) |
| `/redliner:run continuity` | Extract facts, find collisions, adjudicate — see below |

Default with no subcommand: run `status` and recommend the next step.

Manuscript directory comes from the argument, or defaults to the current
directory — the intended usage is `cd` into the author's manuscript
directory, then run `/redliner:run`. State lives in
`<manuscript_dir>/.redliner/`.

## redliner suggests. It doesn't rewrite.

**The author writes; redliner advises.** No pass, no subcommand, and no
agent modifies a `section_*` file on its own initiative — not to fix a
contradiction, not to correct a typo, not to apply a suggestion the
author liked, not "just this one line."

That's what the tool *is*. An editor who rewrites your book has replaced
your voice with theirs, and an author cannot review a change they didn't
make. Every artifact redliner produces — findings, letters, canon,
observations — is *about* the manuscript and lands outside it.

**This is not a lock.** If the author directly asks the assistant to make
changes, do it — it's their book and their session. But **say once,
plainly, what's happening**: that rewriting isn't what redliner is for,
and they've stepped out of the tool into ordinary AI assistance. Not a
warning or a guilt trip — one sentence, so the boundary stays legible
and they know which hat the thing across from them is wearing. Then
help properly.

**Any such changes go to a markup copy, never the source.** See
"Markup copies" below.

Concretely:

- **Point precisely, then stop.** Name the section and line, quote what's
  there, say what's wrong and why it matters. Precision is the deliverable
  — "section_05 line 16 says X where the rest of the manuscript says Y" is
  a finding the author can act on in seconds.
- **Never offer to make the change.** Not "want me to fix that?", not
  "I can update that for you", not "shall I apply this?" Suggest, and
  stop. The offer is the problem, not just the edit: it puts the author
  in the position of declining their own authorship, and it reframes
  advice as a job the tool is waiting to do. Make the suggestion good
  enough to act on and leave the acting to them.
- **Pushback runs both ways.** Say plainly when a proposed fix doesn't
  solve the problem. And when the author disagrees with a finding, treat
  that as information, not an obstacle: they know things the manuscript
  doesn't say — intent, planned reveals, deliberate choices. Ask what you
  were missing, and if they're right, say so and record it in the brief
  so no later pass re-reports it. Don't cave to be agreeable, and don't
  re-litigate a decision they've made.
- **Never silently "clean up" anything** — stray notes to self, leftover
  scaffolding, inconsistent formatting. Report it and let them decide.

## Markup copies

When the author *has* asked for changes (see above), write them to a
**markup copy**, never to the source:

```
<manuscript_dir>/markup/section_05.md
```

Create `markup/` if it doesn't exist, copy the source file across on
first touch, and edit only the copy. The author's file is never modified,
so they can diff, take some changes and not others, or throw the whole
thing away — which is the point.

**The subdirectory is load-bearing, not cosmetic.** Section discovery
globs `section_*.{txt,md}` in the manuscript directory itself and is not
recursive, so `markup/section_05.md` is invisible to it. A sibling file
like `section_05_markup.md` or `section_05.markup.md` would instead be
picked up as an **extra section** — silently inflating the canon,
producing phantom collisions against a near-duplicate of a real chapter,
and skewing every count in the letter. Never put a markup file beside the
chapters.

Tell the author where the copy is, by absolute path, and that their
original is untouched.

**This is a boundary, not a lock, and deliberately so** (decided
2026-08-14). Agent `Write` is not path-restricted and there is no
pre-write hook enforcing any of the above — enforcement was considered
and **rejected**. A hard lock would fight the author on their own files
to defend a principle that exists for their benefit, and the failure it
prevents (an assistant rewriting prose unasked) is one this file's
instructions already address. Don't add one later without a concrete
incident to point at.

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
all six roles: `developmental-editor`, `line-editor`,
`editorial-aggregator`, `continuity-extractor`, `continuity-adjudicator`,
`continuity-joiner`.

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
2. **Archive the previous round before clearing anything**
   (`redliner rounds archive <dir> developmental`, and `... line` if a
   line pass ran). Then prepare `<manuscript_dir>/.redliner/findings/`
   and `.redliner/letters/`, clearing stale files from the previous
   round.

   This step is where the "before" used to be destroyed: every pass
   rewrites findings in place, so clearing without archiving left nothing
   to compare a later round against. Archiving first makes the clear
   safe. Say in one line that you've archived and where.
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
7. Task `redliner:<domain>-editorial-aggregator` for the **developmental**
   letter, giving it **both output paths explicitly**:
   - Markdown → `<manuscript_dir>/Developmental Letter - Round <N>.md`
   - JSON → `<manuscript_dir>/.redliner/letters/developmental_round<N>.json`

   where `<N>` is the round from step 1. The agent's contract is "write
   both output paths you're given" — if you don't supply them it has to
   invent a location, and the letter then lands somewhere nothing else
   can reliably find.

   **The Markdown letter goes in the manuscript folder itself, not inside
   `.redliner/`, and that split is deliberate: hidden storage for machine
   state, visible files for anything a human reads.** `.redliner` is a
   dotfile directory that Finder hides by default, and an author who
   can't find the letter got nothing for the whole pass. Real feedback,
   2026-08-13: "I don't know where the .redliner directory you linked
   is." The JSON sidecar stays hidden — that one is for the tool.

   Sitting beside the chapters is safe: section discovery globs
   `section_*`, so a letter named this way is never mistaken for
   manuscript text.
8. Validate again, then read and show the developmental letter, then the
   continuity summary from step 5.
9. Re-apply author decisions, archive the completed pass, and record it:
   `redliner decisions apply <dir>`, `redliner rounds archive <dir> developmental`
   (and `... continuity`), `redliner state pass <dir> developmental`.
   (and `... continuity`, since step 5 ran one). This is what lets
   `status` tell the author what has actually been run rather than only
   what phase they're in. **Print the letter's absolute path**
   when you show it — the author needs to be able to reopen it later
   without hunting for it, and telling them only that "the letter is
   written" is how a pass ends with the author unable to find its one
   deliverable.

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
4. **Never edit the manuscript, and never offer to.** See the absolute
   rule at the top of this file. This is the subcommand where the pull is
   strongest — you're mid-conversation about a specific fix and offering
   to apply it feels helpful. It isn't; it hands you the authorship.
5. If they push back on the finding, take it seriously — they know intent
   the page doesn't carry. If they're right, say so, drop it, and offer
   to record the reason in the brief so no later pass raises it again.

## `/redliner:run resolve <id>`

Set that finding's `status` to `claimed` (not `addressed` — the author's
claim isn't verification). Confirm, and note that `/redliner:run recheck`
will verify it.

**Find the finding by the shape of its id — it may be in any of several
files:**

- `dev-NNN` → `.redliner/findings/developmental.json`
- `line-<section_stem>-NNN` → `.redliner/findings/line_<section_stem>.json`
  (the stem is embedded in the id, so `line-section_03-007` lives in
  `line_section_03.json` — don't scan every file)

  Parse it precisely: the stem contains **underscores, not hyphens**, so
  splitting the id on `-` gives exactly three parts and the stem is the
  middle one. Splitting on the wrong separator produces a plausible
  filename that doesn't exist (`line_section_03_007.json`), which fails
  as a confusing "no such file" rather than a clean "no such finding".

If the id matches nothing, say so and list the id prefixes that exist
rather than guessing at a near match. Resolving the wrong finding is
worse than failing to find one.

Line findings were unresolvable until 2026-08-14 — `resolve` only ever
touched `developmental.json`, so line findings could never be anything
but `open` and their count could only grow. The schema always allowed it;
the flow just didn't.

## `/redliner:run wontfix <id> [reason]`

The author considered the finding and is declining it. Set `status` to
`wontfix` — same id lookup as `resolve` above — and **record why**.

**Always ask for a reason if none was given**, in one short question.
A bare `wontfix` is a landmine: a later round sees a suppressed finding
with no explanation and can't tell a deliberate craft choice from an
abandoned one, and neither can the author six months on.

**Record it in two places.** First append it to
`.redliner/decisions.json` — the durable record, described under "Author
decisions survive re-runs" below — then write the matching `resolution`
block into the finding itself. Extra keys are permitted on findings
(unlike facts, whose schema is deliberately sealed), so this needs no
schema change:

```json
"status": "wontfix",
"resolution": {
  "set_by": "author",
  "at": "2026-08-14T12:00:00Z",
  "reason": "The register shift is intentional here."
}
```

`set_by` is `author` when a human decided, and the pass name
(`recheck`, `line`) when a pass did. That distinction is the point:
**a status a person chose must never be overwritten by a status a
machine inferred.**

### One finding, or the whole class?

`wontfix` stops *that finding* coming back. It does nothing about the
next one of the same kind, in another section, next round.

So ask which it is. If the reason is a standing property of the work —
an intentional device, a convention, a deliberate omission — **offer to
add it to the brief's "Deliberate choices" as well**, and say plainly
that this is what stops the whole class recurring. That mechanism is
proven: a bilingual-naming convention and a planned-reveal setup were
recorded in one manuscript's brief and no subsequent pass re-raised
either, across five independent agents that never saw each other's
output.

### Tell authors this exists

Neither letter mentions it, so nobody would know. When showing findings
— in a letter, in `work`, in `status` — say once that a finding they
disagree with can be declined with a reason and won't be re-raised.
Authors know things the manuscript doesn't say; the tool's job is to
make recording that cheap, not to keep re-litigating.

## Completed passes are kept, not overwritten

Each finished pass is archived to
`.redliner/rounds/<pass>-round<N>/` — a copy of that pass's findings (or,
for continuity, its collisions and adjudication). `redliner rounds list`
shows what's there.

They're kept **by default** because they're small and the alternative is
unrecoverable: a full round of developmental, line and continuity
artifacts for a five-section manuscript is about 180KB, so even a long
project stays under a few megabytes. Losing the previous round means
losing the ability to answer "what changed since last time" forever.

**Never delete anything under `rounds/` without asking the author
first**, and don't offer to prune unprompted — the cost of keeping is
negligible and the cost of a wrong deletion is total. If they ask what's
accumulating, show `rounds list` and the size, and let them decide.

## Author decisions survive re-runs

`resolve` and `wontfix` both append to `.redliner/decisions.json`:

```json
{"decisions": [
  {"id": "line-section_01-013", "status": "wontfix", "set_by": "author",
   "at": "2026-08-14T12:05:00Z", "reason": "deliberate; rule chosen, see brief"}
]}
```

**Then run the apply-decisions operation at the end of every pass**
(`redliner decisions apply <dir>` on the CLI), after the agents have
written and validation has passed.

Why this exists rather than trusting the agents: the developmental and
line editors **rewrite findings files wholesale** on every re-check. They
are told to preserve author-set statuses, and mostly will — but an
instruction is not a guarantee, and a single agent that renumbers or
forgets silently discards a decision a human made, with nothing detecting
it. `decisions.json` is a file no agent writes, so re-applying it makes
the guarantee structural. Verified: after simulating a pass that reset an
author's `wontfix` to `open`, apply reported `1 restored` and put the
status and reason back.

It reports three things — how many decisions were already correct, which
it had to restore (each restore means a pass overwrote a human decision,
worth noticing), and which decisions no longer match any finding. That
last one isn't an error: a section gets cut and its findings go with it.
It's reported so a decision doesn't sit there applying to nothing
forever.

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
4. **Verify line claims too, if there are any.** Structure isn't the only
   layer the author revises against. If `.redliner/findings/line_*.json`
   exist and any finding in them is `claimed`, re-task
   `redliner:<domain>-line-editor` for **each section holding a claimed
   finding** — not every section — passing that section's existing
   findings file so ids carry forward and statuses get resolved to
   `addressed`/`stale`. Skip this entirely when the developmental verdict
   is `restructured`: a heavy rewrite makes line findings stale wholesale,
   and re-running the line editor over churning prose spends money
   polishing text that is still moving. Say that's why you skipped it.
5. Record the current text as the new assessed baseline, aggregate a
   fresh developmental letter, show it, then show the continuity summary
   from step 3. If step 4 ran, aggregate and show a fresh line letter as
   well, and record the pass.
6. Then say plainly whether structure looks settled enough for line
   editing — count open `major`/`critical` findings and give a real
   recommendation, not a hedge.

## `/redliner:run line`

0. **Check the draft stage first — this gate is harder than the one
   below, and skipping it wastes the author's money.** Read state's
   `draft_stage`. If it is a stage whose implication rules out
   line-level findings (fiction's `exploratory / partial`: "no
   line-level findings at all"), **stop and say so before spawning
   anything.** The line editors would each read the brief, correctly
   return nothing, and hand the author an empty letter after N model
   calls. Tell them the stage, quote its implication, and name the
   command that changes it (`redliner state stage <dir> <stage>`).
   Proceed only if they set a stage that permits line findings.

   If `draft_stage` is unset, don't guess — say it's unset, show the
   domain's stage list, and ask which applies.
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
6. Task `redliner:<domain>-editorial-aggregator` for the **line** letter,
   giving it **both output paths explicitly**:
   - Markdown → `<manuscript_dir>/Line Letter - Round <N>.md`
   - JSON → `<manuscript_dir>/.redliner/letters/line_round<N>.json`

   where `<N>` is the manuscript's current `developmental_round` — line
   passes aren't round-tracked themselves, so they're stamped with the
   structural round they follow, which keeps a later line pass from
   silently overwriting an earlier one. Same visible/hidden split and
   same reason as the developmental step.
7. Validate, then read and show the letter — and **print its absolute
   path**, so the author can open it later without hunting.
8. Re-apply author decisions (`redliner decisions apply <dir>`), archive
   the pass (`redliner rounds archive <dir> line`), then record it
   (`redliner state pass <dir> line`).

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
   `.redliner/canon/collisions.json` from every current observations
   file, not just the ones just (re-)extracted.

   **What `collisions.json` holds is narrower than it used to be**: one
   entity carrying two different values under the *same attribute name*.
   It no longer tries to link attributes that merely share a word. That
   linking produced 87% of collisions on a real corpus as artifacts and
   never made the join it was added for, because a string comparison
   cannot tell that two names denote one thing. See TODO.md, "Is
   deterministic collision-finding the right architecture?".

6. **Two passes now run over this, not one.** They find different
   classes, neither is a superset of the other, and both were measured:

   a. **Adjudicate the collisions.** Read `collisions.json`.
      - **Empty list** — nothing to adjudicate; don't spend a Task call.
        Skip to (b).
      - **Non-empty** — Task the
        `redliner:<domain>-continuity-adjudicator` subagent with the
        manuscript directory and output path
        `.redliner/canon/continuity.json`.

   b. **Join across the whole corpus.** Write the fact bundle with
      `redliner canon bundle <dir> > <dir>/.redliner/canon/bundle.txt`,
      then Task the `redliner:<domain>-continuity-joiner` subagent with
      the manuscript directory and output path
      `.redliner/canon/joined.json`.

      This is the pass that catches a contradiction whose two halves are
      filed under different names — measured at 4/4 on a manuscript where
      the deterministic pass scores 0/4, and stable at 4/4 across five
      runs. It reads one compact line per fact rather than the prose, so
      it is one call regardless of manuscript length.

   c. **Merge, deterministically.** `redliner canon merge <dir>` folds
      `joined.json` into `continuity.json`, renumbering the joiner's ids
      into the `cont-5NN` range so provenance stays readable and the two
      agents never write the same file. Two agents editing one path is
      how author decisions got clobbered before; the merge is a command,
      not an instruction to an agent.

      If neither pass produced anything, write
      `.redliner/canon/continuity.json` as `{"contradictions": []}`
      yourself.

7. Validate again.
8. Archive and record the pass: `redliner rounds archive <dir> continuity`,
   then `redliner state pass <dir> continuity`.
9. Report a short summary: entity/fact counts from the canon, and
   contradiction counts broken out by `kind` (`contradiction` vs.
   `unverified`) and by `severity`. Name any
   `likely_unpropagated_revision` collisions explicitly — those are the
   ones the author can act on fastest ("section_02 changed since the
   last pass, section_07 didn't").

**One run samples the soft findings; it does not enumerate them.**
Measured across five identical runs, flat contradictions came back every
time, but the judgment-call items (`kind: unverified`) rotated — seven
items across five runs, with an empty intersection, every one of them a
legitimate question. So a clean `unverified` list means "nothing was
asserted falsely", not "there is nothing to ask about". Say it that way
to the author, and don't treat one run's question set as exhaustive.

Contradiction ids and status don't yet carry forward across runs the way
developmental findings do — a collision that was open, gets fixed, then
recurs elsewhere will get a new id rather than reusing the old one. This
is a known limitation, not an oversight; see `TODO.md`.

## `/redliner:run status`

Show phase, developmental round, open findings by severity, and the
diff verdict if the text has moved since the last assessment. Also show
open contradiction counts from `.redliner/canon/continuity.json` (if it
exists) and whether any sections need re-extraction for continuity.

Also show, because an author cannot otherwise see any of it:

- **The draft stage**, from state's `draft_stage`, *with its severity
  implication* from the domain's `draft_stages`. This is the setting that
  most changes what redliner does and it is invisible everywhere else.
  If it's unset, say so and say it must be set before a line pass.
- **What has been run**, from state's `passes` — e.g. "developmental ×3,
  line ×1, continuity ×4". Distinct from `developmental_round`, which
  counts rounds *entered*, not passes *completed*.
- **What's available next, and what isn't** — name the passes that can
  run now, and for any that can't, say why in one line ("line editing is
  gated at this draft stage; it would return nothing"). An author should
  never discover a gate by paying for an empty pass.

End with one concrete recommended next command, and be explicit about
whether redliner is **waiting on them** (a revision, a decision, a stage
change) or simply offering options.
