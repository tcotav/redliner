# edaitor

A layered fiction-editing agent, built as an intro project to agentic
system design. Not a production editing tool — a small, real system to
learn multi-agent design patterns against: orchestration, structured
output under uncertainty, subagent specialization, phased/stateful
workflows, and validation.

## Why this version exists

The original plan was Google's Agent Development Kit (ADK) — a
declarative multi-agent-graph framework (`SequentialAgent`, `LoopAgent`,
API-enforced `output_schema`). That got scaffolded first (see early git
history) and is still a reference for that style of framework.

It got scrapped for a concrete reason, not a preference: ADK bills per
API call, and the only Claude access available for this project is a
Pro/Max subscription — no separate `ANTHROPIC_API_KEY`. Anthropic's own
Agent SDK has the same requirement (verified directly against their
quickstart docs: subscription login is explicitly not permitted for
SDK-built agents). So this version runs entirely on Claude Code —
subagents, skills, and a small deterministic-script layer, billed as
ordinary subscription usage, the same way this conversation itself runs.

That's a different substrate from ADK, not just a different vendor: no
declarative workflow-agent class hierarchy, no API-enforced schema, no
session-state object. Orchestration is written out as procedure (a
skill's instructions), agents are markdown + a system prompt, and
structure is enforced by validation after the fact rather than by the
API — see `bin/schemas/findings_schema.py` and `bin/schemas/canon_schema.py`.

## What it does

Three layers, run separately rather than as one pass:

1. **Developmental editing** — reads the *whole* manuscript at once,
   flags story-level issues (plot, pacing, character arcs, structure,
   stakes, theme). Iterative: runs in rounds, tracks which findings the
   author has addressed, and re-checks after revision.
2. **Line editing** — reads *one chapter at a time*, flags prose-level
   issues (rhythm, voice, show-vs-tell, dialogue, POV, word choice).
   Gated behind developmental work settling — see "Why phases are
   sequential" below.
3. **Continuity** — cross-cutting, runs alongside either phase rather
   than waiting its turn. Extracts checkable facts per chapter
   (judgment-free by schema — no severity/note field an opinion could
   live in), finds contradictions mechanically (same-entity-same-
   attribute-different-value, computed by a script, not asked of a
   model), then has an agent adjudicate only the collisions found —
   real error vs. lying character vs. unpropagated revision.

Every finding carries an `id`, a `status` (open/claimed/addressed/stale/
wontfix), a `category`, and a `severity` — non-negotiable design
decisions from the start, not things to loosen later.

An **intake interview** (`/edaitor:intake`) runs first and produces a
persistent brief — genre, draft stage, and critically a *deliberate
choices* list, because an editor that doesn't know your intent reports
your choices as your mistakes.

## Why phases are sequential

Developmental editing iterates until structure settles; line editing
comes after. A combined pass can produce contradictory advice about the
same paragraph — recommending it be deleted while also giving it
sentence-level rewrites. Confirmed empirically in an early run, not just
a theoretical concern (see git history around the phase-split commit).

## Architecture

edaitor is a **Claude Code plugin** — `.claude-plugin/plugin.json` at the
repo root, `agents/`, `skills/`, and `bin/` alongside it. That's a
deliberate fix, not the original design: an earlier version assumed the
manuscript lived inside this repo and scripts were invoked by relative
path from the repo root, which silently broke the moment a manuscript
lived anywhere else. As a plugin, `bin/` is added to the Bash tool's PATH
while the plugin is enabled, so the same three scripts work as bare
commands from *any* working directory — the intended usage is installing
the plugin once, then `cd`-ing into whatever directory holds the actual
manuscript.

```
edaitor/                          (plugin root)
├── .claude-plugin/plugin.json
├── agents/                       five subagents (Task tool targets)
│   ├── developmental-editor.md
│   ├── line-editor.md
│   ├── editorial-aggregator.md
│   ├── continuity-extractor.md
│   └── continuity-adjudicator.md
├── skills/
│   ├── run/SKILL.md              /edaitor:run <status|assess|work|resolve|recheck|line>
│   └── intake/SKILL.md           /edaitor:intake
└── bin/                          on PATH while the plugin is enabled
    ├── edaitor_state.py          phase, rounds, chapter fingerprinting/diff
    ├── edaitor_canon.py          merges per-chapter facts, finds collisions mechanically
    ├── validate_findings.py      schema + excerpt-verbatim checks
    └── schemas/                  shared vocabulary + validators, imported by all three
```

State lives with the manuscript, not with the tool: every manuscript
directory gets its own `<manuscript_dir>/.edaitor/` (state, brief,
findings, canon) — so edaitor stays reusable across manuscripts, and a
manuscript's editing history travels with it.

### Design decisions worth calling out

- **Findings are files, not just chat output.** Each subagent's
  deliverable is a JSON file written with the `Write` tool, not text in
  its final reply — durable, inspectable, and script-checkable between
  steps.

- **Prompt-enforced structure, checked after the fact.** ADK's
  `output_schema` guarantees shape at the API layer; there's no
  equivalent here, so `bin/validate_findings.py` (pure stdlib) is how
  this version buys back some of that guarantee — including verifying
  that any `excerpt` field is a genuine verbatim substring of the chapter
  it claims to quote, not a paraphrase. That check has already caught a
  real fabricated excerpt in this repo's own sample data, not just
  synthetic test cases.

- **Deterministic detection, model adjudication — kept as two steps.**
  Chapter-hash diffing (`edaitor_state.py diff`) and continuity-collision
  finding (`edaitor_canon.py reconcile`) are both computable exactly and
  identically every run, so they're scripts. Judgment — is this a real
  contradiction, a lying character, or an unpropagated edit — only
  happens after, and only on what the script already found. Don't move
  either direction across that line.

- **Developmental passes run unattended, on purpose.** Subagents have no
  tool to ask the author anything mid-pass, and interrupting a
  whole-manuscript read wouldn't improve it anyway. Ambiguity the brief
  doesn't resolve gets picked, proceeded on, and recorded in an
  `assumptions` list instead of guessed at silently — this has already
  surfaced a real self-contradiction in this repo's own sample brief.

### A known gap in the plugin conversion

Claude Code plugins can ship a root-level `settings.json`, but it only
honors the `agent` and `subagentStatusLine` keys — **not permission
rules**. So the scoped allowlist that stops `bin/`'s scripts from
triggering raw permission prompts (`.claude/settings.json` in this repo,
used for our own dev sessions) doesn't travel with the plugin to an
installer. Anyone installing edaitor who wants that same quiet behavior
needs to add the equivalent to their own project or user settings:

```json
{
  "permissions": {
    "allow": [
      "Bash(edaitor_state.py *)",
      "Bash(edaitor_canon.py *)",
      "Bash(validate_findings.py *)"
    ]
  }
}
```

## Setup

Nothing to install beyond Claude Code itself and Python 3 (stdlib only,
no `pip install`). Load the plugin (`--plugin-dir` while developing; a
marketplace install once this is distributed) from whatever directory
holds the manuscript you want to work on.

## Run

```
/edaitor:intake                 # first time only, or to revise the brief
/edaitor:run assess             # developmental pass
/edaitor:run work dev-003       # talk through one finding
/edaitor:run resolve dev-003    # mark it addressed (author's claim)
/edaitor:run recheck            # verify claims after revision
/edaitor:run line               # line-editing phase (soft-gated on open major/critical findings)
```

Manuscript directories are plain `.txt` files, one per chapter, read in
sorted filename order (`chapter_01.txt`, `chapter_02.txt`, ...). Defaults
to the current directory if no path is given.

`sample_manuscript/` is a bundled **test fixture**, not a model for where
a real manuscript should live — it holds two short placeholder chapters
with deliberately seeded issues (an eye-color continuity slip, a
POV-inconsistent info-dump, repetitive "telling" prose) plus a worked
example `.edaitor/brief.md`, so the pipeline has something real to run
against without pointing it at actual work first.

Validate any manuscript's `.edaitor/` output directly:

```
bin/validate_findings.py <manuscript_dir>
```

## Status / next steps

Working end to end against `sample_manuscript/` through all three layers
(developmental, line, continuity) as of the plugin conversion — but the
plugin structure itself has only been verified statically (file layout,
script executability, PATH-independent imports tested by invoking from
an unrelated directory). It has **not** been loaded through a real
`claude --plugin-dir` session yet — that's the next real step, since
static checks can't confirm skill/agent discovery or the Task tool
resolving namespaced subagents correctly.

Roughly in order after that:

1. Load via `claude --plugin-dir .` from an unrelated project directory
   and run the full `/edaitor:intake` → `/edaitor:run assess` flow for
   real, confirming PATH resolution and skill discovery work as designed
   rather than as tested.
2. Point it at a real manuscript.
3. Revisit `TODO.md` — Go port (deferred until schema churn settles) and
   Obsidian vault integration (read-only, second source of canon).
