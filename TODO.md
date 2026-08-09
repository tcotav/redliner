# Open questions / deferred work

Design questions raised during development that we deliberately parked.
Not a task tracker — the reasoning matters more than the checkbox.

## Domain generalization: build the domain-creation skill (DONE)

**Raised:** 2026-08-09. **Completed:** 2026-08-09.

The user wants edaitor usable beyond fiction (product proposals, design
docs) if it can be done without compromising the fiction use case, which
stays the primary one. This was scoped as four steps; **all four are
done**:

1. ✅ Renamed `chapter_*.txt` → `section_*.txt` and all associated field
   names everywhere (mechanical, no domain concept yet).
2. ✅ Added a `domain` field to `.edaitor/state.json`, defaulting to
   `"fiction"`.
3. ✅ Moved fiction's category vocabulary (developmental/line categories,
   continuity entity types/sources/categories) into
   `domains/fiction/domain.json`, loaded per-manuscript by
   `bin/schemas/domain_loader.py`. `findings_schema.py`/`canon_schema.py`'s
   validators take categories as parameters now, not module constants.
   Also fixed a coupling bug this surfaced: `edaitor_state.py`'s
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
  would also mean renaming the `/edaitor:run assess`/`/edaitor:run line`
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
  now resolves `edaitor:<domain>-<role>` from the manuscript's `domain`
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

**New gap surfaced, deliberately not fixed here:**
`agents/*-continuity-extractor.md` and `*-continuity-adjudicator.md` are
never Tasked from any step in `run/SKILL.md`, for either domain — the
continuity layer's sample data in this repo was produced by hand or
direct script testing, not through the orchestrated pipeline. Wiring the
continuity layer into `/edaitor:run` (probably a new subcommand, or a
flag on `assess`/`recheck`) is real, scoped work and out of scope for
domain generalization specifically — it's a pre-existing gap this work
happened to notice, not something domain generalization caused.

## Port to a compiled language for distributable binaries?

**Raised:** 2026-08-08. **Updated:** 2026-08-08, after the plugin conversion.

The deterministic pieces are all Python (`bin/edaitor_state.py`,
`bin/edaitor_canon.py`, `bin/validate_findings.py`, `bin/schemas/`),
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
  edaitor never writes to it.
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
than one user — maybe a `/edaitor:setup` step that offers to write it for
them, rather than expecting a novelist to hand-edit `settings.json`.
