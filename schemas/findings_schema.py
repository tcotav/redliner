"""Shared vocabulary for edaitor's findings, and validators for the JSON
each subagent writes to findings/.

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
"""

from __future__ import annotations

SEVERITIES = {"minor", "moderate", "major", "critical"}

DEVELOPMENTAL_CATEGORIES = {
    "plot",
    "pacing",
    "character_arc",
    "structure",
    "stakes",
    "theme",
}

LINE_CATEGORIES = {
    "prose_rhythm",
    "voice_consistency",
    "show_dont_tell",
    "dialogue",
    "pov",
    "word_choice",
}


def _check_finding(finding: dict, categories: set, index: int, errors: list) -> None:
    prefix = f"findings[{index}]"
    if not isinstance(finding, dict):
        errors.append(f"{prefix}: not an object")
        return

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

    findings = report.get("findings")
    if not isinstance(findings, list):
        errors.append("'findings' is not a list")
        return errors

    for i, finding in enumerate(findings):
        _check_finding(finding, DEVELOPMENTAL_CATEGORIES, i, errors)
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

    for i, finding in enumerate(findings):
        _check_finding(finding, LINE_CATEGORIES, i, errors)
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
