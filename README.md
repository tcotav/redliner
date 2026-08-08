# edaitor

A layered fiction-editing agent, built as an intro project to agentic
system design. Not a production editing tool — a small, real system to
learn multi-agent design patterns against: orchestration, structured
output under uncertainty, subagent specialization, and validation.

## Why this version exists

The original plan was to build this on Google's Agent Development Kit
(ADK), which is a declarative multi-agent-graph framework: `SequentialAgent`,
`LoopAgent`, typed sub-agent composition, API-enforced `output_schema`.
That version got scaffolded first (see git history) and is still a good
reference for that style of framework.

It got scrapped for a concrete reason, not a preference: ADK bills per
API call, and the only Claude access available for this project is a
Pro/Max subscription — no separate `ANTHROPIC_API_KEY`. Anthropic's own
Agent SDK has the same requirement (verified against their quickstart
docs directly: subscription login is explicitly not permitted for
SDK-built agents). So this version runs entirely on Claude Code —
subagents plus a skill, invoked interactively, billed as ordinary
subscription usage, the same way this conversation itself runs.

That's a different substrate from ADK, not just a different vendor:
there's no declarative workflow-agent class hierarchy here, no
API-enforced schema, no session-state object. Orchestration is written
out as procedure (a skill's instructions), agents are markdown + a
system prompt, and structure is enforced by validation after the fact
rather than by the API. Where that tradeoff shows up is called out below
and in `schemas/findings_schema.py`.

## What it does

Runs a manuscript through two editing layers, then synthesizes the
results into one editorial letter:

1. **Developmental editor** — reads the *whole* manuscript at once, flags
   story-level issues (plot, pacing, character arcs, structure, stakes,
   theme).
2. **Line editor** — reads *one chapter at a time*, flags prose-level
   issues (rhythm, voice, show-vs-tell, dialogue, POV, word choice).
3. **Editorial aggregator** — takes the saved findings from both layers
   (never the raw manuscript) and writes a human-readable editorial
   letter.

Every finding is tagged with a `category` and a `severity`
(minor/moderate/major/critical) — that was a non-negotiable design
decision from the start, not something to loosen later.

## Architecture

```
/edaitor [manuscript_dir]           # .claude/skills/edaitor/SKILL.md — the orchestrator
        │
        ├── Task: developmental-editor        # .claude/agents/developmental-editor.md
        │     reads all chapters, writes findings/developmental.json
        │
        ├── Task: line-editor  (once per chapter)   # .claude/agents/line-editor.md
        │     reads one chapter, writes findings/line_<chapter>.json
        │
        ├── validate_findings.py findings/    # plain script, not an agent
        │
        └── Task: editorial-aggregator        # .claude/agents/editorial-aggregator.md
              reads findings/*.json, writes findings/editorial_letter.{json,md}
```

### Design decisions worth calling out

- **Different context per layer, same as before.** The developmental
  editor needs the whole story's shape; the line editor deliberately only
  sees one chapter. This is the one piece of the design that's identical
  to the ADK version — it's an editing-practice distinction, not a
  framework artifact.

- **Findings are files, not just chat output.** Each subagent's
  deliverable is a JSON file it writes with the `Write` tool, not text in
  its final reply. That's what lets a plain Python script (not another
  agent) validate the output between steps, and it's what makes a
  `findings/` directory a durable, inspectable record of a run instead of
  something that only exists in a transcript.

- **Prompt-enforced structure, checked after the fact.** ADK's
  `output_schema` gets the model's structured-output machinery to
  guarantee shape at the API layer. There's no equivalent here — a
  subagent is *told* the JSON shape and could still get it wrong.
  `validate_findings.py` (pure stdlib, no install needed) is how this
  version buys back some of that guarantee: the skill runs it between the
  editing steps and the aggregator, and stops rather than aggregating
  bad data. Worth noticing empirically once this runs a few times: how
  often validation actually catches something.

- **Sequential by default, parallel as a deliberate variant.** The line
  editor runs once per chapter, one at a time, so a first run is easy to
  follow in the transcript. The skill notes where to fan them out
  concurrently instead if you want to practice that pattern — chapters
  don't share state, so nothing stops it.

## Setup

Nothing to install. This runs inside Claude Code, using whatever
subscription is already authenticated. `validate_findings.py` uses only
the Python standard library.

## Run

From this directory, in Claude Code:

```
/edaitor
```

or, pointed at a real manuscript:

```
/edaitor path/to/real/chapters
```

Manuscript directories are plain `.txt` files, one per chapter, read in
sorted filename order (`chapter_01.txt`, `chapter_02.txt`, ...).

`sample_manuscript/` holds two short placeholder chapters with a few
deliberately seeded issues (an eye-color continuity slip, a
POV-inconsistent info-dump, repetitive "telling" prose) to give the
pipeline something real to find before pointing it at actual work.

You can also run the validator standalone against any findings directory:

```
python3 validate_findings.py findings/
```

## Status / next steps

Scaffolded, not yet run end to end — next actual step is running `/edaitor`
against `sample_manuscript/` and seeing what breaks or reads wrong.
Likely next steps after that, roughly in order:

1. Run it, see whether the seeded issues in `sample_manuscript/` actually
   get caught, and at what severity.
2. Check how often `validate_findings.py` actually catches a malformed
   subagent output — that's the real signal on whether prompt-enforced
   structure was good enough here.
3. Consider a continuity/consistency layer — needs state that persists
   across the whole manuscript (a running character/fact list), which
   this architecture doesn't have yet.
4. Point it at a real manuscript.
