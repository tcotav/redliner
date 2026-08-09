"""Continuity layer: facts extracted from the manuscript, and the
contradictions found among them.

## Why this is a layer and not a phase

Developmental editing precedes line editing because structural revision
destroys line work — no point polishing a sentence in a scene you'll cut.
Continuity isn't destroyed that way. Cut a chapter and the canon simply
loses those facts; the ones that remain are still true. So continuity
runs alongside whatever phase you're in rather than waiting its turn.

## Why extraction carries no judgment

Extraction is deliberately dumb: it records what the text asserts, with
provenance, and nothing else. A `Fact` has no `note`, no `severity`, no
`concern` field — there is nowhere for an opinion to live, and
`validate_observations` rejects unknown keys outright. This is
structural, not stylistic: an extractor that can editorialize will start
flagging contradictions it half-noticed, and then contradiction detection
is happening in two places with two standards.

Judgment happens once, in reconciliation, and only on collisions a script
already found mechanically.
"""

from __future__ import annotations

import re

from schemas.findings_schema import SEVERITIES, STATUSES

ENTITY_TYPES = {
    "character",
    "place",
    "object",
    "organization",
    "event",
    "world_rule",
}

# Where the assertion comes from. This matters in fiction: two narration
# facts that disagree is an error, but a character stating something the
# narration contradicts may just be a character who is lying or wrong.
SOURCES = {"narration", "dialogue", "character_thought"}

# "her green eyes" is explicit; "she reached the top shelf easily" implies
# height. Implied facts shouldn't drive hard contradictions on their own.
CONFIDENCES = {"explicit", "implied"}

CONTRADICTION_KINDS = {
    "contradiction",  # two assertions that cannot both be true
    "unverified",  # looks wrong, but needs the author -- lying character,
    # unreliable narrator, or possible in-world explanation
}

CONTINUITY_CATEGORIES = {
    "character_attribute",
    "timeline",
    "geography",
    "world_rule",
    "naming",
    "relationship",
    "object",
}

FACT_REQUIRED_KEYS = {
    "id",
    "entity",
    "entity_type",
    "attribute",
    "value",
    "excerpt",
    "location",
    "source",
    "confidence",
}
# Intentionally empty. Adding an optional key here is how this schema stops
# being judgment-free -- think hard before doing it.
FACT_OPTIONAL_KEYS: set = set()

FACT_ID_PATTERN = re.compile(r"^fact-[a-z0-9_]+-\d{3}$")
CONTRADICTION_ID_PATTERN = re.compile(r"^cont-\d{3}$")


def _check_fact(fact: dict, index: int, errors: list, seen_ids: set) -> None:
    prefix = f"facts[{index}]"
    if not isinstance(fact, dict):
        errors.append(f"{prefix}: not an object")
        return

    keys = set(fact)
    missing = FACT_REQUIRED_KEYS - keys
    if missing:
        errors.append(f"{prefix}: missing keys {sorted(missing)}")

    # The point of the whole schema: no room for an opinion.
    extra = keys - FACT_REQUIRED_KEYS - FACT_OPTIONAL_KEYS
    if extra:
        errors.append(
            f"{prefix}: unexpected keys {sorted(extra)} — extraction records facts, not judgments"
        )

    fact_id = fact.get("id")
    if not isinstance(fact_id, str) or not FACT_ID_PATTERN.match(fact_id or ""):
        errors.append(
            f"{prefix}: id {fact_id!r} does not match {FACT_ID_PATTERN.pattern}"
        )
    elif fact_id in seen_ids:
        errors.append(f"{prefix}: duplicate id {fact_id!r}")
    else:
        seen_ids.add(fact_id)

    if fact.get("entity_type") not in ENTITY_TYPES:
        errors.append(
            f"{prefix}: entity_type {fact.get('entity_type')!r} not in {sorted(ENTITY_TYPES)}"
        )

    if fact.get("source") not in SOURCES:
        errors.append(
            f"{prefix}: source {fact.get('source')!r} not in {sorted(SOURCES)}"
        )

    if fact.get("confidence") not in CONFIDENCES:
        errors.append(
            f"{prefix}: confidence {fact.get('confidence')!r} not in {sorted(CONFIDENCES)}"
        )

    for key in ("entity", "attribute", "value", "excerpt", "location"):
        if key in keys and not str(fact.get(key) or "").strip():
            errors.append(f"{prefix}: missing/empty {key!r}")


def validate_observations(report: dict) -> list:
    """Validate one chapter's extracted facts."""
    errors: list = []
    if not isinstance(report, dict):
        return ["observations file is not a JSON object"]

    if not report.get("chapter"):
        errors.append("missing/empty 'chapter'")

    # Recorded so the skill can skip re-extracting unchanged chapters.
    if not report.get("chapter_sha256"):
        errors.append("missing/empty 'chapter_sha256'")

    facts = report.get("facts")
    if not isinstance(facts, list):
        errors.append("'facts' is not a list")
        return errors

    seen_ids: set = set()
    for i, fact in enumerate(facts):
        _check_fact(fact, i, errors, seen_ids)
    return errors


def validate_continuity_report(report: dict) -> list:
    """Validate the adjudicated contradictions."""
    errors: list = []
    if not isinstance(report, dict):
        return ["continuity report is not a JSON object"]

    contradictions = report.get("contradictions")
    if not isinstance(contradictions, list):
        errors.append("'contradictions' is not a list")
        return errors

    seen_ids: set = set()
    for i, item in enumerate(contradictions):
        prefix = f"contradictions[{i}]"
        if not isinstance(item, dict):
            errors.append(f"{prefix}: not an object")
            continue

        item_id = item.get("id")
        if not isinstance(item_id, str) or not CONTRADICTION_ID_PATTERN.match(
            item_id or ""
        ):
            errors.append(
                f"{prefix}: id {item_id!r} does not match {CONTRADICTION_ID_PATTERN.pattern}"
            )
        elif item_id in seen_ids:
            errors.append(f"{prefix}: duplicate id {item_id!r}")
        else:
            seen_ids.add(item_id)

        if item.get("status") not in STATUSES:
            errors.append(
                f"{prefix}: status {item.get('status')!r} not in {sorted(STATUSES)}"
            )

        if item.get("kind") not in CONTRADICTION_KINDS:
            errors.append(
                f"{prefix}: kind {item.get('kind')!r} not in {sorted(CONTRADICTION_KINDS)}"
            )

        if item.get("category") not in CONTINUITY_CATEGORIES:
            errors.append(
                f"{prefix}: category {item.get('category')!r} not in {sorted(CONTINUITY_CATEGORIES)}"
            )

        if item.get("severity") not in SEVERITIES:
            errors.append(
                f"{prefix}: severity {item.get('severity')!r} not in {sorted(SEVERITIES)}"
            )

        fact_ids = item.get("fact_ids")
        if not isinstance(fact_ids, list) or len(fact_ids) < 2:
            errors.append(
                f"{prefix}: 'fact_ids' must list at least the two conflicting facts"
            )

        if not str(item.get("note") or "").strip():
            errors.append(f"{prefix}: missing/empty 'note'")

    return errors
