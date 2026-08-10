# Open questions / deferred work

Design questions raised during development that we deliberately parked.
Not a task tracker — the reasoning matters more than the checkbox.

## Domain generalization: build the domain-creation skill (DONE)

**Raised:** 2026-08-09. **Completed:** 2026-08-09.

The user wants redliner usable beyond fiction (product proposals, design
docs) if it can be done without compromising the fiction use case, which
stays the primary one. This was scoped as four steps; **all four are
done**:

1. ✅ Renamed `chapter_*.txt` → `section_*.txt` and all associated field
   names everywhere (mechanical, no domain concept yet).
2. ✅ Added a `domain` field to `.redliner/state.json`, defaulting to
   `"fiction"`.
3. ✅ Moved fiction's category vocabulary (developmental/line categories,
   continuity entity types/sources/categories) into
   `domains/fiction/domain.json`, loaded per-manuscript by
   `bin/schemas/domain_loader.py`. `findings_schema.py`/`canon_schema.py`'s
   validators take categories as parameters now, not module constants.
   Also fixed a coupling bug this surfaced: `redliner_state.py`'s
   round-increment logic hardcoded a check against the literal string
   `"developmental"` — now reads `round_tracked_phase` from the domain
   config instead.

See git log around commits `95a25e5` and `51b33bb` for the detail and the
verification each step went through (no regression against
`sample_manuscript`, confirmed by direct testing, not assumed).

**Step 4, done: the domain-creation skill + docs.** Design decisions made
for it before starting, held to throughout — recorded here so they don't
get re-litigated by a future reader:

- **Generate concrete per-domain agent files (B1), don't make agent
  prompts generic with runtime-injected vocabulary (B2).** The agent
  files are the best-engineered part of this system (real iterated
  prompt craft, e.g. `developmental-editor.md`'s handling of
  `deferred_to_line`). Runtime injection would either hollow that out
  into generic text or move it into a config file where it's harder to
  read/review/diff, and it would make the orchestrating skill's
  prompt-construction step — already the site of one real bug (the
  namespace issue in `SKILL.md`) — the place every future bug hides. A
  domain should be a template that *generates* static files like
  `agents/design-doc-structural-editor.md`, regenerated when the domain
  changes, hand-editable after.
- **Category design needs explicit guardrails**, because it's a harder
  interview than intake: bad category design (too many, overlapping,
  unclear boundaries) silently degrades every finding downstream forever.
  Give the skill explicit rules: 4–7 categories per phase, each must be
  something a reviewer could plausibly disagree about being present, no
  category that's really a severity in disguise (e.g. don't allow both
  `minor_issue` and a `severity` field — that's redundant, not two
  useful axes).
- **Write the design-doc domain's `entity_types`/`categories` by hand
  first, before building anything.** This was the sanity check on
  whether the domain-config format is right: "the summary says Q3
  launch, the timeline section says Q4" needs to be expressible without
  contorting fiction-shaped fields. If it isn't easy to express, the
  format is wrong, not the example.
- **Phase names stay `developmental`/`line` internally, not made
  domain-configurable, for now.** Renaming those to something generic
  (`structural`/`detail`) was considered and deliberately deferred — it
  would also mean renaming the `/redliner:run assess`/`/redliner:run line`
  command surface, which is worse UX for the fiction case that's still
  primary. A second domain can use the same two internal phase keys with
  different *meaning* (its `domain.json`'s category lists carry the
  actual semantic difference) without needing the keys themselves to
  change. Revisit only if a domain genuinely doesn't fit a two-phase
  structural/detail split at all.

**What actually got built:** `skills/new-domain/SKILL.md` plus
FIXED/AUTHORED templates for all five agent roles in
`skills/new-domain/reference/templates/`. Before writing the skill, two
prerequisite gaps had to close first (found by trying to actually use
the format, not by inspection):

- `skills/intake/SKILL.md` hardcoded fiction's brief questions *and* its
  draft-stage severity table directly in prose — `domain.json` having a
  `brief_fields` list didn't matter if nothing read it. Fixed by adding
  a `draft_stages` array (`{name, implication}`) to the domain schema
  alongside `brief_fields`, and rewriting intake to read both from
  whichever domain is active, asking which domain only when more than
  one exists.
- `agents/*.md` were renamed to `agents/<domain>-<role>.md`, but the
  **filename rename alone did nothing** — Claude Code registers a
  subagent's id from the `name:` field in its frontmatter, not the
  filename. This was caught by an actual `claude --plugin-dir` load
  test (bare renamed files kept resolving to the old unprefixed agent
  id), the same methodology that already caught the bare-vs-namespaced
  bug earlier in this project. Fixed by changing both. `run/SKILL.md`
  now resolves `redliner:<domain>-<role>` from the manuscript's `domain`
  field at Task time rather than hardcoding fiction's names.

The design-doc domain's `entity_types`/`sources`/`categories` were
written by hand first, as planned, and the "summary says Q3, timeline
says Q4" case mapped cleanly (`sources: body/summary/appendix`, with
`summary` playing the same "can legitimately be an imprecise
restatement" role fiction's `dialogue`/`character_thought` play) —
confirming the format generalizes without contortion. Verified two ways,
described in README's Status section: a hand-built findings/canon
fixture through `validate_findings.py` (no model calls), and a live
`claude --plugin-dir` check that all five generated `design-doc` agents
actually register and respond under their expected ids.

**Gap surfaced here, since fixed** — see "Wire the continuity layer into
`/redliner:run`" below.

## Wire the continuity layer into `/redliner:run` (DONE)

**Raised:** 2026-08-09 (surfaced while closing out domain
generalization). **Completed:** 2026-08-09.

`agents/*-continuity-extractor.md` and `*-continuity-adjudicator.md`
were never Tasked from any step in `run/SKILL.md`, for either domain —
the continuity layer's sample data in this repo had been produced by
hand or direct script testing, not through the orchestrated pipeline.

**What got built:** `/redliner:run continuity` — callable standalone, and
run automatically at the end of both `assess` and `recheck`. Steps:
`redliner_canon.py stale` (extended to report each stale section's
current hash, so the orchestrator doesn't need a second round trip or to
hash sections itself) → Task the extractor per stale section → validate
→ `redliner_canon.py reconcile` → if `collisions.json` is empty, write
`.redliner/canon/continuity.json` as `{"contradictions": []}` directly,
no model call; otherwise Task the adjudicator → validate → summarize.
Not phase-gated — extraction is judgment-free and tracks its own
per-section staleness independent of the developmental round counter, so
it's safe to run any time after intake.

Verified live, three times, against real agents rather than hand-built
fixtures (the thing hand-built fixtures specifically couldn't prove: a
real agent choosing its own `excerpt` text and that text surviving the
verbatim check):
- A two-section fixture with a planted contradiction (Mira's eyes:
  green in section_01, blue in section_02) — extraction, reconciliation,
  and adjudication all ran for real, produced a correctly-categorized
  `character_attribute` contradiction at `moderate` severity, and passed
  `validate_findings.py` including the excerpt check.
- A matching clean fixture with no contradictions — confirmed the
  adjudicator is genuinely skipped (zero Task calls for it) and
  `continuity.json` is written directly as `{"contradictions": []}`.
- A real `assess` → edit one section → `recheck` cycle, which caught a
  real ordering bug before it shipped: the first draft of this wiring
  ran `redliner_state.py snapshot` *before* continuity's `reconcile`.
  `reconcile` computes `likely_unpropagated_revision` by diffing against
  whatever baseline currently sits in `state.json` — snapshotting first
  moves that baseline to match the current text, so the diff always came
  back empty and the flag could never fire, silently. Invisible on a
  first `assess` (baseline's empty either way, so it happened to look
  right), and only visible once a real prior baseline existed and got
  overwritten too early — exactly the case a synthetic fixture with no
  history couldn't surface. Fixed by moving continuity's steps (through
  `reconcile`) to run before `snapshot` in both `assess` and `recheck`;
  reconfirmed live afterward that the edited-section collision correctly
  came back `likely_unpropagated_revision: true`.

**Deliberately deferred, not built in this pass** (see advisor guidance
at the time: design this after seeing real pipeline output once, not
against zero observed data): **contradiction id/status carry-forward
across `recheck` runs.** Right now every `continuity` run adjudicates
the full fresh collision set from scratch — a contradiction that's
`open`, gets fixed, then a *different* pair of sections develops the
same kind of contradiction later gets a brand-new id rather than reusing
the old one. Developmental findings don't have this problem because the
model is explicitly handed the prior findings file and told to preserve
ids; collisions don't have a stable identity the same way (they're
recomputed fresh each `reconcile` from an (entity, attribute) key, not
carried as objects).

The fix sketched (not built): extend `redliner_canon.py reconcile` to
optionally read the existing `continuity.json` and emit a `carry_forward`
block matching prior contradiction ids to fresh collisions by (entity,
attribute), with `addressed`/`stale` computed the same way
`likely_unpropagated_revision` already is — script-computed, not model
guesswork:
- Prior contradiction's (entity, attribute) absent from fresh
  collisions, but still present with a single value in `canon.json` →
  `addressed` (genuinely fixed).
- Prior contradiction's (entity, attribute) absent from `canon.json`
  entirely → `stale` (the section(s) carrying it were cut).
- Otherwise → still `open`, same id.

Then the adjudicator's instruction becomes one line: reuse ids handed to
you in `carry_forward`, don't renumber. Keeps model judgment scoped to
what a script can't do — matching prior/fresh collisions and inferring
addressed-vs-stale from absence is exactly computable, the same
principle behind `diff_manuscript` and `likely_unpropagated_revision`.

**Bug caught and fixed by the same live testing, not part of the
original plan:** the first real `assess` run showed the developmental
editor double-reporting the planted contradiction — once as `cont-001`
via continuity (correct), and again as a `deferred_to_line` finding
(wrong; `deferred_to_line` is for genuinely structural prose
observations, not continuity errors, and the continuity layer already
owns those independently). Fixed with a one-line scope exclusion in
`agents/fiction-developmental-editor.md`,
`agents/design-doc-developmental-editor.md`, and the FIXED scope block
in `skills/new-domain/reference/templates/developmental-editor.md` (so
future-generated domains don't reintroduce it); reconfirmed live that a
fresh `assess` no longer reports the duplicate.

**Small gap noticed, not addressed:** `/redliner:run work <id>` and
`/redliner:run resolve <id>` only operate on `developmental.json` — there's
no equivalent for talking through or resolving a `continuity.json`
contradiction directly. Today the workaround is real (fix the text,
`/redliner:run recheck` re-derives continuity from scratch since there's
no persistent id to resolve yet anyway — see the carry-forward item
above, which is a prerequisite for this actually mattering) but worth
revisiting once carry-forward exists.

## Markdown support, structure templates, and a serial-fiction domain (DONE)

**Raised:** 2026-08-09, from the user reading the README with fresh
eyes. **Completed:** 2026-08-09.

Three ideas raised together, which turned out to be three different
mechanisms rather than one feature:

1. **`.md` section files, alongside `.txt`.** Small and mechanical —
   `section_files()` in `project_state.py` now globs both extensions,
   erroring (`SectionCollisionError`) only if the same stem exists under
   both, which would be genuinely ambiguous. Scrivener specifically was
   raised too, and deliberately **not** designed against: "Scrivener
   markdown" isn't one fixed format (depends on the author's own
   compile/sync settings, and can leak synopsis/notes into the exported
   text if compile settings include them) — same discipline already
   applied to the Obsidian idea below: design against a real exported
   sample when there is one, not an imagined format.
2. **Design-doc template compliance** — mechanically checkable, given
   markdown headers, but built as brief-level content (an optional
   `required_structure` field checked by a `structure_compliance`
   category that only fires when the field is non-empty) rather than a
   header-parsing script. Deliberately the smaller version: a real
   deterministic header-presence checker is a legitimate upgrade path
   later, once it's clear presence/order matters more than the
   section's actual adequacy (which still needs judgment either way).
3. **Serial-fiction chapter-ending pull** — not a template at all, a
   judgment call, so it became a proper domain (`serial-fiction`) rather
   than a mechanism bolted onto fiction. See below.

**Two real bugs caught by live testing, not by inspection** (same
methodology as everywhere else in this project — a script compiling or
a fixture validating is not the same claim as a live agent producing
correct behavior):

- **Markdown emphasis broke excerpt verification.** An agent extracting
  a fact from `**relentless**` correctly quoted the words
  (`relentless`), not the markup — and `validate_findings.py`'s
  whitespace-only normalizer treated that as a failed verbatim match.
  Fixed by stripping markdown emphasis/code delimiters from both sides
  of the comparison before matching, which can only make a genuine quote
  match (a fabricated one still fails, since the actual wording would
  differ, not just the punctuation around it).
- **The `SectionCollisionError` import got silently dropped from both
  `redliner_state.py` and `redliner_canon.py`** between the edit that
  added it and the edit that used it — the third occurrence of this
  exact bug class in this project (see the domain-generalization and
  earlier sections). `py_compile` doesn't catch it (a `NameError` only
  fires when the code path actually executes); only running the script
  against the real failure case does. Both fixed, reverified by
  execution, not just by reading the diff back.

**The `fiction` domain's most important addition this round:**
`release_format` (standalone / series / serialized), added directly to
the base domain, not gated behind switching to `serial-fiction`. This
exists because the user hit the actual failure once, outside this
plugin: a developmental pass flagged a serialized manuscript's
intentionally-open chapter endings as structural defects, because
nothing had told it chapters weren't meant to resolve individually.
Verified with a controlled pair, not just a single run: identical
two-section text, assessed twice, differing only in this one field —
the serialized run raised nothing about the unresolved thread; the
standalone run explicitly reasoned that the same unresolved setup "reads
as an unfinished fragment rather than an intentional hook" given the
brief ruled out serial format, folding that into a `critical` finding
about the manuscript's completeness. The field demonstrably changed the
reasoning; the fixture (two ~50-word sections) was too short to call
this a clean isolated A/B result on its own, so hold it as strong
evidence, not proof by elimination.

**`serial-fiction` domain** — hand-designed the same way `design-doc`
was (categories with the same 4–7/disputable/not-a-severity guardrails,
then generated via the templates, then verified). Reuses fiction's
`line_categories` and entire `continuity` block verbatim — sentence-level
craft and what counts as a checkable fact don't change with release
cadence, so there was no reason to invent different ones just to look
distinct. What's actually different is `developmental_categories`:
`arc_plot`, `episodic_pacing`, `character_arc`, `chapter_hook`, `stakes`,
`reader_reorientation`. `chapter_hook` is calibrated entirely by a new
`hook_expectation` brief field (every chapter / most chapters / only
when it fits) — the prompt explicitly warns against treating "must end
on a hook" as a blanket genre rule regardless of what the brief says,
which is the same category of mistake `release_format` fixes for
fiction generally. All five of `serial-fiction`'s new agents were
confirmed live to register and respond under their
`redliner:serial-fiction-<role>` ids (fifteen total across all three
domains now carry that same live confirmation, `fiction`'s and
`design-doc`'s from earlier work). `serial-fiction`'s `chapter_hook`
category was further confirmed against a live
two-chapter fixture (chapter 1 deliberately ending flat and fully
resolved, chapter 2 ending on a real reveal, `hook_expectation` set to
"every chapter"): the developmental pass correctly raised `chapter_hook`
(`major`) against chapter 1's flat ending and raised nothing against
chapter 2's — while also surfacing real `episodic_pacing`,
`reader_reorientation`, and `character_arc` findings on the same
fixture, unprompted, confirming the other five categories aren't just
schema entries nobody's prompt actually uses.

## Cowork support via an MCP server variant (DONE, v0/Python)

**Raised:** 2026-08-10. **Completed:** 2026-08-10.

The user tried installing `redliner` in Claude Cowork (a GUI/desktop
surface, not the terminal-based CLI) and it failed outright. The real
error — found in `~/Library/Logs/Claude/claude.ai-web.log`, not the
generic "Marketplace sync failed. Check the repository URL and try
again." shown in the UI — was:

> Plugin contains a top-level bin/ directory (...). claude.ai-hosted
> plugins may not ship bin/ executables because they are added to PATH
> on the CLI but are not shown on the admin approval surface. Declare
> executable entry points via hooks, commands, or mcpServers instead.

This is a deliberate Cowork content policy, not a bug: `bin/`-on-PATH is
invisible to whatever review surface an org admin uses to approve a
plugin. Checked the other two alternatives the error names before
committing to `mcpServers`: **hooks** are event-triggered (wrong shape
for on-demand operations like "check the manuscript's phase"), and
**commands** in the plugin-manifest sense just means markdown-based
skills — already what `SKILL.md` is, not a script-execution mechanism.

This mattered beyond "get Cowork working": the tool's stated primary
audience (non-technical fiction writers, students writing web serials)
is much more likely to use a normal GUI app than a CLI that requires
terminal comfort as a hard prerequisite even to install. CLI-only isn't
*less comfortable* for that audience, it's a wall — the same
generalize-without-compromising-the-primary-case test already applied
to domain generalization.

**Sequencing, per explicit direction:** this (v0) is Python, reusing the
existing `bin/schemas/*.py` logic unchanged. Porting the whole thing to
Go (v1), so the MCP server and CLI converge into one dual-mode compiled
binary with no Python dependency at all, is separate, later work — see
the "Port to a compiled language" section below, whose sequencing note
this now updates.

**What got built:** `cowork/` — a second plugin root in the same repo,
listed as a second entry (`redliner-cowork`) in `.claude-plugin/
marketplace.json` alongside the original `redliner` entry.
`cowork/mcp_server.py` exposes the exact same 10 operations the CLI
already has (`state_init/status/diff/snapshot/phase`,
`canon_stale/reconcile`, `domain_list/show`, `validate_findings`) as MCP
tools, named 1:1 with their CLI subcommands. `cowork/schemas`,
`cowork/agents`, `cowork/skills`, `cowork/redliner_canon.py`, and
`cowork/validate_findings.py` are all symlinks into the existing
`bin/`/`agents/`/`skills/` — one shared source of truth, not a fork.
`skills/run/SKILL.md`, `skills/intake/SKILL.md`, and
`skills/new-domain/SKILL.md` were rewritten to describe operations by
*intent* ("check the manuscript's current state") rather than exact CLI
syntax, so the same skill files drive either variant depending on what
tools the session actually has — proven to work by a real spike before
committing to it (see below), not assumed.

**Two spikes run first, both confirmed live before building anything
real** (the plan's own explicit gate — stop and reassess if either
failed):
1. A minimal MCP-only plugin (one dummy tool, no `bin/` anywhere) pushed
   to a standalone repo and added via Cowork's real "Add marketplace"
   GUI. Passed cleanly. (A first attempt using a branch + subdirectory of
   the main repo silently mis-tested the *original* rejection instead —
   Cowork's Add-marketplace field doesn't parse `/tree/<branch>/<path>`
   URLs, it just falls back to the repo's default branch root. A genuine
   standalone repo was needed for an unambiguous result.)
2. A real MCP server wrapping actual domain-loading logic, with a skill
   phrased entirely by intent (no tool names given). Run isolated
   (`--plugin-dir`, no `bin/` on PATH) against the real
   `sample_manuscript` — Claude picked all three correct tools
   unprompted, and every returned value matched the real `domain.json`/
   `state.json` content exactly (verified by direct diff).

**Two real bugs found by live install-testing, not by reasoning about
docs:**
1. `mcp_server.py` originally imported `redliner_canon`/
   `validate_findings` via a `sys.path` hack reaching `../bin` from its
   own directory — worked perfectly running straight from the dev source
   tree (`cowork/` and `bin/` really are siblings there), and would have
   shipped looking correct. Broke immediately on a real
   `claude plugin marketplace add` + `install` cycle:
   `ModuleNotFoundError: No module named 'redliner_canon'`, because the
   installed plugin cache only contains what's inside `cowork/`'s own
   directory — `../bin` from there lands outside the plugin entirely.
   ("Path traversal limitations" is explicitly documented behavior, not
   an edge case — plugins genuinely cannot reference files outside their
   own directory once installed.) Fixed by symlinking the two CLI modules
   directly into `cowork/` (same treatment as `schemas`/`agents`/
   `skills`) and making the import relative to the file's own directory.
   Reverified via a full clean uninstall → reinstall → cache inspection:
   both modules now arrive as real dereferenced copies, correct siblings
   of `schemas/`.
2. The original manifest's bare `python3` command assumes `mcp` is
   already installed system-wide — false on a fresh machine, exactly the
   dependency-friction concern this plan raised up front. Fixed with the
   documented, sanctioned pattern for exactly this: a `SessionStart` hook
   (`cowork/hooks/hooks.json`) that builds a venv in
   `${CLAUDE_PLUGIN_DATA}` (a data directory that persists across plugin
   updates) and installs `requirements.txt` on first run — diff-based, so
   it also re-installs when the manifest changes — with `mcpServers.
   command` pointing at that persisted venv's Python instead of bare
   `python3`. Verified live: a fresh install actually built the venv and
   installed `mcp` into it.

**One known rough edge, documented rather than engineered around for
v0:** on a plugin's very first load, the `SessionStart` hook (building
the venv, a real few seconds of `pip install`) races the MCP server's own
startup attempt — if the server tries to spawn
`${CLAUDE_PLUGIN_DATA}/venv/bin/python3` before the hook has finished
creating it, that first spawn fails silently (the interpreter it's told
to run doesn't exist yet). A restart *after* the hook completes succeeds,
which is exactly what happened in the user's own real Cowork test: first
attempt needed a manual MCP server restart, then it worked correctly.
Not fixed here — v1's Go port removes the whole hook-bootstraps-a-venv
dance entirely (a static binary has no dependency to install), so the
race disappears on its own rather than needing a bespoke synchronization
fix in Python. **Tell anyone installing `redliner-cowork`: first use may
need one MCP server restart.**

**Verification, in order of what's actually load-bearing:**
- All 10 MCP tools functionally verified against real data with exact
  parity to the CLI, including the two side-effect-based ones
  (`state_init`'s double-init error, `canon_reconcile`'s file writes
  matching what's on disk byte-for-byte).
- `claude plugin validate --strict` passes for both manifests.
- A full local marketplace install (not `--plugin-dir` — the mechanism
  that actually matters, since it's what exercises the symlink-to-copy
  and dependency-bootstrap behavior a raw source run doesn't) confirmed
  the cache contains real files, not broken symlinks.
- **The real test:** the user installed `redliner-cowork` in actual
  Cowork and, after the one-time restart above, got a correct answer to
  a question that required a real MCP tool call. This is the test
  nothing else here could substitute for — my own CLI-based attempts at
  the same check (six different invocation modes: `-p` mode, piped
  stdin, marketplace-installed, `--plugin-dir`-loaded, self-report, and
  direct scoped-name invocation) all showed the tool as genuinely
  unavailable, which in hindsight was each attempt hitting the same
  hook/server startup race fresh in an isolated one-shot process, never
  getting the "long-lived session survives past the hook, then
  reconnects" pattern a real restart provides.

## Port to a compiled language for distributable binaries?

**Raised:** 2026-08-08. **Updated:** 2026-08-08, after the plugin conversion.

The deterministic pieces are all Python (`bin/redliner_state.py`,
`bin/redliner_canon.py`, `bin/validate_findings.py`, `bin/schemas/`),
which assumes a working Python on the machine. Claude Code itself will be
present — but possibly as the desktop app rather than a terminal with a
dev toolchain, and a novelist using this is much less likely to have
Python set up than a developer is.

Since this was raised, the project moved these scripts into the plugin's
`bin/` (PATH-resolved while the plugin is enabled) specifically to fix a
cwd-dependent invocation bug. That fix is orthogonal to this question —
`bin/` is exactly where a future Go binary would live too — so it doesn't
change the recommendation below, just confirms `bin/` is the right target
directory whenever this happens.

Worth considering: port the deterministic layer to Go or Rust and ship
prebuilt binaries, so the only prerequisite is Claude Code itself.

Points in favor:
- The scripts are pure stdlib, no dependencies — a genuinely small port.
- They're deterministic by design (hashing, diffing, collision-finding),
  which is exactly the kind of code that ports cleanly and benefits from
  being a single self-contained artifact.
- The audience for a fiction-editing tool skews non-technical.
- Go in particular: trivial cross-compilation, single static binary, no
  runtime.

Points against / to think about:
- The **agent definitions and skills are markdown** and don't port at
  all — a binary only replaces the deterministic third of the system.
  The `.claude/` directory still has to be installed somehow, so "just
  download one binary" doesn't fully solve distribution even after a
  port — it solves the runtime-dependency part of it.
- Contributors editing schema vocabulary (categories, severities) would
  need a toolchain to rebuild, where today they edit a file. That's a
  real cost for a project whose vocabulary is still moving — we've
  revised it three times in one session so far.
- ~~Python-on-macOS/Linux is usually present~~ — checked, don't assume
  this. Anthropic's own *recommended* Claude Code install path (as of
  2026) is a self-contained native binary specifically so it doesn't
  depend on a Node install; a meaningful share of users will have
  Claude Code and nothing else. That's the same problem this project has
  with Python, and Anthropic's own answer was "ship a standalone
  binary" — real precedent for doing the same here, not just a guess.

**Rough recommendation when we return to this:** the destination is Go
(stdlib's `encoding/json` + `crypto/sha256` map closely onto
`schemas/*.py` already, so it should be a mechanical port, not a
redesign) — but sequence it after the schema vocabulary stops changing,
so there's one implementation to iterate on instead of two to keep in
lockstep. Treat the binary as solving the runtime-dependency problem,
not the whole install story — the markdown agents still need to land in
`.claude/` somehow either way.

**Sequencing update, 2026-08-10, now that Cowork support exists (see
"Cowork support via an MCP server variant" above):** this is no longer
just "port the CLI." There are now two Python front doors onto the same
`bin/schemas/*.py` logic — `bin/redliner_*.py` (CLI, bare executables)
and `cowork/mcp_server.py` (MCP tools) — and v1 should converge them
into **one dual-mode Go binary** that behaves as a CLI when invoked
directly and as an MCP server when given the right flag/argv, rather
than porting each front door separately. This is why Phase 1 of the
Cowork work deliberately named its MCP tools 1:1 with the CLI's
subcommands (`state_status` ~ `redliner_state.py status`) — so the
convergence is a mechanical rename, not a second design pass. Porting
also permanently removes the one real rough edge Cowork support
shipped with (the `SessionStart`-hook-builds-a-venv race on first
load) — a static Go binary has no dependency to bootstrap, so there's
nothing left to race.

## Obsidian vault integration

**Raised:** 2026-08-08

The author keeps worldbuilding notes as a markdown wiki in Obsidian.
Deliberately deferred until the internal continuity layer works.

The valuable direction is reading the vault as a **second source of
canon** — because vault-vs-manuscript contradictions are their own error
class, and arguably the more useful one: "your wiki says Selkirk was
founded 300 years ago; chapter 4 says 500." That's intent-vs-text, which
neither layer catches alone.

Constraints decided up front:
- **Read-only, always.** The vault is the author's creative work.
  redliner never writes to it.
- Don't design the fact schema around an imagined Obsidian frontmatter
  format. Read actual notes from the real vault first, then add an
  `origin` field distinguishing manuscript-derived from vault-derived
  facts.

## Permission-allowlist doesn't travel with the plugin

**Raised:** 2026-08-08

Claude Code plugin-root `settings.json` only honors `agent` and
`subagentStatusLine` — not `permissions`. So the allowlist that keeps
`bin/`'s scripts from triggering raw command-line permission prompts
(fine for a developer, opaque for a novelist) can't ship inside the
plugin itself. Current stopgap: the README documents the snippet to copy
into your own project/user settings. Worth a real fix once this has more
than one user — maybe a `/redliner:setup` step that offers to write it for
them, rather than expecting a novelist to hand-edit `settings.json`.
