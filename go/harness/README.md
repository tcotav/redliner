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

## The Go-side comparison runner (Phase 3+)

`go/internal/cli/golden_test.go`'s `TestCLI_MatchesPythonGolden` re-runs
every fixture's step sequence through `cli.Dispatch` **in-process**
(not by exec'ing a built `cmd/redliner` binary) and diffs the result
against this directory's `golden/`, using the same comparison rule as
`capture_baseline.comparable()`: `stdout_json` (timestamp-stripped) for
JSON-shaped output, exact `stdout` text otherwise. It also re-parses
every JSON file under the working copy's `.redliner/` after each step
(mirrors `capture_baseline.py`'s `snapshot_state_dir`) — this is what
actually verifies `canon reconcile`'s array ordering (per-attribute
fact lists, each collision's `facts[]`), not just its summary print.

**CLI-shape divergence, by name, not by exit code.** The Go CLI is
subcommands of one binary (`redliner state status <dir>`), not four
script names (`redliner_state.py status <dir>`) — see TODO.md's "v1
plan" for why. This changes exactly one piece of stdout text on purpose:
`requireState`'s "no state yet" message now says `redliner state init
<dir>`, tracked as the single entry in `golden_test.go`'s
`knownDivergentStdout`. Every other non-JSON step's stdout — including
failure-path messages like `"State already exists at ..."` and
`"Section file error: ..."` — is compared exactly, regardless of exit
code. If you add a step whose text legitimately needs to differ, name it
in `knownDivergentStdout` rather than exempting a whole category (an
earlier version of this test exempted all nonzero-exit steps and missed
that 5 of them matched Python byte-for-byte with no code change needed).

**This test only passes from the checkout path the golden data was
captured from.** The golden files embed `go/harness/.work/<fixture>`'s
*absolute* path verbatim (state.json's `manuscript_dir`, the
`"Initialized <path>"`/`"OK <path>"` messages) — real captured Python
behavior, not something worth normalizing away. The test reuses
`capture_baseline.py`'s own `.work/<fixture>` working directory (same
convention, gitignored) so the embedded paths match exactly instead of
chasing this with path-substitution logic — which means it's checkout-
path-bound. Running it from a worktree, a second clone, or CI at a
different path produces failures indistinguishable from real port bugs;
the test detects this itself and skips with a clear message instead of
failing silently wrong. **`capture_baseline.py` and this test both own
`go/harness/.work/<fixture>` and both delete it on run — don't run them
concurrently.**
