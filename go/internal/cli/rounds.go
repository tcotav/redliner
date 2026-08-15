package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tcotav/redliner/go/internal/schemas"
)

// Completed passes are archived under .redliner/rounds/<pass>-round<N>/
// so a later round can be diffed against an earlier one.
//
// Without this the "before" simply doesn't exist: every pass rewrites
// findings in place, and `assess` explicitly clears stale findings from
// the previous round before writing new ones. Archiving at the end of a
// pass is what makes that clear step safe.
const roundsUsage = `Usage:
  redliner rounds archive <manuscript_dir> <developmental|line|continuity>
  redliner rounds list    <manuscript_dir>`

func roundsDir(manuscriptDir string) string {
	return filepath.Join(schemas.StateDir(manuscriptDir), "rounds")
}

// RunRounds implements `redliner rounds <subcommand> <dir> [pass]`.
func RunRounds(args []string, stdout io.Writer) int {
	if len(args) < 2 {
		fmt.Fprintln(stdout, roundsUsage)
		return 1
	}
	command, manuscriptDir := args[0], args[1]
	if info, err := os.Stat(manuscriptDir); err != nil || !info.IsDir() {
		fmt.Fprintf(stdout, "No such directory: %s\n", manuscriptDir)
		return 1
	}

	switch command {
	case "archive":
		if len(args) < 3 {
			fmt.Fprintln(stdout, "archive requires a pass name")
			return 1
		}
		return cmdRoundsArchive(manuscriptDir, args[2], stdout)
	case "list":
		return cmdRoundsList(manuscriptDir, stdout)
	default:
		fmt.Fprintf(stdout, "Unknown command %s\n", pyReprStr(command))
		fmt.Fprintln(stdout, roundsUsage)
		return 1
	}
}

// freeArchiveDir picks a destination that doesn't already hold an
// archive, appending `.2`, `.3`, ... to the round-numbered name.
//
// The plain `<pass>-round<N>` name is not unique per archive, and
// assuming it was cost real data: the round counter only advances on
// entering the developmental phase, while continuity is explicitly not
// phase-gated (see cmdStatePass) and a line pass can legitimately run
// twice in one round. Both archives resolved to one directory and the
// second overwrote the first -- silently, reporting success, destroying
// the "before" this command exists to keep.
//
// Suffixing rather than refusing: the archive is a side effect of
// finishing a pass, so failing it would fail the pass for a reason the
// author has no way to act on, and refusing would leave the newer
// findings unarchived. Keeping both, in round order, loses nothing.
func freeArchiveDir(roundsRoot, pass string, round int) (string, error) {
	base := filepath.Join(roundsRoot, fmt.Sprintf("%s-round%d", pass, round))
	if _, err := os.Stat(base); os.IsNotExist(err) {
		return base, nil
	}
	// Bounded so a bug upstream can't spin here. 99 archives of one pass
	// in one round is not a real workflow.
	for n := 2; n <= 99; n++ {
		candidate := fmt.Sprintf("%s.%d", base, n)
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("%s already holds 99 archives for this round", base)
}

func cmdRoundsArchive(manuscriptDir, pass string, stdout io.Writer) int {
	valid := false
	for _, k := range passKinds {
		if k == pass {
			valid = true
		}
	}
	if !valid {
		fmt.Fprintf(stdout, "Unknown pass %s. Must be one of: %s\n", pyReprStr(pass), strings.Join(passKinds, ", "))
		return 1
	}
	state, ok := requireState(manuscriptDir, stdout)
	if !ok {
		return 1
	}

	// Which files belong to this pass. Continuity's artifacts live under
	// canon/, not findings/, so it archives a different set.
	var sources []string
	stateDir := schemas.StateDir(manuscriptDir)
	switch pass {
	case "developmental":
		sources, _ = filepath.Glob(filepath.Join(stateDir, "findings", "developmental.json"))
	case "line":
		sources, _ = filepath.Glob(filepath.Join(stateDir, "findings", "line_*.json"))
	case "continuity":
		sources, _ = filepath.Glob(filepath.Join(stateDir, "canon", "continuity.json"))
		more, _ := filepath.Glob(filepath.Join(stateDir, "canon", "collisions.json"))
		sources = append(sources, more...)
	}
	if len(sources) == 0 {
		fmt.Fprintf(stdout, "Nothing to archive for the %s pass yet.\n", pass)
		return 0
	}

	dest, err := freeArchiveDir(roundsDir(manuscriptDir), pass, state.DevelopmentalRound)
	if err != nil {
		fmt.Fprintf(stdout, "Error choosing an archive directory: %v\n", err)
		return 1
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		fmt.Fprintf(stdout, "Error creating %s: %v\n", dest, err)
		return 1
	}

	sort.Strings(sources)
	for _, src := range sources {
		data, err := os.ReadFile(src)
		if err != nil {
			fmt.Fprintf(stdout, "Error reading %s: %v\n", src, err)
			return 1
		}
		if err := os.WriteFile(filepath.Join(dest, filepath.Base(src)), data, 0o644); err != nil {
			fmt.Fprintf(stdout, "Error writing archive: %v\n", err)
			return 1
		}
	}
	fmt.Fprintf(stdout, "Archived %d file(s) to %s\n", len(sources), dest)
	return 0
}

func cmdRoundsList(manuscriptDir string, stdout io.Writer) int {
	entries, err := os.ReadDir(roundsDir(manuscriptDir))
	if err != nil {
		fmt.Fprintln(stdout, "No archived rounds yet.")
		return 0
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	if len(names) == 0 {
		fmt.Fprintln(stdout, "No archived rounds yet.")
		return 0
	}
	sort.Strings(names)
	fmt.Fprintf(stdout, "Archived rounds (%d):\n", len(names))
	for _, n := range names {
		files, _ := os.ReadDir(filepath.Join(roundsDir(manuscriptDir), n))
		fmt.Fprintf(stdout, "  %-26s %d file(s)\n", n, len(files))
	}
	return 0
}
