"""Loads a domain's vocabulary config.

A domain declares the category vocabulary each phase's agents may use,
the continuity layer's entity types/sources/categories, and which phase
is the iterative, round-tracked one. Domains live at
`<plugin_root>/domains/<name>/domain.json`.

This is what keeps the engine (state machine, schemas, validators)
domain-agnostic: it asks "what categories does this manuscript's domain
allow" at validation time, rather than any of those categories being
hardcoded as fiction vocabulary.
"""

from __future__ import annotations

import json
from pathlib import Path


def _find_domains_dir() -> Path:
    """Locate `domains/` relative to this file, without assuming a fixed
    nesting depth.

    The two plugin roots this file ships from don't nest the same way:
    `bin/schemas/domain_loader.py` sits two levels under the repo/plugin
    root (`bin/schemas` -> `bin` -> root, so `domains/` is three levels
    up), but in the installed `redliner-cowork` plugin, `cowork/` *is*
    the plugin root itself (`cowork/schemas` -> `cowork`, so `domains/`
    is only two levels up). A fixed `parent.parent.parent` is correct for
    the former and silently walks outside the plugin root for the
    latter — the same path-traversal class that already broke this
    file's sibling imports once (see TODO.md's "Cowork support" section).
    Checking each candidate depth and taking the nearest hit that
    actually exists works for both without hardcoding either layout.
    """
    schemas_dir = Path(__file__).resolve().parent
    for ancestor in (schemas_dir.parent, schemas_dir.parent.parent):
        candidate = ancestor / "domains"
        if candidate.is_dir():
            return candidate
    # Nothing found at either depth -- fall back to the historical
    # three-levels-up guess so error messages still point somewhere
    # plausible rather than crashing on `PLUGIN_ROOT` being undefined.
    return schemas_dir.parent.parent / "domains"


DOMAINS_DIR = _find_domains_dir()

REQUIRED_KEYS = {
    "name",
    "display_name",
    "round_tracked_phase",
    "developmental_categories",
    "line_categories",
    "continuity",
    "brief_fields",
    "draft_stages",
}
REQUIRED_CONTINUITY_KEYS = {"entity_types", "sources", "categories"}
REQUIRED_BRIEF_FIELD_KEYS = {"name", "label", "prompt"}
REQUIRED_DRAFT_STAGE_KEYS = {"name", "implication"}


class DomainError(Exception):
    pass


def domain_path(name: str) -> Path:
    return DOMAINS_DIR / name / "domain.json"


def list_domains() -> list:
    if not DOMAINS_DIR.is_dir():
        return []
    return sorted(p.name for p in DOMAINS_DIR.iterdir() if (p / "domain.json").exists())


def load_domain(name: str) -> dict:
    path = domain_path(name)
    if not path.exists():
        available = ", ".join(list_domains()) or "(none)"
        raise DomainError(f"No domain config at {path}. Available domains: {available}")

    domain = json.loads(path.read_text(encoding="utf-8"))

    missing = REQUIRED_KEYS - set(domain)
    if missing:
        raise DomainError(f"{path}: missing required keys {sorted(missing)}")

    continuity_missing = REQUIRED_CONTINUITY_KEYS - set(domain.get("continuity", {}))
    if continuity_missing:
        raise DomainError(
            f"{path}: 'continuity' missing required keys {sorted(continuity_missing)}"
        )

    if not domain.get("brief_fields"):
        raise DomainError(f"{path}: 'brief_fields' must be a non-empty list")
    for field in domain["brief_fields"]:
        missing = REQUIRED_BRIEF_FIELD_KEYS - set(field)
        if missing:
            raise DomainError(
                f"{path}: brief_fields entry {field!r} missing keys {sorted(missing)}"
            )

    if not domain.get("draft_stages"):
        raise DomainError(f"{path}: 'draft_stages' must be a non-empty list")
    for stage in domain["draft_stages"]:
        missing = REQUIRED_DRAFT_STAGE_KEYS - set(stage)
        if missing:
            raise DomainError(
                f"{path}: draft_stages entry {stage!r} missing keys {sorted(missing)}"
            )

    return domain
