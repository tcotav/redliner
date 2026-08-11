// Command redliner is the v1 dual-mode binary: a CLI when invoked with a
// subcommand, and (once Phase 4 lands) an MCP stdio server when invoked
// as `redliner mcp`. See TODO.md's "Port to a compiled language for
// distributable binaries?" section for the full phased plan.
//
// All subcommand logic lives in internal/cli, tested there directly
// against the golden baselines in harness/golden/ -- this file is just
// argv/exit-code plumbing.
package main

import (
	"os"

	"github.com/tcotav/redliner/go/internal/cli"
)

func main() {
	os.Exit(cli.Dispatch(os.Args[1:], os.Stdout, os.Stderr))
}
