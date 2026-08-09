#!/usr/bin/env python3
"""Merge per-section observations into a canon, and find collisions.

Lives in the plugin's bin/ (on PATH while the plugin is enabled) -- runs
as `edaitor_canon.py ...` from any working directory. See the sys.path
bootstrap below for how it finds its sibling `schemas` package.

Two commands:

    edaitor_canon.py stale     <manuscript_dir>   # which sections need re-extraction
    edaitor_canon.py reconcile <manuscript_dir>   # build canon + find collisions

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
    grouped = defaultdict(list)
    for section, report in observations.items():
        for fact in report.get("facts", []):
            facts_by_id[fact["id"]] = {**fact, "section": section}
            key = (fact["entity"].strip().lower(), fact["attribute"].strip().lower())
            grouped[key].append(fact["id"])

    collisions = []
    for (entity, attribute), fact_ids in sorted(grouped.items()):
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
