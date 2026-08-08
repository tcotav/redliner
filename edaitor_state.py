"""CLI for reading and updating a manuscript's edaitor state.

The skill shells out to this instead of reasoning about state itself.
Phase transitions, chapter hashing, and change detection are deterministic
work — putting them in a script means they behave the same every run and
can't be talked out of by a persuasive-sounding prompt.

Usage:
    python3 edaitor_state.py status   <manuscript_dir>
    python3 edaitor_state.py init     <manuscript_dir>
    python3 edaitor_state.py diff     <manuscript_dir>
    python3 edaitor_state.py snapshot <manuscript_dir>            # record current text as assessed
    python3 edaitor_state.py phase    <manuscript_dir> <phase>
"""

import json
import sys
from pathlib import Path

from schemas.project_state import (
    PHASES,
    diff_manuscript,
    fingerprint_manuscript,
    load_state,
    new_state,
    save_state,
    state_path,
)


def _require_state(manuscript_dir: Path) -> dict:
    state = load_state(manuscript_dir)
    if state is None:
        print(
            f"No edaitor state in {manuscript_dir}. Run: edaitor_state.py init {manuscript_dir}"
        )
        raise SystemExit(1)
    return state


def cmd_init(manuscript_dir: Path) -> int:
    if load_state(manuscript_dir) is not None:
        print(f"State already exists at {state_path(manuscript_dir)}")
        return 1
    state = new_state(manuscript_dir)
    path = save_state(manuscript_dir, state)
    print(f"Initialized {path} (phase: intake)")
    return 0


def cmd_status(manuscript_dir: Path) -> int:
    state = _require_state(manuscript_dir)
    print(json.dumps(state, indent=2))
    return 0


def cmd_diff(manuscript_dir: Path) -> int:
    state = _require_state(manuscript_dir)
    print(json.dumps(diff_manuscript(manuscript_dir, state), indent=2))
    return 0


def cmd_snapshot(manuscript_dir: Path) -> int:
    state = _require_state(manuscript_dir)
    state["chapter_fingerprints"] = fingerprint_manuscript(manuscript_dir)
    save_state(manuscript_dir, state)
    print(f"Snapshotted {len(state['chapter_fingerprints'])} chapters as assessed.")
    return 0


def cmd_phase(manuscript_dir: Path, phase: str) -> int:
    if phase not in PHASES:
        print(f"Unknown phase {phase!r}. Must be one of: {', '.join(PHASES)}")
        return 1
    state = _require_state(manuscript_dir)
    previous = state.get("phase")
    state["phase"] = phase
    if phase == "developmental" and previous != "developmental":
        state["developmental_round"] = state.get("developmental_round", 0) + 1
    save_state(manuscript_dir, state)
    print(
        f"Phase: {previous} -> {phase} (developmental_round: {state.get('developmental_round', 0)})"
    )
    return 0


def main(argv: list) -> int:
    if len(argv) < 3:
        print(__doc__)
        return 1

    command, manuscript_dir = argv[1], Path(argv[2])
    if not manuscript_dir.is_dir():
        print(f"No such directory: {manuscript_dir}")
        return 1

    if command == "init":
        return cmd_init(manuscript_dir)
    if command == "status":
        return cmd_status(manuscript_dir)
    if command == "diff":
        return cmd_diff(manuscript_dir)
    if command == "snapshot":
        return cmd_snapshot(manuscript_dir)
    if command == "phase":
        if len(argv) < 4:
            print("phase requires a target phase")
            return 1
        return cmd_phase(manuscript_dir, argv[3])

    print(f"Unknown command {command!r}")
    print(__doc__)
    return 1


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
