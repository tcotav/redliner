#!/usr/bin/env python3
"""Validate a manuscript's .redliner/ output against its domain's schema.

Lives in the plugin's bin/ (on PATH while the plugin is enabled) -- runs
as `validate_findings.py ...` from any working directory. See the
sys.path bootstrap below for how it finds its sibling `schemas` package.

Usage:
    validate_findings.py <manuscript_dir>

Takes a manuscript directory only -- not a findings/ or canon/ path
directly. An earlier version accepted either and inferred which; that
inference was wrong for any layout other than the one nested exactly one
level under .redliner/, and failed *silently* (exit 0, canon layer quietly
skipped) rather than erroring. One required argument, checked to actually
contain .redliner/, removes the guess entirely.

The category vocabulary checked against comes from the manuscript's
domain (`.redliner/state.json`'s `domain` field, defaulting to `fiction`
if absent) via `domain_loader.py` -- this script has no fiction-specific
knowledge itself.

Exits 0 if everything present validates cleanly, 1 otherwise. Missing
files are not errors by themselves (the pipeline may not have reached
that step yet) -- only files that exist and fail validation fail the run.

Where an excerpt is checked, it must be a genuine substring of the
section it claims to quote (whitespace-normalized, since sections are
hard-wrapped, and markdown-emphasis-normalized, since an agent quoting
"relentless" from source text reading "**relentless**" is still quoting
verbatim -- the emphasis markers aren't part of the words) -- an excerpt
an agent invented rather than quoted is worse than no excerpt, so this
isn't optional the way most content checks are.
"""

import json
import re
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

from schemas.canon_schema import (
    validate_continuity_report,
    validate_observations,
)
from schemas.domain_loader import DomainError, load_domain
from schemas.findings_schema import (
    validate_developmental_report,
    validate_editorial_letter,
    validate_line_report,
)
from schemas.project_state import DEFAULT_DOMAIN, SECTION_EXTENSIONS, load_state


def _check(path: Path, errors: list) -> bool:
    if errors:
        print(f"FAIL {path}")
        for error in errors:
            print(f"  - {error}")
        return False
    print(f"OK   {path}")
    return True


# Markdown inline emphasis/code delimiters -- stripped before the excerpt
# comparison so quoting the *words* verbatim doesn't fail just because an
# agent's excerpt (reasonably) dropped the formatting markup around them.
# Removing the same characters from both sides of the comparison can only
# make a genuine quote match; it can't make a fabricated one match, since
# that requires the actual wording to differ, not just the punctuation.
#
# Deliberately only the doubled/paired forms (`**bold**`, `__bold__`,
# `` `code` ``, `~~strike~~`), not bare single `*`/`_`. Those are common
# inside ordinary content this schema already relies on being exact --
# snake_case attribute names (`eye_color`), file paths, code identifiers --
# and stripping them would let `eye_color` and `eyecolor` normalize as
# equal, quietly widening what counts as a "verbatim" match. Extend this
# if a real bare-emphasis case shows up; don't guess at it now.
_MARKDOWN_EMPHASIS = re.compile(r"\*\*|__|`|~~")


def _normalize(text: str) -> str:
    text = _MARKDOWN_EMPHASIS.sub("", text)
    return " ".join(text.split())


def _load_section_text(manuscript_dir: Path, section_stem: str) -> str | None:
    for ext in SECTION_EXTENSIONS:
        path = manuscript_dir / f"{section_stem}{ext}"
        if path.exists():
            return path.read_text(encoding="utf-8")
    return None


def _verify_excerpts(items: list, section_text: str, label: str) -> list:
    """Check each item's `excerpt` (if present) is a real substring of the
    section it claims to quote. Returns error strings; empty if clean."""
    errors = []
    normalized_section = _normalize(section_text)
    for i, item in enumerate(items):
        excerpt = item.get("excerpt") if isinstance(item, dict) else None
        if not excerpt:
            continue
        if _normalize(excerpt) not in normalized_section:
            item_id = (
                item.get("id", f"index {i}") if isinstance(item, dict) else f"index {i}"
            )
            errors.append(
                f"{label}[{item_id}]: excerpt not found verbatim in section text: {excerpt!r}"
            )
    return errors


def _validate_canon(
    manuscript_dir: Path, redliner_path: Path, domain: dict, ok: bool
) -> bool:
    """Validate the continuity layer's files, if any exist yet."""
    canon_path = redliner_path / "canon"
    if not canon_path.is_dir():
        return ok

    continuity = domain["continuity"]
    entity_types = set(continuity["entity_types"])
    sources = set(continuity["sources"])
    categories = set(continuity["categories"])

    for obs_file in sorted((canon_path / "observations").glob("*.json")):
        report = json.loads(obs_file.read_text())
        errors = validate_observations(report, entity_types, sources)
        section_text = _load_section_text(manuscript_dir, obs_file.stem)
        if section_text is not None:
            errors += _verify_excerpts(
                report.get("facts", []), section_text, obs_file.name
            )
        ok = _check(obs_file, errors) and ok

    continuity_file = canon_path / "continuity.json"
    if continuity_file.exists():
        errors = validate_continuity_report(
            json.loads(continuity_file.read_text()), categories
        )
        ok = _check(continuity_file, errors) and ok

    return ok


def main(manuscript_dir_arg: str) -> int:
    manuscript_dir = Path(manuscript_dir_arg)
    redliner_path = manuscript_dir / ".redliner"
    if not redliner_path.is_dir():
        print(
            f"No .redliner/ under {manuscript_dir} -- pass a manuscript directory, not a findings/canon path."
        )
        return 1

    state = load_state(manuscript_dir) or {}
    domain_name = state.get("domain", DEFAULT_DOMAIN)
    try:
        domain = load_domain(domain_name)
    except DomainError as e:
        print(f"Domain config error: {e}")
        return 1

    findings_path = redliner_path / "findings"
    ok = _validate_canon(manuscript_dir, redliner_path, domain, True)

    if not findings_path.is_dir():
        print(f"No findings/ yet under {redliner_path}")
        return 0 if ok else 1

    dev_file = findings_path / "developmental.json"
    if dev_file.exists():
        errors = validate_developmental_report(
            json.loads(dev_file.read_text()), set(domain["developmental_categories"])
        )
        # Developmental findings don't carry excerpts -- they're
        # manuscript-scope, not tied to one quotable location the way a
        # line finding or an extracted fact is.
        ok = _check(dev_file, errors) and ok

    line_pattern = re.compile(r"^line_(.+)\.json$")
    line_files = sorted(findings_path.glob("line_*.json"))
    for line_file in line_files:
        report = json.loads(line_file.read_text())
        errors = validate_line_report(report, set(domain["line_categories"]))
        match = line_pattern.match(line_file.name)
        if match:
            section_text = _load_section_text(manuscript_dir, match.group(1))
            if section_text is not None:
                errors += _verify_excerpts(
                    report.get("findings", []), section_text, line_file.name
                )
        ok = _check(line_file, errors) and ok

    letter_file = findings_path / "editorial_letter.json"
    if letter_file.exists():
        errors = validate_editorial_letter(json.loads(letter_file.read_text()))
        ok = _check(letter_file, errors) and ok

    if not dev_file.exists() and not line_files and not letter_file.exists():
        print(f"Nothing to validate yet in {findings_path}")

    return 0 if ok else 1


if __name__ == "__main__":
    if len(sys.argv) < 2:
        print(__doc__)
        raise SystemExit(1)
    raise SystemExit(main(sys.argv[1]))
