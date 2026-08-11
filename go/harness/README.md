# Differential harness

Proves the Go port matches the current Python implementation, operation
by operation, rather than trusting a read-through of the ported code.
Built in Phase 1, *before* any Go logic exists, so there's something
honest to diff against from the start (see TODO.md's "Differential
harness" paragraph for why this is sequenced first).

## Layout

- `fixtures/` — input manuscripts, checked in, never mutated by a run:
  - `happy/` — a copy of `../../sample_manuscript`: full pipeline, real
    `.redliner/` state already in place (developmental phase, round 1,
    two real continuity collisions already reconciled) — exercises
    every read path plus the double-`init`-fails case.
  - `crlf/` — one CRLF-line-ended section, no state yet. Exercises the
    hashing hazard called out in TODO.md: Python's `Path.read_text()`
    does universal-newline translation and Go's `os.ReadFile` doesn't;
    the Go port has to normalize explicitly to produce the same hash.
  - `collision/` — `section_01.txt` *and* `section_01.md`, same stem.
    Triggers `SectionCollisionError`.
  - `empty/` — no sections, no `.redliner/` at all. Exercises the "no
    state yet" and "not a manuscript dir" error paths.
- `capture_baseline.py` — runs each fixture's operation sequence against
  a fresh working copy (`.work/`, gitignored) via the real `bin/*.py`
  subprocesses, and records stdout/exit-code/resulting-`.redliner/`-tree
  per step to `golden/<fixture>/<NN>_<op>.json`.
- `normalize.py` — strips `created_at`/`updated_at` before comparison;
  see its docstring for why (timestamps are expected to differ, both
  run-to-run and Python-vs-Go).
- `golden/` — captured baseline output. Checked in once captured, so
  Phase 3+ has something to diff the Go binary against without needing
  Python installed to regenerate it every time (only needed again if the
  Python implementation itself changes).

## Usage

Regenerate the golden baselines (only needed if `bin/*.py` changes):

```
python3 capture_baseline.py
```

Prove the harness itself isn't lying before trusting it for anything —
runs capture twice and diffs, using the same comparison rule real
Go-vs-Python diffing will use (`stdout_json`, timestamp-stripped, for
JSON output; exact `stdout` text only for non-JSON human-facing prints —
see `capture_baseline.comparable()`):

```
python3 capture_baseline.py --self-check
```

## What this does *not* cover yet

**MCP-front-door parity is not captured here.** `cowork/mcp_server.py`'s
10 tools are documented thin wrappers over the same `schemas`-level calls
the CLI baseline already exercises (`state_status` literally calls
`load_state`/returns it as a dict instead of printing it; `canon_reconcile`
and `validate_findings` capture the *same* CLI functions' stdout/file
writes this harness already captures). Capturing real MCP-protocol
baselines would require installing the `mcp` package outside the
plugin's own bootstrapped venv, which this project deliberately avoids
(see TODO.md's Cowork section on why dependency bootstrapping was worth
removing, not adding to). Real MCP-front-door verification — tool names,
descriptions, and behavior — happens at the Phase 5 gate: full
marketplace uninstall/reinstall + live Cowork query, the same protocol
that already caught two real bugs in this project and is not something a
mocked-up local capture would have caught either time.

## Adding a comparison runner for the Go binary (Phase 3+)

Not built yet. When it exists, it re-runs each fixture's step sequence
against `../cmd/redliner` binary output instead of `bin/*.py`, and diffs
against `golden/` using `capture_baseline.comparable()`'s same rule.
