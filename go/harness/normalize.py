"""Shared normalization for differential-harness comparisons.

Both the CLI and (indirectly, since it wraps the same schemas functions)
the MCP front door emit JSON that includes wall-clock timestamps
(`created_at`/`updated_at` in state.json). Those are expected to differ
run-to-run and language-to-language (see TODO.md's "Port to a compiled
language" section: Python's `isoformat()` vs a Go RFC3339 formatter
aren't going to match, and that's fine -- nothing but redliner itself
reads these files). Stripping them here is what makes "parsed JSON with
timestamps normalized out" (the harness's stated comparison rule) an
actual implementation rather than a description.

Deliberately NOT sorting keys or otherwise reshaping structure -- the
point is to compare the *content* two implementations produce, not to
launder away a real divergence by normalizing too aggressively.
"""

from __future__ import annotations

TIMESTAMP_KEYS = {"created_at", "updated_at"}


def strip_timestamps(value):
    """Recursively drop known timestamp keys from a JSON-shaped value."""
    if isinstance(value, dict):
        return {
            k: strip_timestamps(v) for k, v in value.items() if k not in TIMESTAMP_KEYS
        }
    if isinstance(value, list):
        return [strip_timestamps(v) for v in value]
    return value
