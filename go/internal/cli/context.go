package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/tcotav/redliner/go/internal/schemas"
)

// `redliner context` exists to cut coordinator round trips, not to add
// capability: every field below was already obtainable, just across four
// or five separate commands. Measured on a real assess run (see TODO.md,
// "Cost/latency optimization"), the coordinator spent 18 API calls, half
// of them Bash, re-reading ~50K tokens of context each time -- including
// `domain show <name>` run twice back to back. One command that answers
// "where does this manuscript stand and what vocabulary applies" removes
// those round trips, and each removed round trip is a full model latency
// cycle as well as the tokens.
//
// Go-only by design: this is a convenience over operations the Python
// baseline already had, not a ported operation, so it is deliberately
// absent from go/harness/python-baseline and from the golden fixtures.
// The oracle's contract covers the ported surface; adding a composite
// here does not weaken it.

// ContextReport is the single JSON payload `redliner context` prints.
// Field names match the existing per-command outputs so a reader who
// knows `state status` / `canon stale` recognizes them unchanged.
type ContextReport struct {
	ManuscriptDir string             `json:"manuscript_dir"`
	State         *schemas.State     `json:"state"`
	Domain        interface{}        `json:"domain"`
	Sections      []string           `json:"sections"`
	Diff          schemas.DiffResult `json:"diff"`
	Canon         StaleResult        `json:"canon"`
	Files         map[string]bool    `json:"files_present"`
}

const contextUsage = `Usage:
  redliner context <manuscript_dir>   # state + domain + sections + diff + canon staleness, in one call`

// BuildContext is RunContext's computation, pure (no writer, no
// printing) so internal/mcpserver's `context` tool reuses it unchanged --
// same split as ComputeStale, and for the same reason: both front doors
// must answer identically. domainsDir is a parameter rather than resolved
// internally so the MCP server passes the one every other tool uses.
func BuildContext(manuscriptDir, domainsDir string) (ContextReport, error) {
	state, err := schemas.LoadState(manuscriptDir)
	if err != nil {
		return ContextReport{}, err
	}
	domain, err := schemas.LoadDomain(domainsDir, state.DomainName())
	if err != nil {
		return ContextReport{}, err
	}
	sections, err := schemas.SectionFiles(manuscriptDir)
	if err != nil {
		return ContextReport{}, err
	}
	diff, err := schemas.DiffManuscript(manuscriptDir, state)
	if err != nil {
		return ContextReport{}, err
	}

	// Staleness needs observations; a manuscript with no continuity run
	// yet is a normal state, not an error -- report the zero value
	// rather than failing the whole call.
	stale, staleErr := ComputeStale(manuscriptDir)
	if staleErr != nil {
		stale = StaleResult{}
	}

	stateDir := schemas.StateDir(manuscriptDir)
	exists := func(rel string) bool {
		_, err := os.Stat(filepath.Join(stateDir, rel))
		return err == nil
	}

	return ContextReport{
		ManuscriptDir: manuscriptDir,
		State:         state,
		Domain:        domain,
		Sections:      sections,
		Diff:          diff,
		Canon:         stale,
		Files: map[string]bool{
			"brief.md":               exists("brief.md"),
			"findings/developmental": exists("findings/developmental.json"),
			"findings/line":          exists("findings/line.json"),
			"canon/canon.json":       exists("canon/canon.json"),
			"canon/collisions.json":  exists("canon/collisions.json"),
			"canon/continuity.json":  exists("canon/continuity.json"),
		},
	}, nil
}

// RunContext implements `redliner context <manuscript_dir>`.
func RunContext(args []string, stdout io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stdout, contextUsage)
		return 1
	}
	manuscriptDir := args[0]

	if _, ok := requireState(manuscriptDir, stdout); !ok {
		return 1
	}
	domainsDir, err := schemas.FindDomainsDir()
	if err != nil {
		fmt.Fprintf(stdout, "Domain config error: %v\n", err)
		return 1
	}
	report, err := BuildContext(manuscriptDir, domainsDir)
	if err != nil {
		if _, ok := err.(*schemas.SectionCollisionError); ok {
			return reportSectionError(err, stdout)
		}
		fmt.Fprintf(stdout, "Domain config error: %v\n", err)
		return 1
	}
	return printJSON(stdout, report)
}
