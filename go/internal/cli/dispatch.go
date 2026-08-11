package cli

import (
	"fmt"
	"io"
)

const usage = `Usage:
  redliner state init|status|diff|snapshot <manuscript_dir> [domain]
  redliner state phase <manuscript_dir> <phase>
  redliner canon stale|reconcile <manuscript_dir>
  redliner domain list
  redliner domain show <name>
  redliner validate <manuscript_dir>
  redliner mcp   # not implemented yet -- Phase 4`

// Dispatch is the binary's top-level subcommand router, shared by
// cmd/redliner/main.go and this package's own tests (so tests exercise
// the exact same routing main() uses, not a reimplementation of it).
func Dispatch(args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stdout, usage)
		return 1
	}
	switch args[0] {
	case "state":
		return RunState(args[1:], stdout)
	case "canon":
		return RunCanon(args[1:], stdout)
	case "domain":
		return RunDomain(args[1:], stdout, stderr)
	case "validate":
		return RunValidate(args[1:], stdout)
	case "mcp":
		fmt.Fprintln(stdout, "redliner mcp: not implemented yet (Phase 4)")
		return 1
	default:
		fmt.Fprintf(stdout, "Unknown subcommand %s\n\n%s\n", pyReprStr(args[0]), usage)
		return 1
	}
}
