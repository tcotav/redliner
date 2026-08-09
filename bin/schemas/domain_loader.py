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

# bin/schemas/domain_loader.py -> bin/schemas -> bin -> plugin root
PLUGIN_ROOT = Path(__file__).resolve().parent.parent.parent
DOMAINS_DIR = PLUGIN_ROOT / "domains"

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
