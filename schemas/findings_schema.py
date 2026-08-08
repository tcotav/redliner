"""Shared vocabulary for edaitor's findings, and validators for the JSON
each subagent writes.

There's no API-level schema enforcement in this version of edaitor — that
was the payoff of ADK's `output_schema`, which required an Anthropic API
key we decided not to spend on (see README's "Why this version exists").
Instead, subagents are *instructed* to write JSON matching these shapes,
and this module checks their work after the fact.

That's a real tradeoff worth sitting with, not a minor implementation
detail: prompt-enforced structure is not the same guarantee as
framework-enforced structure. A validation step like this is how you buy
back some of that gap when the substrate you're on doesn't give it to you
for free.

## Why findings carry IDs and status

Developmental editing is iterative: the author reads a finding, revises,
and comes back. That only works if a finding has a stable identity across
rounds — you can't mark "the third item in the array" as addressed when
the array gets regenerated. So every finding has an `id` (unique within
its report) and a `status`, and `findings/` becomes a *mutable record*
across a revision cycle rather than write-once output.
"""

from __future__ import annotations

import re

SEVERITIES = {"minor", "moderate", "major", "critical"}

STATUSES = {
    "open",  # raised, not yet addressed
    "addressed",  # author revised; verified by a re-check pass
    "claimed",  # author says it's addressed; not yet verified
    "stale",  # manuscript changed so much this finding no longer applies as written
    "wontfix",  # author considered it and declined; don't re-raise
}

DEVELOPMENTAL_CATEGORIES = {
    "plot",
    "pacing",
    "character_arc",
    "structure",
    "stakes",
    "theme",
    # Prose-level observations noticed during a developmental read are
    # recorded here and deferred to the line phase rather than acted on --
    # a developmental editor does notice prose, but polishing it before the
    # structure settles is wasted work.
    "deferred_to_line",
}

LINE_CATEGORIES = {
    "prose_rhythm",
    "voice_consistency",
    "show_dont_tell",
    "dialogue",
    "pov",
    "word_choice",
}

DEV_ID_PATTERN = re.compile(r"^dev-\d{3}$")
LINE_ID_PATTERN = re.compile(r"^line-[a-z0-9_]+-\d{3}$")


def _check_finding(
    finding: dict,
    categories: set,
    index: int,
    errors: list,
    id_pattern: re.Pattern,
    seen_ids: set,
) -> None:
    prefix = f"findings[{index}]"
    if not isinstance(finding, dict):
        errors.append(f"{prefix}: not an object")
        return

    finding_id = finding.get("id")
    if not isinstance(finding_id, str) or not id_pattern.match(finding_id or ""):
        errors.append(
            f"{prefix}: id {finding_id!r} does not match {id_pattern.pattern}"
        )
    elif finding_id in seen_ids:
        errors.append(f"{prefix}: duplicate id {finding_id!r}")
    else:
        seen_ids.add(finding_id)

    status = finding.get("status")
    if status not in STATUSES:
        errors.append(f"{prefix}: status {status!r} not in {sorted(STATUSES)}")

    category = finding.get("category")
    if category not in categories:
        errors.append(f"{prefix}: category {category!r} not in {sorted(categories)}")

    severity = finding.get("severity")
    if severity not in SEVERITIES:
        errors.append(f"{prefix}: severity {severity!r} not in {sorted(SEVERITIES)}")

    if not finding.get("location"):
        errors.append(f"{prefix}: missing/empty 'location'")

    if not finding.get("note"):
        errors.append(f"{prefix}: missing/empty 'note'")


def validate_developmental_report(report: dict) -> list:
    errors: list = []
    if not isinstance(report, dict):
        return ["report is not a JSON object"]

    if not report.get("scope"):
        errors.append("missing/empty 'scope'")

    if not isinstance(report.get("round"), int) or report.get("round", 0) < 1:
        errors.append("'round' must be an integer >= 1")

    findings = report.get("findings")
    if not isinstance(findings, list):
        errors.append("'findings' is not a list")
        return errors

    seen_ids: set = set()
    for i, finding in enumerate(findings):
        _check_finding(
            finding, DEVELOPMENTAL_CATEGORIES, i, errors, DEV_ID_PATTERN, seen_ids
        )
    return errors


def validate_line_report(report: dict) -> list:
    errors: list = []
    if not isinstance(report, dict):
        return ["report is not a JSON object"]

    if not report.get("chapter"):
        errors.append("missing/empty 'chapter'")

    findings = report.get("findings")
    if not isinstance(findings, list):
        errors.append("'findings' is not a list")
        return errors

    seen_ids: set = set()
    for i, finding in enumerate(findings):
        _check_finding(finding, LINE_CATEGORIES, i, errors, LINE_ID_PATTERN, seen_ids)
    return errors


def validate_editorial_letter(letter: dict) -> list:
    errors: list = []
    if not isinstance(letter, dict):
        return ["letter is not a JSON object"]

    for key in ("summary", "developmental_notes", "line_notes"):
        if not letter.get(key):
            errors.append(f"missing/empty '{key}'")

    top_priorities = letter.get("top_priorities")
    if not isinstance(top_priorities, list) or not top_priorities:
        errors.append("'top_priorities' must be a non-empty list")

    return errors
