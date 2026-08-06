# edaitor

A layered fiction-editing agent, built as an intro project to Google's
[Agent Development Kit (ADK)](https://adk.dev/). The goal isn't a
production editing tool — it's a small, real system to learn agentic
design patterns against: multi-agent orchestration, structured output,
session state, and custom control flow.

## What it does

Runs a manuscript through two editing layers, then synthesizes the results
into one editorial letter:

1. **Developmental editor** — reads the *whole* manuscript at once, flags
   story-level issues (plot, pacing, character arcs, structure, stakes,
   theme).
2. **Line editor** — reads *one chapter at a time*, flags prose-level
   issues (rhythm, voice, show-vs-tell, dialogue, POV, word choice).
3. **Aggregator** — takes the structured findings from both layers (never
   the raw manuscript) and writes a human-readable editorial letter.

Every finding is tagged with a `category` and a `severity`
(minor/moderate/major/critical) — see `edaitor_agent/schemas.py`. That's
what makes the aggregation step synthesis-over-data instead of another
read-everything-and-summarize pass, and it's what a future evaluation
harness would grade against.

## Architecture

```
run_pipeline.py                     # loads chapters -> session state, drives the Runner
        │
        ▼
root_agent (SequentialAgent)        # edaitor_agent/agent.py
        │
        ├── developmental_editor    # LlmAgent, sees full_manuscript_text
        │     output_key: developmental_report
        │
        ├── ChapterLineEditLoop     # custom BaseAgent (not LoopAgent — see below)
        │     └── line_editor       # LlmAgent, sees one chapter at a time
        │           output_key: current_line_report (collected into line_reports)
        │
        └── editorial_letter_aggregator   # LlmAgent, sees only the structured
              output_key: editorial_letter #   reports above, not raw text
```

### Design decisions worth calling out

- **Different context per layer.** The developmental editor needs the
  whole story's shape; the line editor deliberately only sees one chapter.
  That's a real editing distinction (you can't judge pacing from one
  chapter, or prose rhythm from a synopsis), and it's also what forces
  actual use of session state to pass data between agents instead of
  cramming everything into one prompt.

- **Custom agent instead of `LoopAgent` for the chapter loop.** ADK's
  `LoopAgent` is built for "repeat until a stop condition" (iterative
  refinement). What we need is "run this agent once per item in a known
  list" — plain, deterministic control flow. Per ADK's own guidance, that
  belongs in a custom `BaseAgent` subclass, not bent into `LoopAgent`'s
  `exit_loop`/`max_iterations` mechanism. See
  `edaitor_agent/sub_agents/chapter_loop.py`.

- **No tools on the editor agents.** ADK only supports combining `tools`
  and `output_schema` on the same `LlmAgent` for specific models (Gemini
  3.0+ at time of writing). Structured output is non-negotiable here, so
  the editor agents stay tool-free. If a later layer needs a tool (e.g. a
  continuity/fact-tracking database for a future consistency-checking
  layer), the pattern is a tool-calling agent feeding a separate
  formatting agent — not one agent doing both.

- **A script, not `adk web`.** `adk web`/`adk run` are built around a
  conversational loop. edaitor acts on manuscript files already on disk
  with no back-and-forth needed, so `run_pipeline.py` wires the `Runner`
  directly and seeds session state from disk before the first agent runs.

## Setup

```bash
python -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
cp .env.example .env   # then fill in GOOGLE_API_KEY
```

## Run

```bash
python run_pipeline.py                  # uses sample_manuscript/
python run_pipeline.py path/to/chapters  # or point it at real chapters
```

Manuscript directories are plain `.txt` files, one per chapter, read in
sorted filename order (`chapter_01.txt`, `chapter_02.txt`, ...).

`sample_manuscript/` holds two short placeholder chapters with a few
deliberately seeded issues (an eye-color continuity slip, an info-dump
paragraph, repetitive phrasing) to give the pipeline something to find
before pointing it at real work.

## Status / next steps

This is a first working scaffold, not yet run end-to-end against a live
model — some ADK API details (`BaseAgent` field declarations, `Runner`/
session constructor signatures) were pieced together from current docs and
should be treated as provisional until we run it and see what breaks.
Likely next steps, roughly in order:

1. Run it, fix whatever the actual installed `google-adk` API disagrees
   with.
2. Look at real output against `sample_manuscript/` — does severity
   tagging actually track what a human editor would flag as severe?
3. Consider a continuity/consistency layer (needs persistent state across
   the whole manuscript, and is a natural place to finally exercise
   tool-calling within an agent, per the constraint above).
4. Point it at a real manuscript.
