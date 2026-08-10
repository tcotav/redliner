#!/usr/bin/env python3
"""MCP server exposing redliner's deterministic operations as tools, for
plugin hosts that don't allow a top-level bin/ directory on PATH (Cowork
rejects that shape outright -- see TODO.md's "Cowork support" section for
the exact rejection and why mcpServers is the only viable alternative).

This is a second front door onto the *same* logic `bin/redliner_state.py`,
`bin/redliner_canon.py`, `bin/redliner_domain.py`, and
`bin/validate_findings.py` already expose over the CLI -- not a
reimplementation. `cowork/schemas`, `cowork/redliner_canon.py`, and
`cowork/validate_findings.py` are all symlinks into `bin/` (the actual
shared logic lives there, in one place). This file imports the latter
two directly, for the two operations (`canon_reconcile`,
`validate_findings`) that only exist as CLI `main()`/`cmd_*` functions
rather than as schemas-level functions returning structured data. Where
a CLI command communicates via stdout prints and file writes rather than
a return value, this wrapper captures stdout or reads back the written
file, instead of re-deriving the same computation a second way -- see
each tool's docstring for which applies.

Every import here is deliberately relative to this file's own directory,
not `../bin` -- a plugin cannot reference files outside its own
directory once installed (Claude Code's plugin cache only copies what's
inside the plugin root, dereferencing same-marketplace symlinks like the
ones above into real copies, but a runtime `sys.path` reach at `../bin`
would land outside the installed plugin's directory entirely and find
nothing there). This was a real bug caught by an actual local
marketplace-install test, not by reasoning about the docs alone --
`import redliner_canon` failed with `ModuleNotFoundError` the moment
this ran from the real plugin cache instead of straight from source.

Tool names mirror their CLI subcommand names 1:1 (`state_status` ~
`redliner_state.py status`), deliberately, so a future Go dual-mode
binary (CLI + MCP in one compiled artifact) can reuse this exact naming
without a second design pass.
"""
from __future__ import annotations

import contextlib
import io
import json
import sys
from pathlib import Path

# This file's own directory holds schemas/, redliner_canon.py, and
# validate_findings.py -- all either symlinked in (dev) or copied in
# (installed), so importing relative to here works in both cases. See
# the module docstring above for why `../bin` doesn't.
sys.path.insert(0, str(Path(__file__).resolve().parent))

import redliner_canon
import validate_findings as validate_findings_cli
from mcp.server import MCPServer
from schemas.domain_loader import DomainError, list_domains, load_domain
from schemas.project_state import (
    DEFAULT_DOMAIN,
    PHASES,
    SectionCollisionError,
    diff_manuscript,
    fingerprint_manuscript,
    load_state,
    new_state,
    save_state,
    state_path,
)

mcp = MCPServer("redliner")


def _require_state(manuscript_dir: Path) -> dict | None:
    return load_state(manuscript_dir)


# ---------------------------------------------------------------------
# state_* -- mirrors bin/redliner_state.py
# ---------------------------------------------------------------------


@mcp.tool()
def state_init(manuscript_dir: str, domain: str = DEFAULT_DOMAIN) -> dict:
    """Initialize redliner state for a manuscript directory, in the given
    domain (defaults to "fiction"). Fails if state already exists, or if
    the domain name doesn't match a real domain config. Mirrors
    `redliner_state.py init <manuscript_dir> [domain]`."""
    path = Path(manuscript_dir)
    if load_state(path) is not None:
        return {"error": f"State already exists at {state_path(path)}"}
    try:
        load_domain(domain)
    except DomainError as e:
        return {"error": f"Domain config error: {e}"}
    state = new_state(path, domain=domain)
    saved_path = save_state(path, state)
    return {
        "status": "initialized",
        "path": str(saved_path),
        "domain": domain,
        "phase": "intake",
    }


@mcp.tool()
def state_status(manuscript_dir: str) -> dict:
    """Report a manuscript's current redliner state (domain, phase, round,
    section fingerprints) as JSON. Mirrors `redliner_state.py status
    <manuscript_dir>`."""
    state = _require_state(Path(manuscript_dir))
    if state is None:
        return {
            "error": f"No redliner state in {manuscript_dir}. "
            f"Call state_init first."
        }
    return state


@mcp.tool()
def state_diff(manuscript_dir: str) -> dict:
    """Compare the manuscript's text on disk against the last assessed
    snapshot; returns a verdict (unchanged/targeted/restructured) plus
    which sections changed. Mirrors `redliner_state.py diff
    <manuscript_dir>`."""
    path = Path(manuscript_dir)
    state = _require_state(path)
    if state is None:
        return {"error": f"No redliner state in {manuscript_dir}"}
    try:
        return diff_manuscript(path, state)
    except SectionCollisionError as e:
        return {"error": f"Section file error: {e}"}


@mcp.tool()
def state_snapshot(manuscript_dir: str) -> dict:
    """Record the manuscript's current text as the assessed baseline, so a
    later state_diff can tell what changed. Mirrors `redliner_state.py
    snapshot <manuscript_dir>`."""
    path = Path(manuscript_dir)
    state = _require_state(path)
    if state is None:
        return {"error": f"No redliner state in {manuscript_dir}"}
    try:
        state["section_fingerprints"] = fingerprint_manuscript(path)
    except SectionCollisionError as e:
        return {"error": f"Section file error: {e}"}
    save_state(path, state)
    return {
        "status": "snapshotted",
        "section_count": len(state["section_fingerprints"]),
    }


@mcp.tool()
def state_phase(manuscript_dir: str, phase: str) -> dict:
    """Move a manuscript to a new phase (intake/developmental/line/
    complete). Entering the domain's round-tracked phase from elsewhere
    increments the round counter automatically. Mirrors `redliner_state.py
    phase <manuscript_dir> <phase>`."""
    if phase not in PHASES:
        return {"error": f"Unknown phase {phase!r}. Must be one of: {', '.join(PHASES)}"}
    path = Path(manuscript_dir)
    state = _require_state(path)
    if state is None:
        return {"error": f"No redliner state in {manuscript_dir}"}

    try:
        domain = load_domain(state.get("domain", DEFAULT_DOMAIN))
    except DomainError as e:
        return {"error": f"Domain config error: {e}"}
    round_tracked_phase = domain["round_tracked_phase"]

    previous = state.get("phase")
    state["phase"] = phase
    if phase == round_tracked_phase and previous != round_tracked_phase:
        state["developmental_round"] = state.get("developmental_round", 0) + 1
    save_state(path, state)
    return {
        "previous_phase": previous,
        "phase": phase,
        "developmental_round": state.get("developmental_round", 0),
    }


# ---------------------------------------------------------------------
# canon_* -- mirrors bin/redliner_canon.py
# ---------------------------------------------------------------------


@mcp.tool()
def canon_stale(manuscript_dir: str) -> dict:
    """Report which sections need (re-)extraction for the continuity layer
    -- never extracted, or changed since their facts were extracted --
    along with each such section's current hash and any orphaned
    observation files. Mirrors `redliner_canon.py stale <manuscript_dir>`."""
    path = Path(manuscript_dir)
    observations = redliner_canon.load_observations(path)
    stale, missing = [], []
    current_hashes = {}

    try:
        sections = redliner_canon.section_files(path)
    except SectionCollisionError as e:
        return {"error": f"Section file error: {e}"}

    for section_path in sections:
        recorded = observations.get(section_path.stem)
        section_hash = redliner_canon.fingerprint_section(section_path)["sha256"]
        if recorded is None:
            missing.append(section_path.stem)
            current_hashes[section_path.stem] = section_hash
            continue
        if recorded.get("section_sha256") != section_hash:
            stale.append(section_path.stem)
            current_hashes[section_path.stem] = section_hash

    orphaned = sorted(set(observations) - {p.stem for p in sections})

    return {
        "needs_extraction": sorted(missing + stale),
        "never_extracted": missing,
        "changed_since_extraction": stale,
        "current_hashes": current_hashes,
        "orphaned_observations": orphaned,
    }


@mcp.tool()
def canon_reconcile(manuscript_dir: str) -> dict:
    """Rebuild the merged canon and find continuity collisions from every
    current observations file. Writes canon.json and collisions.json to
    the manuscript's .redliner/canon/ directory (same side effect as the
    CLI) and returns their contents. Mirrors `redliner_canon.py reconcile
    <manuscript_dir>`."""
    path = Path(manuscript_dir)
    buf = io.StringIO()
    with contextlib.redirect_stdout(buf):
        exit_code = redliner_canon.cmd_reconcile(path)
    if exit_code != 0:
        return {"error": buf.getvalue().strip()}

    canon_dir = redliner_canon.canon_dir(path)
    canon = json.loads((canon_dir / "canon.json").read_text(encoding="utf-8"))
    collisions = json.loads(
        (canon_dir / "collisions.json").read_text(encoding="utf-8")
    )
    return {"canon": canon, "collisions": collisions["collisions"]}


# ---------------------------------------------------------------------
# domain_* -- mirrors bin/redliner_domain.py
# ---------------------------------------------------------------------


@mcp.tool()
def domain_list() -> list:
    """List every domain config available (name, display name,
    description). Mirrors `redliner_domain.py list`."""
    domains = []
    for name in list_domains():
        try:
            d = load_domain(name)
        except DomainError:
            continue
        domains.append(
            {
                "name": d["name"],
                "display_name": d["display_name"],
                "description": d.get("description", ""),
            }
        )
    return domains


@mcp.tool()
def domain_show(name: str) -> dict:
    """Show the full config for one named domain -- categories, continuity
    vocabulary, brief fields, draft stages. Mirrors `redliner_domain.py
    show <name>`."""
    try:
        return load_domain(name)
    except DomainError as e:
        return {"error": str(e)}


# ---------------------------------------------------------------------
# validate_findings -- mirrors bin/validate_findings.py
# ---------------------------------------------------------------------


@mcp.tool()
def validate_findings(manuscript_dir: str) -> dict:
    """Validate everything currently under a manuscript's .redliner/
    directory (canon observations, continuity, developmental/line
    findings, editorial letter) against its domain's schema, including
    excerpt-verbatim checks against the actual section text. Mirrors
    `validate_findings.py <manuscript_dir>` -- same pass/fail logic and
    per-file detail, captured from its stdout rather than re-derived."""
    buf = io.StringIO()
    with contextlib.redirect_stdout(buf):
        exit_code = validate_findings_cli.main(manuscript_dir)
    return {"ok": exit_code == 0, "output": buf.getvalue()}


if __name__ == "__main__":
    mcp.run()
