# redliner (Go port, v1)

Dual-mode Go port of the CLI (`bin/redliner_*.py`, `bin/validate_findings.py`)
and MCP server (`cowork/mcp_server.py`) front doors, converging into one
compiled binary. Full rationale, phased plan, and known porting hazards
(CRLF hashing, JSON key order, MCP tool-description parity, etc.) are in
the repo root `TODO.md`, "Port to a compiled language for distributable
binaries?" section — read that before working here, this README doesn't
repeat it.

**Status: Phase 4 done (scaffold, schemas, CLI, MCP server all ported
and verified against real Python golden data).** Not yet done:
cross-compiled/committed binaries (Phase 6) and the full marketplace
install + live Cowork gate (Phase 5) — see `harness/README.md`'s "MCP
front door" section for exactly what is and isn't covered by the
automated suite today.

## Layout

- `cmd/redliner/` — the binary's `main()`. Dispatches `state`/`canon`/
  `domain`/`validate` into `internal/cli.Dispatch`, and `mcp` into
  `internal/mcpserver.NewServer` directly (mcpserver imports cli, so
  `mcp` can't be handled inside `cli.Dispatch` without a cycle — see
  `main.go`'s header comment).
- `internal/schemas/` — Go port of `bin/schemas/*.py` (`project_state`,
  `domain_loader`, `canon_schema`, `findings_schema`). Also implements
  `FindDomainsDir`'s explicit search contract (env override, else search
  near the binary), which is stricter than Python's original fixed-depth
  assumption — see the package's own doc comments for why.
- `internal/cli/` — Go port of `bin/redliner_{state,canon,domain}.py`
  and `bin/validate_findings.py`, consolidated into subcommands of one
  binary rather than four script names. `ComputeStale`, `ComputeReconcile`
  / `WriteCanonFiles`, and `ValidateManuscript` are exported specifically
  for `internal/mcpserver` to call directly, mirroring how
  `cowork/mcp_server.py` imports straight into `schemas/` and reuses
  `redliner_canon.py`/`validate_findings.py`'s functions rather than
  re-deriving the same logic.
- `internal/mcpserver/` — Go port of `cowork/mcp_server.py`: the same 10
  tools, same names, descriptions copied verbatim (`descriptions.go`,
  checked against Python's real docstrings by
  `TestToolNamesAndDescriptions_MatchPython`, not against a second copy
  of the same Go constants).
- `harness/` — the differential-testing harness every phase since Phase
  1 is checked against. See `harness/README.md`.

`go build ./cmd/redliner` produces one binary that's both the CLI and
(via `redliner mcp`) the MCP stdio server. It gets copied (not symlinked
— Cowork rejects `bin/` on PATH, and the plugin cache dereferences
symlinks into real files anyway) to both `../bin/redliner` and
`../cowork/redliner` once Phase 6 decides which platforms to commit
prebuilt binaries for; neither exists yet.

**Toolchain note:** `go.mod` requires Go 1.25+ (bumped from Phase 1's
1.22 when `github.com/modelcontextprotocol/go-sdk` was added in Phase
4 — the SDK's own `go.mod` requires 1.25, this isn't an accidental
toolchain drift).

## Building

```
cd go
go build ./...
go vet ./...
go test ./...
```
