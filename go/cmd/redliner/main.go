// Command redliner is the v1 dual-mode binary: a CLI when invoked with a
// subcommand, and an MCP stdio server when invoked as `redliner mcp`.
// See TODO.md's "Port to a compiled language for distributable
// binaries?" section for the full phased plan.
//
// All CLI subcommand logic lives in internal/cli, tested there directly
// against the golden baselines in harness/golden/. The MCP server lives
// in internal/mcpserver, which imports internal/cli -- this file is the
// one place that imports both, so "mcp" is dispatched here rather than
// inside internal/cli.Dispatch (which would need to import
// internal/mcpserver back, a cycle).
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/tcotav/redliner/go/internal/cli"
	"github.com/tcotav/redliner/go/internal/mcpserver"
	"github.com/tcotav/redliner/go/internal/schemas"
)

func main() {
	if len(os.Args) >= 2 && os.Args[1] == "mcp" {
		os.Exit(runMCP())
	}
	os.Exit(cli.Dispatch(os.Args[1:], os.Stdout, os.Stderr))
}

func runMCP() int {
	domainsDir, err := schemas.FindDomainsDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "redliner mcp: %v\n", err)
		return 1
	}
	server := mcpserver.NewServer(domainsDir)
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		fmt.Fprintf(os.Stderr, "redliner mcp: %v\n", err)
		return 1
	}
	return 0
}
