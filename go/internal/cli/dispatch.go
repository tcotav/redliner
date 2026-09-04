package cli

import (
	"fmt"
	"io"

	"github.com/tcotav/redliner/go/internal/buildinfo"
)

const usage = `Usage:
  redliner state init|status|diff|snapshot <manuscript_dir> [domain]
  redliner state phase <manuscript_dir> <phase>
  redliner canon stale|reconcile <manuscript_dir>
  redliner outline stale|join|render|archive|versions <manuscript_dir>   # scene-level view of the plot
  redliner domain list
  redliner domain show <name>
  redliner validate <manuscript_dir>
  redliner context <manuscript_dir>   # state+domain+sections+diff+canon in one call
  redliner decisions list|apply <manuscript_dir>   # author resolve/wontfix decisions
  redliner rounds archive|list <manuscript_dir> [pass]   # keep completed passes for diffing
  redliner version   # the release this binary was built from, or "dev"
  redliner mcp   # MCP stdio server (see cmd/redliner/main.go)`

// Dispatch is the binary's top-level subcommand router for every
// subcommand *except* "mcp", shared by cmd/redliner/main.go and this
// package's own tests. "mcp" is handled by main.go directly, not here:
// internal/mcpserver imports internal/cli (mirroring
// mcp_server.py importing straight into schemas/ and reusing
// redliner_canon.py's/validate_findings.py's functions), so this
// package cannot import internal/mcpserver back without a cycle.
func Dispatch(args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stdout, usage)
		return 1
	}
	switch args[0] {
	case "state":
		return RunState(args[1:], stdout)
	case "canon":
		return RunCanon(args[1:], stdout, stderr)
	case "outline":
		return RunOutline(args[1:], stdout, stderr)
	case "domain":
		return RunDomain(args[1:], stdout, stderr)
	case "validate":
		return RunValidate(args[1:], stdout)
	case "context":
		return RunContext(args[1:], stdout)
	case "decisions":
		return RunDecisions(args[1:], stdout)
	case "rounds":
		return RunRounds(args[1:], stdout)
	case "version", "--version", "-v":
		// The flag spellings are accepted alongside the subcommand
		// because they are what anyone actually types first -- and
		// through 0.7.0 `redliner --version` answered "Unknown
		// subcommand '--version'", which reads like the binary is
		// broken rather than like the flag is spelled differently.
		fmt.Fprintln(stdout, buildinfo.Version)
		return 0
	default:
		fmt.Fprintf(stdout, "Unknown subcommand %s\n\n%s\n", pyReprStr(args[0]), usage)
		return 1
	}
}
