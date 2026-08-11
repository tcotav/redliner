# redliner (Go port, v1)

Dual-mode Go port of the CLI (`bin/redliner_*.py`, `bin/validate_findings.py`)
and MCP server (`cowork/mcp_server.py`) front doors, converging into one
compiled binary. Full rationale, phased plan, and known porting hazards
(CRLF hashing, JSON key order, MCP tool-description parity, etc.) are in
the repo root `TODO.md`, "Port to a compiled language for distributable
binaries?" section — read that before working here, this README doesn't
repeat it.

**Status: Phase 1 (scaffold + differential harness). No logic ported
yet.** `cmd/redliner/main.go` is a stub that proves the module builds;
`internal/schemas/` is an empty placeholder for Phase 2.

## Layout

- `cmd/redliner/` — the binary's `main()`. Subcommand dispatch
  (`state`/`canon`/`domain`/`validate`/`mcp`) lands here in later phases.
- `internal/schemas/` — Go port of `bin/schemas/*.py`
  (`project_state`, `domain_loader`, `canon_schema`, `findings_schema`).
  Empty until Phase 2.
- `harness/` — the differential-testing harness that every later phase
  is checked against. See `harness/README.md`.

Eventually `go build` produces one binary, copied (not symlinked — Cowork
rejects `bin/` on PATH, and the plugin cache dereferences symlinks into
real files anyway) to both `../bin/redliner` and `../cowork/redliner`.
Neither of those exists yet; that's Phase 6.

## Building

```
cd go
go build ./...
go vet ./...
```
