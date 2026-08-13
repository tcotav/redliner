#!/usr/bin/env python3
"""Merge per-section observations into a canon, and find collisions.

Lives in the plugin's bin/ (on PATH while the plugin is enabled) -- runs
as `redliner_canon.py ...` from any working directory. See the sys.path
bootstrap below for how it finds its sibling `schemas` package.

Two commands:

    redliner_canon.py stale     <manuscript_dir>   # which sections need re-extraction
    redliner_canon.py reconcile <manuscript_dir>   # build canon + find collisions

Finding a collision is a *computation*, not a judgment: two facts about
the same entity and attribute with different values collide, and a script
can determine that exhaustively and identically every run. Asking a model
to scan for contradictions instead would be slower, non-deterministic,
and worse at exactly the case that matters -- a detail contradicted two
hundred pages later, which is where human readers fail too.

The model's job comes after: adjudicating the collisions this finds.
Some are real errors; some are characters lying, unreliable narration, or
an in-world explanation. That needs judgment. Finding them does not.

## The revision signal

Most "contradictions" in a manuscript under active revision aren't
continuity errors at all -- they're an edit the author hasn't propagated
yet. Section 2 was rewritten; section 7 still says the old thing.

So each collision is annotated with whether its sections have changed
since the last assessment snapshot. A collision between one recently
edited section and one untouched section is probably unpropagated
revision, and saying so is far more useful to the author than "these
contradict."
"""

import json
import re
import sys
from collections import defaultdict
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

from schemas.project_state import (
    SectionCollisionError,
    diff_manuscript,
    fingerprint_section,
    load_state,
    section_files,
    state_dir,
)


def observations_dir(manuscript_dir: Path) -> Path:
    return state_dir(manuscript_dir) / "canon" / "observations"


def canon_dir(manuscript_dir: Path) -> Path:
    return state_dir(manuscript_dir) / "canon"


def load_observations(manuscript_dir: Path) -> dict:
    directory = observations_dir(manuscript_dir)
    if not directory.is_dir():
        return {}
    out = {}
    for path in sorted(directory.glob("*.json")):
        out[path.stem] = json.loads(path.read_text(encoding="utf-8"))
    return out


def cmd_stale(manuscript_dir: Path) -> int:
    """Report sections whose text has changed since their facts were extracted.

    Extraction cost scales with section count; re-reading 40 sections to
    catch one edit is how a layer becomes something you avoid running.

    Also reports each such section's *current* hash, in `current_hashes`.
    The continuity-extractor agent needs to be given its section's hash
    verbatim (it copies it into `section_sha256` rather than computing it
    itself) -- surfacing it here means the orchestrating skill can read
    one JSON blob instead of hashing sections itself or making a second
    round trip.
    """
    observations = load_observations(manuscript_dir)
    stale, missing = [], []
    current_hashes = {}

    try:
        sections = section_files(manuscript_dir)
    except SectionCollisionError as e:
        print(f"Section file error: {e}")
        return 1

    for path in sections:
        recorded = observations.get(path.stem)
        section_hash = fingerprint_section(path)["sha256"]
        if recorded is None:
            missing.append(path.stem)
            current_hashes[path.stem] = section_hash
            continue
        if recorded.get("section_sha256") != section_hash:
            stale.append(path.stem)
            current_hashes[path.stem] = section_hash

    orphaned = sorted(set(observations) - {p.stem for p in sections})

    print(
        json.dumps(
            {
                "needs_extraction": sorted(missing + stale),
                "never_extracted": missing,
                "changed_since_extraction": stale,
                "current_hashes": current_hashes,
                "orphaned_observations": orphaned,
            },
            indent=2,
        )
    )
    return 0


# --- entity/attribute normalization for collision grouping -------------
# Added 2026-08-12. Exact string matching on (entity, attribute) silently
# missed a real contradiction because two independent per-section
# extractions named the same thing differently: "tide clock" vs "the tide
# clock", attribute "duration_not_working" vs "stopped_duration". See
# TODO.md, "Continuity misses contradictions when extractions name things
# differently". Deliberately loosens matching rather than tightening
# extraction: over-reporting is reviewed by the adjudicator, while
# under-reporting has no safety net at all.
_ARTICLES = ("the ", "a ", "an ")
_ATTR_STOPWORDS = {
    "of", "the", "a", "an", "is", "was", "are", "were", "be", "been",
    "to", "in", "at", "on", "for", "not", "no",
}


def _norm_entity(value: str) -> str:
    """Lowercase, trim, drop one leading article."""
    text = str(value).strip().lower()
    for article in _ARTICLES:
        if text.startswith(article):
            return text[len(article):].strip()
    return text


def _attr_tokens(value: str) -> set:
    """Significant tokens of an attribute name, split on non-alphanumerics."""
    parts = re.split(r"[^a-z0-9]+", str(value).strip().lower())
    return {p for p in parts if p and p not in _ATTR_STOPWORDS}


def _link_by_attribute(fact_ids, facts_by_id):
    """Group facts for collision detection.

    Exact-attribute groups first (the original behaviour), then *pairwise*
    unions of two groups whose attribute names share a significant token.
    Deliberately NOT a transitive closure: chaining A~B~C merges attributes
    with nothing in common and hands the adjudicator a malformed collision
    (observed: four unrelated values fused under one label). Pairwise keeps
    each reported collision explainable.
    """
    exact = {}
    for fid in fact_ids:
        key = str(facts_by_id[fid]["attribute"]).strip().lower()
        exact.setdefault(key, []).append(fid)

    keys = sorted(exact)
    groups = [list(exact[k]) for k in keys]

    tokens = [_attr_tokens(k) for k in keys]
    for i in range(len(keys)):
        for j in range(i + 1, len(keys)):
            if tokens[i] & tokens[j]:
                merged = exact[keys[i]] + exact[keys[j]]
                merged.sort(key=fact_ids.index)
                groups.append(merged)

    # Drop any group whose fact set is contained in a larger one, so a
    # merged pair supersedes its two halves rather than reporting thrice.
    seen, out = set(), []
    for g in sorted(groups, key=len, reverse=True):
        fs = frozenset(g)
        if any(fs <= s for s in seen):
            continue
        seen.add(fs)
        out.append(g)
    out.sort(key=lambda g: fact_ids.index(g[0]))
    return out


def cmd_reconcile(manuscript_dir: Path) -> int:
    observations = load_observations(manuscript_dir)
    if not observations:
        print(
            f"No observations in {observations_dir(manuscript_dir)}. Run extraction first."
        )
        return 1

    state = load_state(manuscript_dir) or {}
    changed_since_snapshot = set()
    if state.get("section_fingerprints"):
        try:
            verdict = diff_manuscript(manuscript_dir, state)
        except SectionCollisionError as e:
            print(f"Section file error: {e}")
            return 1
        changed_since_snapshot = set(verdict["changed"]) | set(verdict["added"])

    facts_by_id = {}
    by_entity = defaultdict(list)
    for section, report in observations.items():
        for fact in report.get("facts", []):
            facts_by_id[fact["id"]] = {**fact, "section": section}
            by_entity[_norm_entity(fact["entity"])].append(fact["id"])

    # Group by normalized entity, then link facts whose attribute names
    # share a significant token, so synonymous attributes collide.
    groups = []
    for entity in sorted(by_entity):
        for fact_ids in _link_by_attribute(by_entity[entity], facts_by_id):
            sort_attr = min(
                str(facts_by_id[f]["attribute"]).strip().lower() for f in fact_ids
            )
            groups.append((entity, sort_attr, fact_ids))
    groups.sort(key=lambda g: (g[0], g[1]))

    collisions = []
    for entity, attribute, fact_ids in groups:
        values = {}
        for fact_id in fact_ids:
            value = str(facts_by_id[fact_id]["value"]).strip().lower()
            values.setdefault(value, []).append(fact_id)

        if len(values) < 2:
            continue

        involved = [facts_by_id[fid] for fid in fact_ids]
        sections = sorted({f["section"] for f in involved})
        edited = sorted(s for s in sections if s in changed_since_snapshot)
        untouched = sorted(s for s in sections if s not in changed_since_snapshot)

        collisions.append(
            {
                "entity": facts_by_id[fact_ids[0]]["entity"],
                "attribute": facts_by_id[fact_ids[0]]["attribute"],
                "distinct_values": sorted(values),
                "facts": [
                    {
                        "id": f["id"],
                        "section": f["section"],
                        "value": f["value"],
                        "excerpt": f["excerpt"],
                        "location": f["location"],
                        "source": f["source"],
                        "confidence": f["confidence"],
                    }
                    for f in involved
                ],
                # Adjudication hints -- context for the model, not verdicts.
                "all_narration": all(f["source"] == "narration" for f in involved),
                "any_implied": any(f["confidence"] == "implied" for f in involved),
                "sections_edited_since_snapshot": edited,
                "sections_untouched_since_snapshot": untouched,
                "likely_unpropagated_revision": bool(edited) and bool(untouched),
            }
        )

    canon = {
        "entities": {},
        "fact_count": len(facts_by_id),
        "sections_covered": sorted(observations),
    }
    for fact in facts_by_id.values():
        entity = canon["entities"].setdefault(
            fact["entity"], {"entity_type": fact["entity_type"], "attributes": {}}
        )
        entity["attributes"].setdefault(fact["attribute"], []).append(
            {
                "value": fact["value"],
                "section": fact["section"],
                "fact_id": fact["id"],
                "source": fact["source"],
                "confidence": fact["confidence"],
            }
        )

    out_dir = canon_dir(manuscript_dir)
    out_dir.mkdir(parents=True, exist_ok=True)
    (out_dir / "canon.json").write_text(
        json.dumps(canon, indent=2) + "\n", encoding="utf-8"
    )
    (out_dir / "collisions.json").write_text(
        json.dumps({"collisions": collisions}, indent=2) + "\n", encoding="utf-8"
    )

    print(f"Canon: {len(canon['entities'])} entities, {canon['fact_count']} facts.")
    print(f"Collisions to adjudicate: {len(collisions)}")
    for collision in collisions:
        flag = (
            " (likely unpropagated revision)"
            if collision["likely_unpropagated_revision"]
            else ""
        )
        print(
            f"  - {collision['entity']}.{collision['attribute']}: {collision['distinct_values']}{flag}"
        )
    return 0


def main(argv: list) -> int:
    if len(argv) < 3:
        print(__doc__)
        return 1

    command, manuscript_dir = argv[1], Path(argv[2])
    if not manuscript_dir.is_dir():
        print(f"No such directory: {manuscript_dir}")
        return 1

    if command == "stale":
        return cmd_stale(manuscript_dir)
    if command == "reconcile":
        return cmd_reconcile(manuscript_dir)

    print(f"Unknown command {command!r}")
    return 1


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
