"""Per-manuscript project state: what phase we're in, and what the
manuscript looked like the last time we assessed it.

State lives in `<manuscript_dir>/.edaitor/state.json` — with the
manuscript, not with the tool — so each manuscript carries its own
editing history and edaitor stays a reusable tool.

## Why hashes

Developmental editing is iterative, and the author marks findings
resolved as they revise. But responding to a structural note sometimes
means restructuring — cutting a subplot, merging chapters — and after
that, findings elsewhere may be invalidated rather than resolved. The
author can't reliably know which.

So rather than asking a model to guess whether "a lot" changed, we hash
each chapter at assessment time and diff on re-check. The diff is
deterministic and cheap; model judgment enters *after* it, applied to the
specific chapters the diff points at. Thresholds are config, not vibes.
"""

from __future__ import annotations

import hashlib
import json
from datetime import datetime, timezone
from pathlib import Path

PHASES = ("intake", "developmental", "line", "complete")

STATE_DIRNAME = ".edaitor"
STATE_FILENAME = "state.json"

# A chapter whose word count moves by more than this fraction is treated as
# rewritten rather than tweaked, which forces a full re-read. Deliberately a
# constant you can tune after watching real revisions, not a model judgment.
MAJOR_WORDCOUNT_DELTA = 0.25


def state_dir(manuscript_dir: Path) -> Path:
    return Path(manuscript_dir) / STATE_DIRNAME


def state_path(manuscript_dir: Path) -> Path:
    return state_dir(manuscript_dir) / STATE_FILENAME


def chapter_files(manuscript_dir: Path) -> list:
    return sorted(Path(manuscript_dir).glob("chapter_*.txt"))


def fingerprint_chapter(path: Path) -> dict:
    text = path.read_text(encoding="utf-8")
    return {
        "sha256": hashlib.sha256(text.encode("utf-8")).hexdigest(),
        "words": len(text.split()),
    }


def fingerprint_manuscript(manuscript_dir: Path) -> dict:
    return {
        path.stem: fingerprint_chapter(path) for path in chapter_files(manuscript_dir)
    }


def load_state(manuscript_dir: Path) -> dict | None:
    path = state_path(manuscript_dir)
    if not path.exists():
        return None
    return json.loads(path.read_text(encoding="utf-8"))


def save_state(manuscript_dir: Path, state: dict) -> Path:
    directory = state_dir(manuscript_dir)
    directory.mkdir(parents=True, exist_ok=True)
    state["updated_at"] = datetime.now(timezone.utc).isoformat()
    path = state_path(manuscript_dir)
    path.write_text(json.dumps(state, indent=2) + "\n", encoding="utf-8")
    return path


def new_state(manuscript_dir: Path) -> dict:
    return {
        "manuscript_dir": str(manuscript_dir),
        "phase": "intake",
        "developmental_round": 0,
        "chapter_fingerprints": {},
        "created_at": datetime.now(timezone.utc).isoformat(),
    }


def diff_manuscript(manuscript_dir: Path, state: dict) -> dict:
    """Compare the manuscript on disk against the last assessed fingerprints.

    Returns a verdict the skill uses to decide how much re-reading a
    re-check actually needs:

      - `unchanged`     nothing changed; "resolved" claims are unverifiable
      - `targeted`      some chapters edited, none added/removed, all deltas
                        small -> verify claimed findings against those chapters
      - `restructured`  chapters added/removed, or a large word-count swing ->
                        full re-read; prior findings may be stale, not resolved
    """
    previous = state.get("chapter_fingerprints") or {}
    current = fingerprint_manuscript(manuscript_dir)

    added = sorted(set(current) - set(previous))
    removed = sorted(set(previous) - set(current))
    changed = []
    large_delta = []

    for name in sorted(set(current) & set(previous)):
        if current[name]["sha256"] == previous[name]["sha256"]:
            continue
        changed.append(name)

        before = previous[name].get("words", 0)
        after = current[name].get("words", 0)
        if before == 0:
            large_delta.append(name)
            continue
        if abs(after - before) / before > MAJOR_WORDCOUNT_DELTA:
            large_delta.append(name)

    if added or removed or large_delta:
        verdict = "restructured"
    elif changed:
        verdict = "targeted"
    else:
        verdict = "unchanged"

    return {
        "verdict": verdict,
        "added": added,
        "removed": removed,
        "changed": changed,
        "large_delta": large_delta,
    }
