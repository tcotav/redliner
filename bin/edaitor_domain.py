#!/usr/bin/env python3
"""CLI for listing and inspecting available domain configs.

Skills need this at two points: intake has to present the choice of
domain (if more than one exists) and read the active domain's
`brief_fields`/`draft_stages` to structure its interview; the
domain-creation skill needs to check whether a name is already taken and
show existing domains as reference. Exposing this as a bin/ script (same
PATH mechanism as edaitor_state.py) keeps skill prose from having to
guess the plugin's filesystem path -- it just calls a bare command, same
as everything else in bin/.

Usage:
    edaitor_domain.py list
    edaitor_domain.py show <name>
"""

import json
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

from schemas.domain_loader import DomainError, list_domains, load_domain


def cmd_list() -> int:
    domains = []
    for name in list_domains():
        try:
            d = load_domain(name)
        except DomainError as e:
            print(f"Domain config error in {name!r}: {e}", file=sys.stderr)
            continue
        domains.append(
            {
                "name": d["name"],
                "display_name": d["display_name"],
                "description": d.get("description", ""),
            }
        )
    print(json.dumps(domains, indent=2))
    return 0


def cmd_show(name: str) -> int:
    try:
        d = load_domain(name)
    except DomainError as e:
        print(f"Domain config error: {e}")
        return 1
    print(json.dumps(d, indent=2))
    return 0


def main(argv: list) -> int:
    if len(argv) < 2:
        print(__doc__)
        return 1

    command = argv[1]
    if command == "list":
        return cmd_list()
    if command == "show":
        if len(argv) < 3:
            print("show requires a domain name")
            return 1
        return cmd_show(argv[2])

    print(f"Unknown command {command!r}")
    print(__doc__)
    return 1


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
