// Command redliner is the v1 dual-mode binary: a CLI when invoked with a
// subcommand, and (once Phase 4 lands) an MCP stdio server when invoked
// as `redliner mcp`. See TODO.md's "Port to a compiled language for
// distributable binaries?" section for the full phased plan this
// scaffold is Phase 1 of.
//
// Phase 1 has no ported logic yet -- this file exists to prove the
// module builds and to give later phases a single place to wire
// subcommand dispatch into, not to do real work. Real subcommands
// (state, canon, domain, validate, mcp) land in Phase 2/3/4 against the
// golden baselines captured in harness/golden/.
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "redliner: no ported subcommands yet (Phase 1 scaffold) -- see TODO.md")
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "redliner: subcommand %q not implemented yet (Phase 1 scaffold)\n", os.Args[1])
	os.Exit(1)
}
