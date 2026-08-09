# edaitor

A layered fiction-editing tool that runs as a Claude Code plugin —
developmental editing, line editing, and a cross-cutting continuity
checker, each running as its own subagent against your manuscript.

## What it does

Three layers, run separately rather than as one pass:

1. **Developmental editing** — reads the *whole* manuscript at once,
   flags story-level issues (plot, pacing, character arcs, structure,
   stakes, theme). Iterative: runs in rounds, tracks which findings
   you've addressed, and re-checks after revision.
2. **Line editing** — reads *one section at a time*, flags prose-level
   issues (rhythm, voice, show-vs-tell, dialogue, POV, word choice).
   Gated behind developmental work settling — see below.
3. **Continuity** — cross-cutting, runs alongside either phase. Extracts
   checkable facts per section, finds contradictions mechanically
   (same entity, same attribute, different value — computed exactly, not
   guessed at), then has an agent judge only the collisions found: real
   error vs. a lying character vs. an edit you made in one section that
   hasn't propagated to another yet.

Every finding carries an `id`, a `status` (open/claimed/addressed/stale/
wontfix), a `category`, and a `severity`.

An **intake interview** (`/edaitor:intake`) runs first and produces a
persistent brief — genre, draft stage, and a *deliberate choices* list,
because an editor that doesn't know your intent will report your choices
as your mistakes.

## Why phases are sequential

Developmental editing iterates until structure settles; line editing
comes after. A combined pass produces contradictory advice about the
same paragraph — recommending it be deleted while also giving it
sentence-level rewrites. That happened in an early version of this tool,
which is why the phases are now enforced separately rather than left as
a suggestion.

## Setup

Nothing to install beyond Claude Code itself and Python 3 (stdlib only —
no `pip install`). Load the plugin from whatever directory holds the
manuscript you want to work on:

```
claude --plugin-dir /path/to/edaitor
```

(A marketplace install, once this is distributed somewhere, would replace
the `--plugin-dir` flag with a one-time `/plugin install`.)

## Run

```
/edaitor:intake                 # first time only, or to revise the brief
/edaitor:run assess             # developmental pass
/edaitor:run work dev-003       # talk through one finding
/edaitor:run resolve dev-003    # mark it addressed (your claim)
/edaitor:run recheck            # verify claims after revision
/edaitor:run line               # line-editing phase (soft-gated on open major/critical findings)
/edaitor:run status             # where things stand
```

Manuscript directories are plain `.txt` files, one per section, read in
sorted filename order (`section_01.txt`, `section_02.txt`, ...). Defaults
to the current directory if no path is given.

Validate a manuscript's `.edaitor/` output directly, without running a
full pass:

```
bin/validate_findings.py <manuscript_dir>
```

## Architecture

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
    ├── edaitor_state.py          phase, rounds, section fingerprinting/diff
    ├── edaitor_canon.py          merges per-section facts, finds collisions mechanically
    ├── validate_findings.py      schema + excerpt-verbatim checks
    └── schemas/                  shared vocabulary + validators, imported by all three
```

State lives with the manuscript, not with the tool: every manuscript
directory gets its own `<manuscript_dir>/.edaitor/` (state, brief,
findings, canon), so edaitor works across multiple manuscripts and a
manuscript's editing history travels with it if you move or back it up.

A few things worth knowing if you're debugging or extending this:

- **Findings are files, not just chat output.** Each subagent writes a
  JSON file with the `Write` tool rather than putting its findings only
  in its final reply — durable, inspectable, and checkable by
  `validate_findings.py` between steps.
- **Nothing here enforces JSON shape at the API level.** A subagent is
  *told* the schema and could still get it wrong, so
  `bin/validate_findings.py` checks after the fact — including verifying
  that any `excerpt` field is a genuine verbatim substring of the section
  it claims to quote, not a paraphrase. That check has already caught a
  real fabricated excerpt in this repo's own sample data.
- **Deterministic detection, model judgment — kept as two separate
  steps.** Section-hash diffing and continuity-collision finding are both
  exactly computable, so they're plain scripts. Judgment (is this really
  a contradiction?) only happens after, on what the script already found.
- **Developmental passes run unattended.** Subagents have no tool to ask
  you anything mid-pass. Ambiguity your brief doesn't resolve gets
  picked, proceeded on, and recorded in an `assumptions` list instead of
  guessed at silently — this has already surfaced a real
  self-contradiction in this repo's own sample brief.
- **Subagents must be referenced by their plugin-namespaced name**
  (`edaitor:developmental-editor`, not `developmental-editor`) anywhere
  `SKILL.md` invokes the Task tool. A bare name fails outright — caught
  once already by an actual plugin load test, not by static checking.

### Known gap: the permission allowlist doesn't travel with the plugin

Claude Code plugin-root `settings.json` only honors the `agent` and
`subagentStatusLine` keys — not permission rules. So the allowlist that
stops `bin/`'s scripts from triggering a permission prompt on every call
can't ship inside the plugin. Add this to your own project or user
`settings.json` if you want that:

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

## Status

Working end to end against `sample_manuscript/` (a bundled two-section
test fixture with deliberately seeded issues — not a model for where
your own manuscript should live) through all three layers. The plugin
structure itself has been verified with a real `claude --plugin-dir`
load against a scratch manuscript with no relationship to this repo, not
just statically — that test caught and fixed the bare-vs-namespaced
subagent bug mentioned above.

Not yet done: a full `/edaitor:intake` → `/edaitor:run assess` pass
through the real Task-orchestrated pipeline, start to finish, on a fresh
manuscript. See `TODO.md` for other open items (a compiled-binary port,
read-only Obsidian vault integration as a second source of canon).
