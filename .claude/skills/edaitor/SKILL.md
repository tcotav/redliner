---
name: edaitor
description: Runs a manuscript through developmental and line editing, then produces an editorial letter. Use when the user asks to edit, review, or run edaitor on a manuscript directory, or types /edaitor.
---

# edaitor pipeline

Orchestrates three subagents — `developmental-editor`, `line-editor` (run
once per chapter), `editorial-aggregator` — over a manuscript directory.
Same three-layer architecture edaitor started with (see README.md), just
running on Claude Code subagents instead of a separate agent framework.

## Steps

1. **Resolve the manuscript directory.** Use the path given as an
   argument to this skill, or default to `sample_manuscript/`. List its
   `chapter_*.txt` files in sorted order — that sort order is the chapter
   order for everything downstream.

2. **Prepare a clean findings directory.** Create `findings/` at the repo
   root if it doesn't exist. If it already has files in it from a
   previous run, remove them first — step 5's validation pass needs to
   check *this* run's output, not stale files left over from last time.

3. **Run the developmental editor.** Invoke the `developmental-editor`
   subagent via the Task tool. Tell it the manuscript directory and the
   exact output path: `findings/developmental.json`.

4. **Run the line editor once per chapter.** For each chapter file,
   invoke the `line-editor` subagent via the Task tool with that one
   chapter's file path and an exact output path:
   `findings/line_<chapter_stem>.json` (e.g. `line_chapter_01.json`). Run
   these one at a time rather than firing them all in parallel — there's
   no shared-state conflict either way, but sequential keeps the
   transcript easy to follow while you're still learning the pattern. If
   you'd rather practice ADK-style parallel fan-out, this is the step to
   launch them concurrently in one message instead — see the README note
   on this tradeoff.

5. **Validate before aggregating.** Run
   `python3 validate_findings.py findings/` yourself (not via a
   subagent — this is deterministic code, not something that needs a
   model). If anything fails, stop and report the validation errors
   instead of feeding bad data into the aggregator.

6. **Run the aggregator.** Invoke `editorial-aggregator` via the Task
   tool, pointing it at the `findings/` directory and both output paths
   (`findings/editorial_letter.json`, `findings/editorial_letter.md`).

7. **Validate the letter too** (same script, same directory), then show
   the user `findings/editorial_letter.md` directly — read it and print
   it, don't just say "done."

## Arguments

An optional manuscript directory path, e.g. `/edaitor path/to/real/chapters`.
Defaults to `sample_manuscript/`.
