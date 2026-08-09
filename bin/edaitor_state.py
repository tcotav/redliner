#!/usr/bin/env python3
"""CLI for reading and updating a manuscript's edaitor state.

The skill shells out to this instead of reasoning about state itself.
Phase transitions, section hashing, and change detection are deterministic
work — putting them in a script means they behave the same every run and
can't be talked out of by a persuasive-sounding prompt.

Lives in the plugin's bin/, which Claude Code adds to the Bash tool's PATH
while the plugin is enabled — so this runs as `edaitor_state.py ...` from
any working directory, not just the plugin's own. See the sys.path
bootstrap below for how it finds its sibling `schemas` package regardless
of cwd or how it was invoked.

Usage:
    edaitor_state.py status   <manuscript_dir>
    edaitor_state.py init     <manuscript_dir> [domain]   # domain defaults to "fiction"
    edaitor_state.py diff     <manuscript_dir>
    edaitor_state.py snapshot <manuscript_dir>            # record current text as assessed
    edaitor_state.py phase    <manuscript_dir> <phase>
"""

import json
import sys
from pathlib import Path

# Make `schemas` importable regardless of cwd or invocation method (direct
# exec via PATH, explicit `python3 /abs/path/edaitor_state.py`, etc.).
# Python only auto-adds a script's own directory to sys.path for the
# simplest invocation forms; inserting it explicitly, resolved through any
# symlink, is the version that doesn't depend on which of those forms was
# used.
sys.path.insert(0, str(Path(__file__).resolve().parent))

from schemas.domain_loader import DomainError, load_domain
from schemas.project_state import (
    DEFAULT_DOMAIN,
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


def cmd_init(manuscript_dir: Path, domain: str = DEFAULT_DOMAIN) -> int:
    if load_state(manuscript_dir) is not None:
        print(f"State already exists at {state_path(manuscript_dir)}")
        return 1
    try:
        load_domain(domain)  # fail fast on a typo'd/missing domain, not at first use
    except DomainError as e:
        print(f"Domain config error: {e}")
        return 1
    state = new_state(manuscript_dir, domain=domain)
    path = save_state(manuscript_dir, state)
    print(f"Initialized {path} (domain: {domain}, phase: intake)")
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
    state["section_fingerprints"] = fingerprint_manuscript(manuscript_dir)
    save_state(manuscript_dir, state)
    print(f"Snapshotted {len(state['section_fingerprints'])} sections as assessed.")
    return 0


def cmd_phase(manuscript_dir: Path, phase: str) -> int:
    if phase not in PHASES:
        print(f"Unknown phase {phase!r}. Must be one of: {', '.join(PHASES)}")
        return 1
    state = _require_state(manuscript_dir)

    try:
        domain = load_domain(state.get("domain", DEFAULT_DOMAIN))
    except DomainError as e:
        print(f"Domain config error: {e}")
        return 1
    round_tracked_phase = domain["round_tracked_phase"]

    previous = state.get("phase")
    state["phase"] = phase
    # Entering the domain's round-tracked phase (fiction: "developmental")
    # from anywhere else starts a new round -- not hardcoded to the literal
    # string "developmental" so a domain can name/choose this phase itself.
    if phase == round_tracked_phase and previous != round_tracked_phase:
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
        domain = argv[3] if len(argv) > 3 else DEFAULT_DOMAIN
        return cmd_init(manuscript_dir, domain)
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
