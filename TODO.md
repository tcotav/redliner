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

**v1 plan, 2026-08-10.** Before planning this, checked the *installed*
`redliner-cowork` cache directly (not just reasoned about it) and found
a live bug: `domain_loader.py`'s `PLUGIN_ROOT` walk assumed `bin/`'s
nesting depth (`schemas` -> `bin` -> plugin root) everywhere, but
`cowork/` *is* its own plugin root once installed (`schemas` -> plugin
root, one level shallower) — so `domains/` was unreachable and
`domain_list`/`state_init` were silently broken for real Cowork users.
Fixed ahead of the Go work (`domain_loader.py` now searches nearby
ancestor depths for a real `domains/` dir instead of assuming one fixed
depth; `cowork/domains` symlinked in alongside `schemas`/`agents`/
`skills`), reverified with a full marketplace remove/reinstall +
direct cache inspection (not just the dev tree) — same protocol as the
two bugs Cowork support itself shipped with. Worth remembering as a
category: **anything computing a path relative to `__file__` needs to
be re-verified per plugin root**, not just per dev-tree run, because
`bin/` and `cowork/` don't nest the same way. The Go port's path
handling (below) is designed around this having bitten us twice now.

**Scope.** ~2,100 lines total: `bin/schemas/{project_state,
domain_loader, canon_schema, findings_schema}.py`, `bin/redliner_{state,
canon,domain}.py`, `bin/validate_findings.py`, `cowork/mcp_server.py`.
All pure stdlib (json, hashlib, pathlib, re) except the MCP server's
`mcp` dependency, which Go replaces with a real import compiled into the
binary rather than a runtime dependency — the whole reason this port is
worth doing.

**Layout: one Go source tree, one binary, two installed copies — not a
symlink.** `bin/` still can't exist in `redliner-cowork` (Cowork's
content-policy rejection this whole thing started from, see "Cowork
support" above), so the built binary lands at both `bin/redliner` and
`cowork/redliner` as real files, same as `cowork/schemas` etc. already
get copied rather than symlinked once Claude Code's installer
dereferences them. Building both from one `go build` invocation with two
output paths keeps this from becoming two things to maintain.

**CLI shape — decided: subcommands, not argv[0]-dispatched script-name
symlinks.** `redliner state status <dir>`, `redliner state init <dir>
[domain]`, `redliner state diff/snapshot/phase`, `redliner canon
stale/reconcile`, `redliner domain list/show`, `redliner validate
<dir>`, plus `redliner mcp` for the Cowork stdio-server mode. Checked
first whether keeping the four old script names (`redliner_state.py`
etc.) as symlinks would avoid touching call sites — mostly not code,
just prose (`skills/*/SKILL.md`, `README.md`, `.claude/settings.json`'s
permission allowlist), a small and mechanical rename either way — and
symlinks lose on their own terms besides: the cache dereferences them
into full binary copies, so four names means 4x the binary size per
plugin root for no benefit. One binary, subcommands, update the handful
of prose/settings references as part of cutover.

**Path resolution: explicit contract, not a ported `__file__` walk.**
Given the bug just fixed above, the Go binary doesn't try to infer its
plugin root by walking a fixed number of parent directories at all.
Order: `$REDLINER_DOMAINS_DIR` env override, else search upward from
`os.Executable()` (symlink-resolved) for the nearest `domains/`
directory that actually exists, else fail with an error naming every
path it checked. Same pattern for locating `domains/` from either
`bin/redliner` or `cowork/redliner` without special-casing which plugin
root it's running from.

**Known porting hazards, not just mechanical translation:**
- **CRLF hashing mismatch.** Python's `Path.read_text()` does universal
  newline translation (`\r\n` -> `\n`); Go's `os.ReadFile` doesn't. A
  Windows-authored or Word-exported manuscript would hash differently
  under Go and every section would false-flag as "changed" on first
  `state diff` after cutover. Normalize line endings before hashing and
  before word-counting; put a CRLF fixture in the differential harness
  (below) so this is caught by a test, not a user report.
- **JSON key order.** Go's `map[string]any` marshals keys sorted
  alphabetically; Python dicts preserve insertion order. Don't chase
  byte-for-byte parity on `state.json`/`canon.json`/`collisions.json` —
  nothing but `redliner` itself reads those files, so it's not a real
  compatibility requirement. Use structs (not maps) for anything with a
  deliberate field order so the *Go* output is intentional, not an
  artifact of map iteration.
- **Timestamps.** Python's `datetime.now(timezone.utc).isoformat()`
  (`...+00:00`, microseconds) has no exact Go equivalent format; pick an
  RFC3339 representation and treat the change as intentional, not a
  parity target.
- **MCP tool descriptions are a frozen interface, not incidental
  docstrings.** The Cowork spike's load-bearing result was Claude
  picking all three correct tools *unprompted*, off the Python
  docstrings in `mcp_server.py`. Carry the 10 tool names and their full
  description text over verbatim into the Go MCP SDK's tool
  registration — a terser auto-generated description degrades tool
  selection in a way nothing else in this plan would catch.
- **Human-facing stdout strings are a compatibility surface.** Skill
  prose and this project's own live-verification habit both
  pattern-match on exact CLI output (`Canon: N entities, M facts.`,
  `Phase: X -> Y (...)`, `OK   `/`FAIL ` lines). Preserve these exactly;
  they're cheap to keep and expensive to silently drift.

**Differential harness — build this first, while Python still exists to
diff against.** There's no `tests/` directory in this repo; the existing
regression discipline is the live-verification protocol used throughout
(see e.g. the two Cowork bugs above, both caught by real install cycles,
neither by reasoning about docs). Same approach here: run all 10
operations through both implementations over a fixture set —
`sample_manuscript`, a CRLF variant, a section-stem collision
(`SectionCollisionError`), a fresh dir with no state, a double-`init`,
and a manuscript with `canon/observations/` that actually produces
collisions — and diff. Compare **parsed JSON with timestamps stripped**
(not bytes, per the key-order point above); compare **stdout strings
exactly** for the human-facing prints. This harness is also the gate
before deleting any Python — not an afterthought after the port "looks
done."

**Phases:**
1. Go module scaffold + differential harness against current Python
   (no Go implementation yet — proves the harness itself is honest).
2. Port `schemas/` (`project_state`, `domain_loader`,
   `canon_schema`, `findings_schema`) as a Go package. Pure data +
   validation, no CLI/MCP surface yet. Handles the CRLF and path-
   resolution hazards above at this layer so everything built on top
   inherits the fix.
3. Port the CLI subcommands (`state`, `canon`, `domain`, `validate`)
   against schema harness data. Diff against `bin/redliner_*.py` output
   via the harness.
4. Port the MCP server (`redliner mcp`) using a Go MCP SDK, tool names
   and descriptions carried over verbatim (see above). Diff against
   `cowork/mcp_server.py`'s 10 tools via the harness.
5. **Gate: full uninstall -> marketplace install -> cache inspect ->
   live Cowork query cycle**, same protocol that caught both real
   Cowork-support bugs and the `domains/` bug this session. Not the
   final step — do this *before* deleting any Python, since the port has
   the identical failure mode available to it (a fix that's correct in
   the dev tree and wrong in the installed cache).
6. Cross-compile for **darwin/arm64 only, to start** (matches the dev
   machine; other platforms stay on v0/Python until this expands —
   revisit once the port itself is proven).
   **Superseded, 2026-08-12:** this item originally said "commit both
   `bin/redliner` and `cowork/redliner` as prebuilt binaries" as the
   settled answer, on the reasoning that download-on-first-run or
   build-from-source would both reintroduce the runtime-dependency
   problem this port exists to remove. That reasoning still holds for
   *build*-from-source, but a **download** of a prebuilt static binary
   isn't the same class of problem as Python's venv/pip bootstrap was
   (no toolchain, no compiler, one HTTP GET + chmod) — worth a real
   GitHub Actions release workflow + a lightweight download-on-install
   hook instead of committing binaries to the tracked tree
   permanently. Not built yet. Currently: `bin/redliner`/
   `cowork/redliner` *are* committed on `go-port-v1`, but as a
   deliberately temporary measure to unblock the Phase 5 live-Cowork
   test (see the 2026-08-12 progress note below) — not a decision that
   this is where they'll stay.
7. Cutover deletions, explicit so nothing gets left half-migrated:
   `bin/redliner_*.py`, `bin/schemas/`, `cowork/mcp_server.py`,
   `cowork/hooks/hooks.json` (no venv bootstrap needed — the whole
   first-load race this shipped with disappears), `cowork/
   requirements.txt`, and the `schemas`/`redliner_canon.py`/
   `validate_findings.py`/`domains` symlinks in `cowork/` (now real
   files inside the Go binary instead). **Keep** `cowork/agents` and
   `cowork/skills` — markdown doesn't port. Update `.claude/
   settings.json`'s permission allowlist and the README's copy-paste
   snippet for the new subcommand names.

   **DONE 2026-08-12, but not as written — two items here were wrong,
   and acting on them literally would have broken things:**
   - `cowork/hooks/hooks.json` was **not** deleted. It was repurposed
     into the Go binary's download hook; deleting it breaks Cowork's
     install entirely.
   - The `cowork/domains` symlink was **not** deleted. Its stated
     justification ("now real files inside the Go binary") is false —
     nothing is `go:embed`ed, the binary reads `domains/` off disk.
     Deleting it hard-breaks the Cowork MCP server, which is exactly the
     bug that shipped and had to be fixed the same day.
   - `cowork/requirements.txt` was already gone.
   - The Python was **relocated, not deleted**: `bin/redliner_*.py` and
     `bin/schemas/` now live in `go/harness/python-baseline/` as the
     frozen oracle the goldens are captured from. The mandatory part was
     getting them out of `bin/`, which is the CLI plugin's PATH
     directory — executable `*.py` there let a session invoke Python and
     hit the very runtime dependency this port removes. See
     `go/harness/README.md`'s "The Python baseline".
   - Deleted for real: `cowork/mcp_server.py` and the
     `schemas`/`redliner_canon.py`/`validate_findings.py` symlinks —
     genuinely dead once `mcpServers.command` became the Go binary, and
     until now shipped to every Cowork user as unused code.
   - Allowlist and prose updated: `.claude/settings.json` collapses to
     `Bash(redliner *)`, and `skills/run`, `skills/intake`,
     `skills/new-domain` plus the README no longer name `.py` commands.
     `agents/` needed no changes — it never referenced them.

**Progress, 2026-08-12.** Phases 1–4 done and committed on `go-port-v1`
(module scaffold + differential harness; `internal/schemas`;
`internal/cli`; `internal/mcpserver`) — all four verified against real
Python-captured golden data, not just read-through, including the
MCP tool descriptions checked against Python's actual docstrings via
`ast.get_docstring` rather than a second copy of the same Go constants.

**Phase 5 gate passed for real**, same day: `bin/redliner` and
`cowork/redliner` built and wired additively into both plugin manifests
(nothing Python removed — see the two-item exception list below),
verified against a real local marketplace install/cache first (exec
bit intact, `domain list`/`state status` correct, a genuine in-process
MCP stdio round-trip with no `REDLINER_DOMAINS_DIR` override), then
pushed to `go-port-v1` and tested in the **actual Cowork GUI app** —
the repo's default branch was temporarily pointed at `go-port-v1`
(reversible; Cowork's "Add marketplace" only reads a repo's default
branch, not arbitrary branch URLs, the same finding from the original
Cowork-support work) since a real GUI install needs a GitHub-hosted
marketplace, not the local-directory source Claude Code CLI testing
used. Asked "what domains are available?" and got the three real
domains back, correctly described, no MCP server restart needed this
time (the original venv-bootstrap first-load race this was worried
about inheriting doesn't apply — there's no venv build step left to
race). This is the test TODO.md itself says nothing else can
substitute for.

Two exceptions to "nothing Python removed yet": `bin/redliner` and
`cowork/redliner` (the compiled binaries) are committed to `go-port-v1`
as a **deliberately temporary** measure to unblock this test —
committing them now and deleting later does not reclaim git history
size on its own (blobs persist without a rewrite), accepted for now
since a history rewrite is cheap before this repo has wider visibility
and expensive after. **Correction, 2026-08-12: the install-cost half of
that reasoning is wrong — see "What installing actually costs" below.
Marketplace clones are shallow, so historical blobs are never fetched
and a rewrite buys nothing for users.** Real follow-up, not yet built: a GitHub Actions
release workflow plus a lightweight download-on-install hook, so
binaries stop needing to live in the tracked tree at all.

**Remaining before Phase 7's cutover deletions:** the release-automation
follow-up above; reverting the GitHub default branch back to `main`
once `go-port-v1` testing is done; and — separately — actually merging
`go-port-v1`, which hasn't happened yet.

*(Update, 2026-08-12: the release-automation follow-up is done — see the
section below. The default branch is back on `main`, done manually
outside this repo's history, confirmed via the GitHub API. Merging
`go-port-v1` is still outstanding.)*

**Release automation + download hooks, done, 2026-08-12** (the item
right above, now built): `.github/workflows/release-go-binaries.yml`
cross-compiles `go/cmd/redliner` on `ubuntu-latest` (Go doesn't need a
matching-OS runner) and attaches the binary + `checksums.txt` to a
GitHub Release on a `v*.*.*` tag push, or uploads as a build artifact on
manual `workflow_dispatch` for testing without cutting a real release —
verified both paths for real (a `workflow_dispatch` test run, then a
real `v0.1.0` tag) before wiring anything to depend on it. The
`bin/redliner`/`cowork/redliner` binaries committed on 2026-08-12 as a
stopgap are now removed from the tracked tree — replaced by
`hooks/bootstrap-redliner-binary.sh`, a `SessionStart` hook script that
downloads the release asset matching the plugin's own
`.claude-plugin/plugin.json` version, verifies its checksum, and installs
it, refusing to install anything that doesn't check out.

**The two front doors needed genuinely different destinations, not the
same fix copy-pasted**, because of a fact checked against Claude Code's
own docs rather than assumed: `${CLAUDE_PLUGIN_DATA}`/`${CLAUDE_PLUGIN_ROOT}`
are exported to hook and MCP/LSP subprocess commands *only* — never to a
bare `bin/`-invoked executable's own environment. So a `bin/redliner`
wrapper script can't locate a `${CLAUDE_PLUGIN_DATA}`-based path at
invocation time no matter how it's written. The fix: the hook (which
does get the env vars) downloads straight into
`${CLAUDE_PLUGIN_ROOT}/bin/redliner` for the CLI plugin — overwriting/
creating the real file in place, confirmed writable at runtime by a live
test before committing to the design, not assumed — so by the time any
command runs, it's just a real binary on PATH, no wrapper and no
runtime env-var dependency. The Cowork plugin has it easier:
`mcpServers.command` supports the same `${CLAUDE_PLUGIN_DATA}`
substitution a hook gets, so the hook downloads to
`${CLAUDE_PLUGIN_DATA}/bin/redliner` and the manifest just points there
directly, same shape as the venv hook it replaces
(`cowork/hooks/hooks.json`'s old venv-bootstrap command is gone; so is
`cowork/requirements.txt`).

**A genuinely unresolved reliability problem, disclosed rather than
downplayed:** across roughly seven clean uninstall → reinstall →
one-shot-session test cycles, the CLI plugin's (`redliner`, source
`"./"`) `SessionStart` hook fired successfully exactly **once**; the
Cowork plugin's (`redliner-cowork`, source `"./cowork"`) identically-
shaped hook fired successfully **every single time**, no exceptions.
This is not the one-time first-load race the venv-era caveat described
— a second session after a failure did not reliably fix it either.
Ruled out as causes: JSON syntax, exec bits, an explicit `"hooks"`
field in `plugin.json`, the other plugin's presence or absence,
inline-vs-external-script hook commands, `bash <script>` vs direct
execution, and simple retry (a second session sometimes still fails
right after a first failure). None of these changed the outcome.

**Leading unconfirmed hypothesis:** the CLI plugin's source is the
entire repo root — including `go/`'s full source tree, `README.md`,
`TODO.md`, `sample_manuscript/`, and a full nested copy of `cowork/`
itself — while the Cowork plugin's source is just the lean `cowork/`
subdirectory. If plugin content scanning/copying time affects whether a
`SessionStart` hook completes before some internal deadline, the much
larger plugin would be exactly the one to see this. Not verified by a
controlled test (e.g., trimming the CLI plugin's shipped content and
re-measuring hook success rate) — that's the next real step if this
gets picked back up, not committing to the theory further without it.

**Practical implication, worse than originally written here:** this
isn't "a restart fixes it" — it's "the CLI plugin's binary may simply
not be there most of the time, for reasons not yet understood, and a
user hitting `command not found` has no documented recovery step beyond
retrying." **Not safe to treat as solved.** Before this ships to anyone
but the maintainer: either confirm and fix the root cause, or fall back
to committing `bin/redliner` for the CLI plugin specifically while
keeping the Cowork plugin on the (reliably-working) download hook —
a partial, honest middle ground rather than a design that silently
fails most of the time.

Not yet done: reflecting this in `README.md`'s existing "first
use may need one MCP server restart" caveat (needs generalizing to
cover the CLI plugin too, now that it has the same class of race) — a
Phase 7 README pass, not done here.

**The size hypothesis, tested 2026-08-12: inconclusive, and the test
method itself turned out to be the wrong instrument.** Built a trimmed
spike plugin (`redliner-lean`, ~47 files/312KB — comparable to
Cowork's ~43 files/300KB, vs the full CLI plugin's 127 files/~1.6MB
installed) registered as its own local-directory marketplace, then ran
repeated clean cycles (`rm` the downloaded binary, fresh `claude -p`
one-shot session, check for the binary) against both the lean and
full-size plugin. First pass falsely showed 0/6 and 0/3 — checking the
wrong path, caught before drawing conclusions from it (a marker line
added to the top of a spike copy of the bootstrap script proved the
hook *was* firing every time; it was writing into `$CLAUDE_PLUGIN_ROOT`,
which for a local-directory-source marketplace resolves to the live
source directory, not the `~/.claude/plugins/cache/...` copy that
exists alongside it — that cache copy appears to be vestigial for local
dev sources, not what hooks actually execute against). After fixing the
check path: **6/6 successes**, lean and full alike, no failures at all.

That result doesn't confirm the size hypothesis, but it doesn't
resurrect it either — the original 1/7-vs-7/7 finding was likely
observed via real interactive one-shot sessions (open a session, send
one message, close), while this retest used `claude -p` headless
one-shot invocations against local-directory marketplace sources
exclusively. Both differences are real candidate confounds: `-p` mode
may not race the same internal deadline an interactive session start
does, and a local-directory source skips whatever
scanning/copying-into-cache step a real git/GitHub-hosted marketplace
install goes through (which is what the original test's "clean
uninstall → reinstall" cycles likely exercised, and what the CLI
plugin's real users will always go through — nobody installs redliner
from a local directory). So this test answered "does trimming help
under `-p` + local-directory sources" (no signal either way, everything
passed) without touching the conditions that produced the original
failure.

**Decision: not pursuing this further right now.** A test that actually
reproduces the original conditions would need either a real interactive-
session harness or a GitHub-hosted marketplace pointed at a trimmed
branch/tag — both bigger lifts than this spike, for a bug that already
has a named, working fallback. Taking the fallback instead: commit
`bin/redliner` for the CLI plugin specifically (stop depending on its
`SessionStart` hook), keep the Cowork plugin on the download hook,
which has never failed. Root cause stays open for whoever hits this
again with more budget to spend on it — the local-directory-source
caveat above (cache copy isn't what hooks run against) is worth keeping
regardless of what happens with the size question, since it'll trip up
any future local-marketplace test the same way it tripped up this one.

**Fallback implemented, 2026-08-12.** `bin/redliner` and
`bin/redliner.version` are committed directly again (removed from
`.gitignore`, which now only excludes `cowork/redliner*`); the CLI
plugin's `hooks/hooks.json` and `hooks/bootstrap-redliner-binary.sh` are
deleted outright rather than left as dead weight, since nothing calls
them once the binary ships in the tree. This reintroduces the ~8.3MB
binary into git history (same tradeoff Phase 5 accepted the first time:
a history rewrite is cheap now, expensive after wider visibility — still
true). **Correction, 2026-08-12: that parenthetical is wrong about
install cost, twice repeated and load-bearing, so stated plainly here —
what a rewrite would fix is repo size for full-clone contributors, not
anything a plugin user downloads. See "What installing actually costs"
below.** The Cowork plugin was **not** untouched, contrary to what this
paragraph originally claimed — deleting the repo-root `hooks/` script
left `cowork/hooks/bootstrap-redliner-binary.sh` (a symlink to it)
dangling and silently stopped Cowork's binary from downloading at all.
Fixed the same day; see the two-bug entry above.
Verified `./bin/redliner domain list` runs correctly post-change.
Re-bumping `bin/redliner` on future Go changes now means rebuilding and
re-committing it directly (`go build -o bin/redliner ./go/cmd/redliner`
or whatever the current build command is) instead of relying on the
release/download pipeline for the CLI plugin specifically — the release
workflow and Cowork's download hook still exist and still matter, just
not for this front door anymore.

**Two Cowork-breaking bugs, found and fixed 2026-08-12, both shipped to
`main` before being caught.** Found while checking whether Phase 7 could
safely delete the `cowork/domains` symlink — i.e. by questioning a
stated assumption, not by a test that was designed to catch either.

1. **The Cowork MCP server didn't start at all.** Phase 7's item list
   says the `domains` symlink can go because domains are "now real files
   inside the Go binary instead." **That is false** — nothing is
   `go:embed`ed; `schemas.FindDomainsDir()` searches the filesystem,
   walking up 4 levels from the binary. That worked when the Cowork
   binary lived at `cowork/redliner`, a sibling of `domains/`. The
   download-hook change earlier the same day moved it to
   `${CLAUDE_PLUGIN_DATA}/bin/redliner` — a different tree entirely,
   with no `domains/` anywhere in its walk-up — and `main.go`'s
   `runMCP()` returns 1 *before serving a single request* when
   resolution fails. So Cowork was hard-broken, not degraded. The
   Phase 5 live-Cowork test that passed ran *before* that move and was
   never re-run after it: exactly the "correct in the dev tree, wrong in
   the installed cache" failure mode the Phase 5 gate exists to catch,
   reintroduced by a later change that didn't re-run the gate.
   **Fix:** `cowork/.claude-plugin/plugin.json`'s `mcpServers.redliner`
   now sets `"env": {"REDLINER_DOMAINS_DIR": "${CLAUDE_PLUGIN_ROOT}/domains"}`,
   bridging the DATA tree (binary) to the ROOT tree (domains). That
   `env` values get `${CLAUDE_PLUGIN_ROOT}` substitution was **confirmed
   against Claude Code's plugin reference docs** before relying on it,
   not assumed — the docs' component table lists `command`, `args`, and
   `env` as substituting fields for stdio MCP servers.

2. **`cowork/hooks/bootstrap-redliner-binary.sh` was a symlink to the
   repo-root `hooks/` copy, which the CLI-fallback commit deleted** —
   leaving it dangling, so Cowork's binary silently stopped downloading
   at all. That commit's message claiming Cowork was "untouched" was
   wrong. **Fix:** it's a real file in `cowork/hooks/` now, not a
   symlink, so a change aimed at the CLI variant can't break Cowork's
   install again. (The `cowork/` symlinks that *remain* — `agents`,
   `skills`, `domains`, and the Python ones — are all intra-repo and
   fine; this one was the odd case of a symlink into a directory only
   the other plugin needed.)

**Verified end-to-end, not by inspection:** clean uninstall → reinstall
→ fresh session (hook re-downloads the binary into the DATA dir) →
`claude mcp list` reports `plugin:redliner-cowork:redliner … ✔ Connected`,
where it previously reported `✘ Failed to connect`. Connection success
is itself the proof the domains fix works, since the server exits before
serving when `FindDomainsDir()` fails and the DATA-tree walk-up
demonstrably finds nothing.

**Lesson worth keeping for Phase 7:** the deletion list's parenthetical
justifications are load-bearing claims, and at least one was false.
Re-verify each against the code before deleting anything — particularly
anything the Go binary reads from disk at runtime rather than embedding.
`domains/` must **not** be deleted from either plugin.

## What installing actually costs (measured, 2026-08-12)

Written because two separate decisions above were justified by a belief
about install cost that turned out to be false, and the belief kept
getting restated. Numbers here are measured on this machine, not
reasoned from how git "should" work.

**Marketplace repos are cloned shallow.** `~/.claude/plugins/marketplaces/
thedotmack` is a depth-1 clone (`git rev-list --count HEAD` = 1, `.git/
shallow` present); `anthropic-agent-skills` likewise shallow at 12
commits. `claude-plugins-official` isn't a git clone at all — no `.git`,
a `.gcs-sha` marker instead, so it's fetched as a snapshot from object
storage. **Consequence: historical blobs are never fetched.** The ~8.3MB
binaries sitting in this repo's history cost a plugin user exactly
nothing, and a history rewrite would not change a single byte they
download. A rewrite is only worth doing for full-clone contributors —
a real but different problem.

**A depth-1 clone of this repo, measured directly:** 4.6MB `.git` +
~9.4MB worktree ≈ **14MB**, against 16MB of `.git` in the working tree
that a shallow clone never touches. What dominates is the *tip snapshot*:
the committed `bin/redliner` (8.3MB) is most of it. Incidental baggage
in the CLI plugin's shipped content (`go/` ~992K, `sample_manuscript/`
88K, the nested `cowork/` 52K, docs ~84K) totals only ~1.2MB.

**So the levers, in order of actual effect:**
1. Not committing the binary — 8.3MB, dwarfs everything else.
2. Trimming baggage — ~1.2MB.
3. Rewriting history — **0 bytes** for users.

**Convention check, since it kept coming up:** across the 287 plugins in
`claude-plugins-official`, ~52% use a whole-repo source (`url`/`github`)
and ~48% scope to a subdirectory (`git-subdir` 29%, an inline subdir
path 18.5%). **Zero** use an inline `"./"` root — which is what this
repo's CLI plugin does. The whole-repo half are dedicated plugin repos,
where the repo *is* the plugin. Sizes vary hugely (Anthropic's own
`example-skills` installs at 33MB; `rust-analyzer-lsp` at 92K), so the
convention is not "make it small" — it's "the source is the plugin and
not much else." `cowork/` at 304K already satisfies this; the repo-root
CLI plugin at ~10MB does not.

**Implication for the restructure idea (`plugins/redliner` +
`plugins/redliner-cowork`):** it would *not* reduce what anyone
downloads, because the marketplace is this repo — adding it clones the
whole thing at depth 1 regardless of internal layout. It only shrinks
the local cache *copy*. Worth doing for convention-conformance and
because it's the only honest way to test the hook-size hypothesis, but
not for bandwidth. See the release-repo note below for the lever that
does move the number.

## Release repo + publish workflow (sketched 2026-08-12, NOT built)

> **DOWNGRADED the same day, before any of it was built — read
> "Validating the release-repo plan" below first.** Measuring the thing
> this plan was justified by showed the binary alone accounts for
> essentially all of the install cost: repo tip is **9.7MB with the
> committed binary, 1.3MB without it**. A release repo saves only
> ~0.9MB *beyond* simply not committing the binary, which does not
> justify a content-mirroring CI pipeline and a second repo to keep in
> sync. Keep this section for the mechanics (the `cp -rL` symlink trap
> and the validation gate are still correct and still worth reusing),
> but **do not build it as the next step.**

The lever that actually reduces install cost, per the measurements
above. Sketched in enough detail to pick up cold; nothing here is
implemented yet.

**The idea:** a second, generated repo containing *only* plugin content.
The working repo (`tcotav/redliner`) keeps everything — `go/` source,
the differential harness, `sample_manuscript/`, Python, docs. The
release repo holds what a user installs and nothing else, which makes it
lean **by construction** rather than by remembering to trim.

**Why it's worth doing, stated precisely:** it lets the binary leave the
tracked tree, because a lean plugin can go back to the download hook.
~14MB → ~300KB, the same shape and size as `cowork/` (which has a 100%
hook success rate). Every other lever is worth ~1.2MB or, in the case of
a history rewrite, exactly zero.

**Shape of the release repo** (`tcotav/redliner-plugins`, say):

```
.claude-plugin/marketplace.json      two entries, subdir sources:
                                     ./plugins/redliner, ./plugins/redliner-cowork
plugins/redliner/                    CLI variant
  .claude-plugin/plugin.json
  agents/ skills/ domains/           real files, never symlinks
  hooks/hooks.json
  hooks/bootstrap-redliner-binary.sh
plugins/redliner-cowork/             MCP variant
  .claude-plugin/plugin.json         mcpServers + the REDLINER_DOMAINS_DIR env fix
  agents/ skills/ domains/
  hooks/...
```

No `go/`, no harness, no `sample_manuscript/`, no `bin/*.py`, no
committed binary. This also incidentally puts both plugins on
subdirectory sources, matching what ~48% of the official marketplace
does and what 0/287 of it does with an inline `"./"` root.

**Publish workflow** (`.github/workflows/publish-plugins.yml`, in the
*working* repo):

1. Trigger on a `v*.*.*` tag push, after `release-go-binaries.yml` has
   built and attached the binaries — ordering matters, the plugins are
   useless if their matching release assets don't exist yet.
2. Stage the release tree with **`cp -rL`** (dereference symlinks).
   **This is the step most likely to break Cowork** — `cowork/`'s
   `agents`/`skills`/`domains`/`schemas` are symlinks that Claude Code's
   own installer dereferences into real files. Mirror tooling that
   copies them *as symlinks* produces a plugin that works in the dev
   tree and dangles once installed: exactly the failure that hit twice
   on 2026-08-12. Assert no symlinks survive in the staged tree before
   pushing.
3. Stamp the version into both `plugin.json`s from the git tag. Side
   benefit: kills the "version lives in three places" hazard the
   bootstrap script's comments already worry about.
4. Push to the release repo (`rsync --delete` into a checkout, commit,
   tag identically). Needs a PAT or deploy key with push rights, stored
   as a secret.

**Keep GitHub Releases on the working repo.** The bootstrap script's
`REPO="tcotav/redliner"` stays as-is, so release *assets* and plugin
*content* live in different repos. That's fine and worth stating
explicitly so nobody "fixes" it later.

**The gate before trusting any of this** — the lesson of 2026-08-12,
where a change that was correct in the dev tree hard-broke the installed
plugin and the Phase 5 gate wasn't re-run:

- Install from the real GitHub-hosted marketplace, not a local directory
  (local-directory sources resolve `${CLAUDE_PLUGIN_ROOT}` to the live
  source dir and skip the cache copy entirely — they cannot reproduce
  install-time behavior; this is what invalidated the first attempt at
  the size-hypothesis test).
- `claude mcp list` must report the Cowork server **Connected** — that
  alone proves domain resolution, since `runMCP()` exits before serving
  when `FindDomainsDir()` fails.
- CLI: a bare `redliner domain list` must work, proving the hook
  downloaded the binary and PATH picked it up.
- Confirm no symlinks survived into the installed cache.
- **Run N clean uninstall/reinstall/session cycles and count hook
  successes.** This *is* the size-hypothesis experiment, finally under
  conditions that can reproduce the failure.

**The decision it resolves:** hook reliable at ~300KB → the binary never
goes back in git, and the CLI plugin's `hooks/` returns. Still failing at
~300KB → the size hypothesis is dead for good; commit the binary into the
*release* repo only, leaving the working repo clean. Either outcome is
better than today's.

**Costs, named up front rather than discovered later:** two repos to keep
in sync, with the release repo strictly generated (hand-edit it and the
next publish silently reverts you — worth a `DO NOT EDIT` header in its
README and in each generated `plugin.json`). Existing users' marketplace
URL changes, so migration needs a note in the README. And Cowork's
"only reads a repo's default branch" constraint still applies to the
release repo, so its default branch must be the one publishes land on.

## Validating the release-repo plan (2026-08-12) — it didn't survive

The plan above was written, then stress-tested before building anything.
Most of its *mechanics* held up; its *justification* did not. Recorded in
full because the wrong version is the intuitive one and will otherwise
get re-derived.

**What held up:**

- Staging the release tree with `cp -rL` works: the staged two-plugin
  tree is **376K**, and `find -type l` over it returns nothing, so
  Cowork's symlinks do get dereferenced into real files as required.
- `git-subdir` plugin sources persist as **content only** — installing
  `airtable@claude-plugins-official` (a `git-subdir` entry) produced a
  500K content directory with no `.git` anywhere, and left no clone of
  the upstream repo behind. Whether the *transient* fetch pulls the whole
  repo was not determined (the test repo was only 178K — too small to
  discriminate); don't claim either way.

**What killed the plan — one measurement.** `git archive` of the tip
with and without the committed binary:

| repo tip | size |
| --- | --- |
| current `HEAD` (binary committed) | **9.7MB** |
| `0ce8548` (before the binary) | **1.3MB** |

So the binary *is* the install-cost problem, essentially in full.
Dropping it makes the whole repo small enough that **no restructuring
and no second repo are needed at all** — a plain shallow clone would be
~2MB. The release repo's remaining advantage over "just don't commit the
binary" is ~0.9MB, against the cost of a mirroring pipeline, a second
repo that must be treated as generated, and a marketplace-URL migration
for existing users. Not worth it. This was an over-engineered answer
reached by anchoring on the 14MB figure without first asking how much of
it was one file.

**Which puts the whole question downstream of a single decision:** is
the binary committed, or downloaded by the hook? Everything else was
architecture astronomy on top of that.

**And the reason the binary is committed is now itself in doubt.** The
`SessionStart` hook's alleged ~1/7 CLI success rate was the sole
justification. Three findings from the same day undercut it:

1. **`bin/` does *not* need to exist at plugin load for PATH to work** —
   the leading alternative root cause, and it's false. Tested directly
   with a throwaway plugin (`pathtest`) shipping **no `bin/` directory**
   and a `SessionStart` hook that creates `bin/pathtool` at startup: in
   that *same* session, `command -v pathtool` resolved to the
   hook-created file. PATH is not frozen at load time. The docs are
   silent on this, so it had to be tested, not looked up.
2. **No documented plugin-size limit** affects plugin load or hook
   execution (checked against the official plugins/hooks reference), and
   the default command-hook timeout is **600 seconds** — far too generous
   for "plugin content scanning pushed the hook past a deadline" to be
   plausible. The size hypothesis is now very weak, independent of the
   inconclusive lean-vs-full test.
3. **The CLI hook succeeded 3/3 once measured at the right path.** The
   earlier 0/6 and 0/3 results were an artifact of checking the
   `~/.claude/plugins/cache/...` copy, while a local-directory
   marketplace actually resolves `${CLAUDE_PLUGIN_ROOT}` to the live
   source directory.

**Leading hypothesis now: the original ~1/7 was a measurement artifact**
of exactly the same kind as #3 — checking the cache copy while the
binary was landing in the source tree. Not proven; the original
conditions (real GitHub-hosted marketplace, repeated clean installs)
have never been re-run, and that's the *only* test that settles it.

**So the actual next step is small, not architectural:**
restore the CLI plugin's `hooks/` (deleted earlier the same day),
un-commit `bin/redliner`, and verify with **real GitHub-hosted
marketplace install cycles** — the one condition never properly tested.
If the hook holds up there, the repo tip drops to ~1.3MB, the binary
leaves the tree, and the release repo, the `plugins/` restructure, and
the history rewrite are all moot. If it genuinely fails there, *then*
revisit — with a real failure to diagnose rather than a suspected one.

## RESOLVED: the hook reliability problem was a measurement artifact

**2026-08-12.** The test that settles it was finally run under the
conditions that had never been used: a **real GitHub-hosted marketplace**
(`claude plugin marketplace add tcotav/redliner`), with the binary
un-committed and both plugins on their download hooks.

**7 clean uninstall → reinstall → session cycles: CLI 7/7, Cowork 7/7.**
No failures. Against an alleged ~1/7 CLI success rate that justified
committing an 8.3MB binary and sketching an entire release-repo
architecture.

Both front doors then verified working, not merely present:

- `claude mcp list` → `plugin:redliner-cowork:redliner … ✔ Connected`
  (which also re-proves the `REDLINER_DOMAINS_DIR` fix, since the server
  exits before serving if domain resolution fails).
- A bare `redliner domain list` inside a real session returned the three
  domains — so the hook-downloaded binary landed in the plugin's `bin/`
  and PATH picked it up, with no wrapper and no committed binary.

**The original ~1/7 finding is retired.** The leading explanation is the
measurement artifact described in the validation section above: under a
**local-directory** marketplace, `${CLAUDE_PLUGIN_ROOT}` resolves to the
live source directory, while the `~/.claude/plugins/cache/...` copy that
also exists is *not* what hooks execute against. Checking the cache copy
shows "no binary" while the hook is succeeding a few directories away.
The same mistake was made and caught twice on 2026-08-12 before being
recognized as the likely cause of the original result too.

**Measured payoff, on the real hosted install:**

| | before | after |
| --- | --- | --- |
| repo tip | 9.7MB | **1.4MB** |
| marketplace clone (depth-1, measured) | — | **1.7MB** (344K `.git`) |
| CLI plugin cache | 10MB | **1.4MB** |
| Cowork plugin cache | 304K | 256K |

The hosted clone was confirmed shallow (`rev-list --count HEAD` = 1,
`.git/shallow` present) — the prediction that drove the whole install-cost
analysis, now verified against this repo rather than inferred from others.

**Therefore, all of the following are moot and should not be built:** the
release repo + publish workflow, the `plugins/<name>/` restructure, and
the git history rewrite. Each existed to work around a problem that was
not real. What remains true and worth keeping is the *reasoning* recorded
above about why they wouldn't have helped much anyway.

**Methodological note, the actual lesson of the day.** Three separate
false beliefs drove real decisions here: "domains are embedded in the
binary" (they are not), "history rewrite reduces install cost" (it does
not — clones are shallow), and "the download hook is unreliable" (it is
not — the measurement was wrong). Each was stated confidently in this
file and each survived because it was never checked against the system.
The fix that worked, every time, was running the thing rather than
reasoning about it — and checking *where the system actually writes*
before concluding it didn't write anything.

## Continuity misses contradictions when extractions name things differently

**Raised 2026-08-12, from the first full pass on a fresh manuscript
(below). Not fixed.** This is a **recall** bug, and the more serious of
the two findings from that run, because nothing downstream catches it.

A contradiction was deliberately planted in a scratch manuscript: a tide
clock described as stopped for **eleven years** in section 1 and
**fifteen years** in section 3. The continuity layer extracted both facts
correctly and then **never collided them**:

| | section_01 | section_03 |
| --- | --- | --- |
| `entity` | `tide clock` | `the tide clock` |
| `attribute` | `duration_not_working` | `stopped_duration` |
| `value` | `eleven years` | `fifteen years` |

Collision detection requires an exact match on **both** entity and
attribute. The definite article differs, and the two attribute names are
synonyms chosen independently. So a genuine, blatant contradiction — the
exact error class this layer exists to catch — passed silently.

**This is structural, not a typo.** The design splits deliberately into
"a model extracts facts" and "a script finds collisions exactly,"
precisely so the detection half is trustworthy rather than guessed at.
But the script matches on *free-text strings the model invents*, once per
section, in independent calls with no knowledge of what the other
sections called the same thing. The determinism is real and the
vocabulary underneath it isn't. Those two halves are in tension, and the
seam is invisible from either side.

**Why this never showed up before.** Every prior test ran against
`sample_manuscript`, whose vocabulary was authored alongside the tool —
consistent naming came for free. Only genuinely fresh material, written
without the extractor in mind, exposes it. That is an argument about test
fixtures generally, not just this bug.

**The asymmetry that makes it dangerous:** adjudication reviews every
collision the script reports, so over-reporting gets caught — in the same
run, the adjudicator correctly dismissed two false positives on its own
(see below). Nothing whatsoever reviews what the script *failed* to
report. A missed contradiction produces no artifact, no log line, and no
finding. It looks exactly like a clean manuscript.

**Candidate fixes, in rough order of strength — none implemented, and
the first two are palliatives, not solutions:**

1. **Normalize before matching** (lowercase, strip leading articles).
   Fixes `tide clock`/`the tide clock`. Does nothing for
   `duration_not_working`/`stopped_duration`. Cheap; strictly an
   improvement; not sufficient alone.
2. **Constrain attribute vocabulary per domain**, the way
   `domain.json` already constrains `entity_types`, `sources`, and
   category lists. Narrows synonym drift but can't anticipate every
   attribute a real manuscript needs, and an over-tight list would
   silently drop facts that don't fit — trading a recall bug for a
   different recall bug.
3. **Give the extractor the canon so far**, so section N reuses the
   entity/attribute names sections 1..N-1 already established, and
   naming is consistent *by construction* rather than by coincidence.
   This is the real fix. It costs the per-section independence the
   current design gets for free (sections can currently be extracted in
   any order, and re-extracted individually after an edit), so it needs
   actual thought about ordering and about what happens on re-extraction
   — not a small change.
4. **Mark set-valued attributes as non-colliding** (`owns`, `contains`,
   `knows`). This addresses the *precision* half seen in the same run,
   not this recall bug. Worth doing, separately.

**Do not treat this as fixed by adding a test to `sample_manuscript`.**
The fixture would then be written to match whatever naming the fix
produces, which is the same blind spot that hid the bug. It needs a
fresh manuscript, or a fixture written by someone not looking at the
extractor's output.

## First full intake → assess pass on a fresh manuscript (DONE, 2026-08-12)

The item `README.md` had carried as "not yet done" since the beginning:
a complete `/redliner:intake` → `/redliner:run assess` run through the
real Task-orchestrated pipeline, on a manuscript with no relationship to
this repo. Run against a purpose-written three-section literary-fantasy
scratch manuscript (~750 words) with two issues deliberately seeded: one
crisp factual contradiction, and one ambiguous in-dialogue disagreement
that *should* require judgment rather than mechanical detection.

**What worked, and is now evidence rather than assumption:**

- Intake completed end to end from a single non-interactive prompt,
  wrote `state.json` and `brief.md`, marked the four fields it wasn't
  given as explicitly unspecified rather than inventing them or letting
  "off-limits" read as *nothing is off-limits*.
- Intake noticed the planted contradiction while reading and
  **deliberately kept it out of the brief**, reasoning that filing it
  under "known problem areas" would tell later passes not to report it,
  and would attribute it to the author as already-known. That is
  genuinely good judgment and was not prompted for.
- Assess produced 4 developmental findings (2 moderate, 2 minor) across
  `pacing`/`plot`/`structure`/`stakes` — specific to the text, not
  generic advice — and **zero line-level findings**, correctly gated by
  the `exploratory / partial` draft stage.
- The continuity layer built canon from 60 facts across 16 entities and
  found 3 collisions. Adjudication kept 1 (the dialogue disagreement,
  correctly left open as author-intent) and **dismissed 2 false
  positives on its own**, correctly diagnosing them as reconcile
  treating set-valued attributes (`owns`, `contains`) as single-valued.
- The excerpt-verbatim check held against text the agents chose
  themselves.
- `likely_unpropagated_revision` correctly reported as *not applicable*
  on a first-ever assess rather than as a clean result — the distinction
  the flag is supposed to make.

**What it found:** the recall bug above. Worth stating plainly that the
run's most valuable output was a defect in the tool, not the editorial
letter.

**Duration, measured: ~13.5 minutes for three sections / ~750 words.**
This scales with section count, so a full novel is **hours**, not
minutes. Nothing anywhere sets that expectation.

**Token cost of that same run, from the session transcripts.** Claude
Code writes per-message `usage` into `~/.claude/projects/<cwd>/<session>.jsonl`,
with one `subagents/agent-*.jsonl` per Task call — so a run can be costed
exactly, per step, after the fact. For this run: **113 API calls,
45,347 output tokens, 241,744 cache-write, 2,516,921 cache-read.**

> ⚠️ **Any dollar figure here is an estimate, not a bill.** The *token
> counts* are measured; the money is those tokens valued at published
> per-token API rates. **Most Claude Code users are on a Pro/Max
> subscription**, where usage draws against plan limits rather than
> being charged per token — for them these numbers are a *relative*
> measure of how expensive a pass is, not an amount anyone is billed.
> Rates also change over time, and this run's rates were for
> `claude-opus-5` specifically. Treat the ratios (which step dominates,
> what caching saves) as the durable finding and the totals as
> indicative.

At `claude-opus-5` rates ($5/M in, $25/M out, cache write 1.25×, cache
read 0.1×) the run values at **~$3.91**, split:

| step | calls | cost |
| --- | ---: | ---: |
| coordinator (main session) | 33 | $1.64 |
| developmental editor | 17 | $0.47 |
| continuity extractor ×3 | 31 | $1.03 |
| continuity adjudicator | 11 | $0.22 |
| editorial aggregator | 21 | $0.55 |

**Three findings that hold regardless of anyone's billing
arrangement:**

1. **Prompt caching is doing most of the work.** The identical run
   without caching values at **$14.93** — caching accounts for a **74%**
   reduction. Anything that invalidates the cached prefix (reordering
   the agent prompt, injecting a timestamp) would cost roughly 4× more,
   silently.
2. **The coordinator is the single largest line item at 42%** — more
   than the developmental editor and aggregator combined. That is
   *orchestration overhead*, not editorial work: 33 calls re-reading an
   accumulating session context. It is the obvious optimization target
   and it is invisible if you only think in terms of "the five agents."
3. **The per-section extractor cost (~$0.34/section) is the only part
   that scales cleanly.** The developmental read, aggregator, and
   coordinator all scale with *total* manuscript length, and the
   coordinator's context grows as the run proceeds — so it scales worse
   than linearly. A 40-chapter novel is ~$14 in extractors alone, plus
   whole-manuscript passes on top. **Do not extrapolate the total from
   this fixture** — 750 words is far too small to model context growth
   honestly; measure a real manuscript instead.

**Model choice is the biggest single lever, and redliner deliberately
does not pull it.** All fifteen agent files specify `model: inherit`, so
every pass runs on whatever model the user's session is using — this run
was `claude-opus-5` only because the harness driving it defaulted there.
A Sonnet-tier session runs the same pass for roughly half. Pinning a
cheaper model in the agent frontmatter would make cost predictable at
the price of overriding a choice the user has already made, and the
editorial quality of these passes is the whole product. **Left as
`inherit` on purpose; revisit only with evidence that a cheaper model
holds finding quality.**

**The UX problem this surfaced.** The author of this tool, watching his
own run, could not tell whether it had hung. Two things are true at once:

- Much of that was a **testing artifact** — the run was driven with
  `claude -p`, which has no TUI and buffers all output until exit.
- **Confirmed by an interactive re-run, and it works differently than
  first written here.** redliner's subagents run in the **foreground**,
  so they do *not* appear as rows in the background-agent panel. They
  render inline as a labelled tool-call tree — `redliner:fiction-
  developmental-editor(Developmental pass round 1)` with its `Read(...)`
  calls nested underneath — above the main spinner, which shows elapsed
  time and streaming token count (`1m 32s · ↓ 2.1k tokens`). That is
  more informative than the panel row this section originally predicted:
  you can see *which* subagent is running and *what it is doing*.
  Corrected rather than deleted, because the original claim was wrong in
  a way worth not repeating: `subagentStatusLine` and the agent panel
  govern **background** subagents, which is not how this tool runs.
- **What's genuinely missing is scale, not liveness.** The timer proves
  something is happening; it can't say how long the pass *should* take,
  or how far through it you are. At minute eight of thirteen, a ticking
  clock and "is it stuck?" look identical.

**Options, researched against Claude Code's docs, none implemented:**

- **`subagentStatusLine`** is plugin-shippable — notably one of the only
  two keys a plugin-root `settings.json` honours, the same fact behind
  this file's permission-allowlist gap. Shape:
  `{"subagentStatusLine": {"type": "command", "command": "..."}}`. The
  script receives a `tasks[]` array on stdin (each with `startTime`,
  `status`, `description`, `tokenCount`, `contextWindowSize`) and emits
  `{"id": ..., "content": ...}` per row. `startTime` is what gives
  elapsed time.
- **`SubagentStart`/`SubagentStop` hooks** can write progress state that
  the status line then renders, which is what "step 2 of 5, section 2 of
  3" would require.
- **`refreshInterval` is load-bearing, not optional.** The docs are
  explicit that event-driven status updates go quiet while a coordinator
  waits on background subagents — which is redliner's entire runtime. A
  first implementation without it would freeze the display at precisely
  the moment the user starts wondering whether it hung.
- Status lines require **workspace trust** and render nothing, silently,
  without it — so this can't be the only signal.

**Cheapest fix — DONE 2026-08-12, and it made the status line
unnecessary for now.** `skills/run/SKILL.md` gained a "Say what's about
to happen, before starting a long pass" section: before any subagent-
Tasking subcommand (`assess`/`recheck`/`line`/`continuity`, explicitly
not the fast ones), state the step list counted for *this* manuscript,
a rough duration computed from the real section count, and that silent
stretches are expected. Estimation guidance is **N + 3** model steps for
`assess` (N + 1 for `line`/`continuity`), budgeted at ~2–3 minutes each,
stated as a range and explicitly flagged as extrapolated from a single
measurement on short sections.

It also tells the coordinator to **report each step as it completes**
("developmental read done (1/6)"). That is the piece that actually
answers "how far along is this", and it needs no `subagentStatusLine`,
no hook, no `refreshInterval`, and no workspace trust — the coordinator
is already sitting between the Task calls, so it is just text.

**So the status-line work above is deferred, not planned.** Revisit only
if the prose proves insufficient in real use. Building it first would
have meant shipping the fragile mechanism (trust-gated, silently
degrading, stale without `refreshInterval`) to solve a problem that
plain text solves.

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
