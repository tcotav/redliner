"""Validate everything in a findings/ directory against schemas/findings_schema.py.

Pure stdlib — no pip install needed, so the edaitor skill can shell out to
this between agent steps without adding any dependency to the project.

Usage:
    python3 validate_findings.py [findings_dir]

Exits 0 if everything present validates cleanly, 1 otherwise. Missing
files are not errors by themselves (the pipeline may not have reached
that step yet) — only files that exist and fail validation fail the run.
"""

import json
import sys
from pathlib import Path

from schemas.findings_schema import (
    validate_developmental_report,
    validate_editorial_letter,
    validate_line_report,
)


def _check(path: Path, errors: list) -> bool:
    if errors:
        print(f"FAIL {path}")
        for error in errors:
            print(f"  - {error}")
        return False
    print(f"OK   {path}")
    return True


def main(findings_dir: str) -> int:
    findings_path = Path(findings_dir)
    if not findings_path.exists():
        print(f"No such directory: {findings_path}")
        return 1

    ok = True

    dev_file = findings_path / "developmental.json"
    if dev_file.exists():
        errors = validate_developmental_report(json.loads(dev_file.read_text()))
        ok = _check(dev_file, errors) and ok

    line_files = sorted(findings_path.glob("line_*.json"))
    for line_file in line_files:
        errors = validate_line_report(json.loads(line_file.read_text()))
        ok = _check(line_file, errors) and ok

    letter_file = findings_path / "editorial_letter.json"
    if letter_file.exists():
        errors = validate_editorial_letter(json.loads(letter_file.read_text()))
        ok = _check(letter_file, errors) and ok

    if not dev_file.exists() and not line_files and not letter_file.exists():
        print(f"Nothing to validate yet in {findings_path}")

    return 0 if ok else 1


if __name__ == "__main__":
    target = sys.argv[1] if len(sys.argv) > 1 else "findings"
    raise SystemExit(main(target))
