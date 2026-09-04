// Package buildinfo holds the one version string the binary knows about
// itself.
//
// It exists because for every release through 0.7.0 the binary could not
// answer the question "what version are you?" at all. `redliner
// --version` was an unknown subcommand, and the MCP server introduced
// itself as `Version: "0.1.0"` -- a literal that had been hardcoded in
// mcpserver.NewServer since the port and never moved. The only version
// signal anywhere was `bin/redliner.version`, a sidecar file written by
// hooks/bootstrap-redliner-binary.sh, which records what the *installer
// meant to fetch* rather than what the file on disk actually is. That
// distinction stopped being academic on 2026-09-04, when the v0.7.0
// install gate had to fall back to comparing a SHA-256 against the
// release's checksums.txt to establish provenance.
//
// This package is its own leaf so both cmd/redliner (the `version`
// subcommand) and internal/mcpserver (the serverInfo it reports at
// initialize) can read the same variable. internal/mcpserver imports
// internal/cli, so anything both need has to live below both.
//
// Version is injected at link time by .github/workflows/
// release-go-binaries.yml:
//
//	go build -ldflags "-X github.com/tcotav/redliner/go/internal/buildinfo.Version=${VERSION}"
//
// A plain `go build` leaves it at "dev", which is the honest answer for
// a locally-built binary: it says "this did not come from a release"
// rather than claiming a version number it cannot support.
package buildinfo

// Version is the release this binary was built from, or "dev" for a
// local build. Set via -ldflags; do not assign to it at runtime.
var Version = "dev"
