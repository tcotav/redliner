#!/usr/bin/env python3
"""Capture golden baselines from the current Python implementation, for
the Go port's differential harness (TODO.md, "Port to a compiled
language" -> "Differential harness").

Phase 1 has no Go implementation yet -- this script's only job right now
is to (a) produce the golden outputs Phase 3+ will diff the Go binary
against, and (b) prove the harness itself is honest by running twice and
confirming the normalized captures are identical (`--self-check`).

Each fixture under `fixtures/` gets an ordered sequence of CLI
invocations run against one working copy of that fixture (state built up
step by step, same as a real editing session), rather than one operation
each in isolation -- `state diff` after `state snapshot` only means
anything in sequence. Every step's stdout, exit code, and the resulting
`.redliner/` tree (JSON-parsed, timestamps stripped per `normalize.py`)
is captured to `golden/<fixture>/<NN>_<op>.json`.

This deliberately shells out to the real `bin/redliner_*.py` /
`bin/validate_findings.py` scripts as subprocesses -- black-box, the same
interface a Go binary has to match -- rather than importing their
functions in-process. See this directory's README for why MCP-front-door
parity (`cowork/mcp_server.py`) is *not* captured here: it needs the
`mcp` package, which this project deliberately doesn't install outside
the plugin's own bootstrapped venv, and its 10 tools are documented
thin wrappers over the exact same schemas-level calls the CLI already
exercises -- see README for the actual verification plan for that front
door (deferred to the Phase 5 live-install gate, not faked here).
"""

from __future__ import annotations

import json
import shutil
import subprocess
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from normalize import strip_timestamps

HARNESS_DIR = Path(__file__).resolve().parent
REPO_ROOT = HARNESS_DIR.parent.parent
BIN = REPO_ROOT / "bin"
FIXTURES_DIR = HARNESS_DIR / "fixtures"
GOLDEN_DIR = HARNESS_DIR / "golden"
WORK_DIR = HARNESS_DIR / ".work"

PY = sys.executable


def run(cmd: list) -> dict:
    proc = subprocess.run(cmd, capture_output=True, text=True, cwd=REPO_ROOT)
    stdout = proc.stdout
    stdout_json = None
    stripped = stdout.strip()
    if stripped.startswith("{") or stripped.startswith("["):
        try:
            stdout_json = strip_timestamps(json.loads(stripped))
        except json.JSONDecodeError:
            stdout_json = None
    return {
        "cmd": [str(c) for c in cmd],
        "exit_code": proc.returncode,
        "stdout": stdout,
        "stdout_json": stdout_json,
        "stderr": proc.stderr,
    }


def snapshot_state_dir(manuscript_dir: Path) -> dict:
    """Read back every JSON file under .redliner/, normalized. This is
    what actually verifies side effects (canon_reconcile's file writes,
    state_snapshot's fingerprint updates) -- stdout alone doesn't."""
    redliner_dir = manuscript_dir / ".redliner"
    if not redliner_dir.is_dir():
        return {}
    out = {}
    for path in sorted(redliner_dir.rglob("*.json")):
        rel = str(path.relative_to(redliner_dir))
        try:
            out[rel] = strip_timestamps(json.loads(path.read_text(encoding="utf-8")))
        except json.JSONDecodeError:
            out[rel] = {"__unparseable__": path.read_text(encoding="utf-8")}
    return out


def capture_step(manuscript_dir: Path, cmd: list) -> dict:
    result = run(cmd)
    result["state_dir_snapshot"] = snapshot_state_dir(manuscript_dir)
    return result


# Each entry: (fixture_name, [(step_label, cmd_builder), ...])
# cmd_builder takes the working manuscript dir and returns an argv list.
FIXTURE_SCRIPTS = {
    "happy": [
        ("01_domain_list", lambda d: [PY, BIN / "redliner_domain.py", "list"]),
        ("02_domain_show_fiction", lambda d: [PY, BIN / "redliner_domain.py", "show", "fiction"]),
        ("03_state_status", lambda d: [PY, BIN / "redliner_state.py", "status", d]),
        ("04_state_diff", lambda d: [PY, BIN / "redliner_state.py", "diff", d]),
        ("05_canon_stale", lambda d: [PY, BIN / "redliner_canon.py", "stale", d]),
        ("06_canon_reconcile", lambda d: [PY, BIN / "redliner_canon.py", "reconcile", d]),
        ("07_validate_findings", lambda d: [PY, BIN / "validate_findings.py", d]),
        ("08_state_init_again", lambda d: [PY, BIN / "redliner_state.py", "init", d]),  # expect failure: already exists
        ("09_state_phase_intake", lambda d: [PY, BIN / "redliner_state.py", "phase", d, "intake"]),
        ("10_state_phase_developmental", lambda d: [PY, BIN / "redliner_state.py", "phase", d, "developmental"]),  # round increment
        ("11_state_snapshot", lambda d: [PY, BIN / "redliner_state.py", "snapshot", d]),
    ],
    "crlf": [
        ("01_state_init", lambda d: [PY, BIN / "redliner_state.py", "init", d]),
        ("02_state_status", lambda d: [PY, BIN / "redliner_state.py", "status", d]),
        ("03_state_snapshot", lambda d: [PY, BIN / "redliner_state.py", "snapshot", d]),  # CRLF hashing happens here
        ("04_state_status_after", lambda d: [PY, BIN / "redliner_state.py", "status", d]),
        ("05_state_diff_unchanged", lambda d: [PY, BIN / "redliner_state.py", "diff", d]),
    ],
    "collision": [
        ("01_state_init", lambda d: [PY, BIN / "redliner_state.py", "init", d]),
        ("02_state_diff_collision", lambda d: [PY, BIN / "redliner_state.py", "diff", d]),  # expect SectionCollisionError
        ("03_state_snapshot_collision", lambda d: [PY, BIN / "redliner_state.py", "snapshot", d]),
        ("04_canon_stale_collision", lambda d: [PY, BIN / "redliner_canon.py", "stale", d]),
    ],
    "empty": [
        ("01_state_status_no_state", lambda d: [PY, BIN / "redliner_state.py", "status", d]),
        ("02_domain_list", lambda d: [PY, BIN / "redliner_domain.py", "list"]),
        ("03_validate_findings_no_redliner_dir", lambda d: [PY, BIN / "validate_findings.py", d]),
    ],
}


def fresh_work_copy(fixture: str) -> Path:
    src = FIXTURES_DIR / fixture
    dst = WORK_DIR / fixture
    if dst.exists():
        shutil.rmtree(dst)
    shutil.copytree(src, dst)
    return dst


def capture_all(golden_dir: Path) -> None:
    for fixture, steps in FIXTURE_SCRIPTS.items():
        work = fresh_work_copy(fixture)
        out_dir = golden_dir / fixture
        out_dir.mkdir(parents=True, exist_ok=True)
        for label, cmd_builder in steps:
            cmd = cmd_builder(work)
            result = capture_step(work, cmd)
            (out_dir / f"{label}.json").write_text(
                json.dumps(result, indent=2, default=str) + "\n", encoding="utf-8"
            )
        print(f"{fixture}: {len(steps)} steps captured -> {out_dir}")


def load_captured(golden_dir: Path) -> dict:
    captured = {}
    for path in sorted(golden_dir.rglob("*.json")):
        captured[str(path.relative_to(golden_dir))] = json.loads(path.read_text())
    return captured


def comparable(entry: dict) -> dict:
    """The view actually used for comparison, matching the harness's own
    stated rule: JSON-shaped stdout is compared via `stdout_json` (already
    timestamp-stripped); the raw `stdout` string for those commands still
    contains real timestamps and isn't a meaningful comparison target.
    Only non-JSON ("human-facing print") output is compared as exact
    stdout text."""
    if entry.get("stdout_json") is not None:
        return {k: v for k, v in entry.items() if k != "stdout"}
    return entry


def self_check() -> int:
    """Run capture twice into separate dirs and diff -- proves timestamp
    stripping actually makes two honest runs compare equal, rather than
    the harness silently baking in false diffs from run to run."""
    run_a = HARNESS_DIR / ".selfcheck-a"
    run_b = HARNESS_DIR / ".selfcheck-b"
    for d in (run_a, run_b):
        if d.exists():
            shutil.rmtree(d)
    print("--- self-check pass 1 ---")
    capture_all(run_a)
    print("--- self-check pass 2 ---")
    capture_all(run_b)

    a = load_captured(run_a)
    b = load_captured(run_b)
    if a.keys() != b.keys():
        print("MISMATCH: different file sets between runs")
        return 1

    ok = True
    for key in sorted(a):
        if comparable(a[key]) != comparable(b[key]):
            print(f"MISMATCH: {key} differs between two honest runs")
            ok = False

    if ok:
        shutil.rmtree(run_a)
        shutil.rmtree(run_b)
        print(f"Self-check OK: {len(a)} captured files identical across two independent runs.")
        return 0
    return 1


def main(argv: list) -> int:
    if "--self-check" in argv:
        return self_check()
    GOLDEN_DIR.mkdir(parents=True, exist_ok=True)
    capture_all(GOLDEN_DIR)
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
