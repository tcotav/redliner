"""Shared vocabulary for redliner's findings, and validators for the JSON
each subagent writes.

There's no API-level schema enforcement in this version of redliner — that
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

## What's generic here vs. what's domain vocabulary

`SEVERITIES` and `STATUSES` are universal — every domain's findings carry
a severity and a status. The *category* vocabulary (what a developmental
or line finding can be about) is not: `plot`/`character_arc` mean nothing
for a design doc. Categories come from the manuscript's domain config
(`domains/<name>/domain.json`, loaded by `domain_loader.py`) and get
passed into these validators as a parameter — this module doesn't know or
care what domain it's validating.

`DEFERRED_CATEGORY` is the one exception: it's a protocol-level marker
(the developmental pass observed something prose-level and is handing it
to the line pass), not domain content, so it's a fixed constant rather
than something a domain declares.
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

# Prose-level observations noticed during a developmental read are
# recorded under this category and deferred to the line phase rather than
# acted on -- a developmental editor does notice prose, but polishing it
# before the structure settles is wasted work. Fixed across domains: it's
# how the two phases hand off, not domain content.
DEFERRED_CATEGORY = "deferred_to_line"

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


def validate_developmental_report(report: dict, categories: set) -> list:
    """`categories` is the domain's developmental-phase category set (from
    `domain.json`) -- `DEFERRED_CATEGORY` is allowed automatically, not
    part of what the caller needs to pass."""
    errors: list = []
    if not isinstance(report, dict):
        return ["report is not a JSON object"]

    if not report.get("scope"):
        errors.append("missing/empty 'scope'")

    if not isinstance(report.get("round"), int) or report.get("round", 0) < 1:
        errors.append("'round' must be an integer >= 1")

    # A developmental pass runs unattended -- subagents have no channel to
    # ask the author anything mid-read, and interrupting a whole-manuscript
    # read wouldn't improve it anyway. So ambiguity the brief didn't cover
    # gets recorded here instead of guessed at silently. Each entry is a gap
    # to fix in the brief, not a question to answer live.
    assumptions = report.get("assumptions")
    if not isinstance(assumptions, list):
        errors.append(
            "'assumptions' must be a list (empty if the brief covered everything)"
        )
    else:
        for i, item in enumerate(assumptions):
            if not isinstance(item, dict):
                errors.append(f"assumptions[{i}]: not an object")
                continue
            for key in ("assumption", "because", "affects"):
                if key not in item:
                    errors.append(f"assumptions[{i}]: missing {key!r}")
            if "affects" in item and not isinstance(item["affects"], list):
                errors.append(
                    f"assumptions[{i}]: 'affects' must be a list of finding ids"
                )

    findings = report.get("findings")
    if not isinstance(findings, list):
        errors.append("'findings' is not a list")
        return errors

    allowed = set(categories) | {DEFERRED_CATEGORY}
    seen_ids: set = set()
    for i, finding in enumerate(findings):
        _check_finding(finding, allowed, i, errors, DEV_ID_PATTERN, seen_ids)
    return errors


def validate_line_report(report: dict, categories: set) -> list:
    """`categories` is the domain's line-phase category set (from
    `domain.json`)."""
    errors: list = []
    if not isinstance(report, dict):
        return ["report is not a JSON object"]

    if not report.get("section"):
        errors.append("missing/empty 'section'")

    findings = report.get("findings")
    if not isinstance(findings, list):
        errors.append("'findings' is not a list")
        return errors

    seen_ids: set = set()
    for i, finding in enumerate(findings):
        _check_finding(finding, set(categories), i, errors, LINE_ID_PATTERN, seen_ids)
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
