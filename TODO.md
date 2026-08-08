# Open questions / deferred work

Design questions raised during development that we deliberately parked.
Not a task tracker — the reasoning matters more than the checkbox.

## Port to a compiled language for distributable binaries?

**Raised:** 2026-08-08

The deterministic pieces are all Python (`edaitor_state.py`,
`edaitor_canon.py`, `validate_findings.py`, `schemas/`), which assumes a
working Python on the machine. Claude Code itself will be present — but
possibly as the desktop app rather than a terminal with a dev toolchain,
and a novelist using this is much less likely to have Python set up than
a developer is.

Worth considering: port the deterministic layer to Go or Rust and ship
prebuilt binaries, so the only prerequisite is Claude Code itself.

Points in favor:
- The scripts are pure stdlib, no dependencies — a genuinely small port.
- They're deterministic by design (hashing, diffing, collision-finding),
  which is exactly the kind of code that ports cleanly and benefits from
  being a single self-contained artifact.
- The audience for a fiction-editing tool skews non-technical.
- Go in particular: trivial cross-compilation, single static binary, no
  runtime.

Points against / to think about:
- The **agent definitions and skills are markdown** and don't port at
  all — a binary only replaces maybe a third of the system. The
  `.claude/` directory still has to be installed somehow, so "just
  download one binary" isn't actually achievable.
- Python-on-macOS/Linux is usually present; the real gap is Windows.
- Contributors editing schema vocabulary (categories, severities) would
  need a toolchain to rebuild, where today they edit a file. That's a
  real cost for a project whose vocabulary is still moving.
- Premature: the schemas are still changing shape (we've revised them
  twice already this session).

**Rough recommendation when we return to this:** revisit once the schema
vocabulary stops changing. If it's still worth doing then, Go, and treat
the binary as an optimization of the install story rather than a rewrite
— keep the markdown agents as the real product.

## Obsidian vault integration

**Raised:** 2026-08-08

The author keeps worldbuilding notes as a markdown wiki in Obsidian.
Deliberately deferred until the internal continuity layer works.

The valuable direction is reading the vault as a **second source of
canon** — because vault-vs-manuscript contradictions are their own error
class, and arguably the more useful one: "your wiki says Selkirk was
founded 300 years ago; chapter 4 says 500." That's intent-vs-text, which
neither layer catches alone.

Constraints decided up front:
- **Read-only, always.** The vault is the author's creative work.
  edaitor never writes to it.
- Don't design the fact schema around an imagined Obsidian frontmatter
  format. Read actual notes from the real vault first, then add an
  `origin` field distinguishing manuscript-derived from vault-derived
  facts.
