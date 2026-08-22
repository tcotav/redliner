package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/tcotav/redliner/go/internal/schemas"
)

const stateUsage = `Usage:
  redliner state init <manuscript_dir> [domain]   # domain defaults to "fiction"
  redliner state status   <manuscript_dir>
  redliner state diff     <manuscript_dir>
  redliner state snapshot <manuscript_dir>            # record current text as assessed
  redliner state phase    <manuscript_dir> <phase>
  redliner state stage    <manuscript_dir> <draft_stage>   # gates severity; see the domain's draft_stages
  redliner state pass     <manuscript_dir> <developmental|line|continuity>
  redliner state published <manuscript_dir> <section_stem|none>`

// RunState mirrors redliner_state.py's main(), reshaped for the
// `redliner state <subcommand> <dir> ...` argv layout decided in
// TODO.md's "v1 plan" (one binary, subcommands, not four script-name
// symlinks).
func RunState(args []string, stdout io.Writer) int {
	if len(args) < 2 {
		fmt.Fprintln(stdout, stateUsage)
		return 1
	}
	command, manuscriptDir := args[0], args[1]
	info, err := os.Stat(manuscriptDir)
	if err != nil || !info.IsDir() {
		fmt.Fprintf(stdout, "No such directory: %s\n", manuscriptDir)
		return 1
	}

	switch command {
	case "init":
		domain := schemas.DefaultDomain
		if len(args) > 2 {
			domain = args[2]
		}
		return cmdStateInit(manuscriptDir, domain, stdout)
	case "status":
		return cmdStateStatus(manuscriptDir, stdout)
	case "diff":
		return cmdStateDiff(manuscriptDir, stdout)
	case "snapshot":
		return cmdStateSnapshot(manuscriptDir, stdout)
	case "phase":
		if len(args) < 3 {
			fmt.Fprintln(stdout, "phase requires a target phase")
			return 1
		}
		return cmdStatePhase(manuscriptDir, args[2], stdout)
	case "stage":
		if len(args) < 3 {
			fmt.Fprintln(stdout, "stage requires a draft stage")
			return 1
		}
		return cmdStateStage(manuscriptDir, args[2], stdout)
	case "pass":
		if len(args) < 3 {
			fmt.Fprintln(stdout, "pass requires a phase")
			return 1
		}
		return cmdStatePass(manuscriptDir, args[2], stdout)
	case "published":
		if len(args) < 3 {
			fmt.Fprintln(stdout, stateUsage)
			return 1
		}
		return cmdStatePublished(manuscriptDir, args[2], stdout)
	default:
		fmt.Fprintf(stdout, "Unknown command %s\n", pyReprStr(command))
		fmt.Fprintln(stdout, stateUsage)
		return 1
	}
}

// requireState prints the "no state yet" message and returns ok=false
// if none exists -- mirrors redliner_state.py's _require_state. The
// message itself names the new `redliner state init` invocation, not
// the old `redliner_state.py init` -- an intentional, documented
// divergence from Python's exact text since the CLI surface changed;
// see go/harness/README.md's note on the CLI-shape decision.
func requireState(manuscriptDir string, stdout io.Writer) (*schemas.State, bool) {
	state, err := schemas.LoadState(manuscriptDir)
	if err != nil {
		fmt.Fprintf(stdout, "Error reading state: %v\n", err)
		return nil, false
	}
	if state == nil {
		fmt.Fprintf(stdout, "No redliner state in %s. Run: redliner state init %s\n", manuscriptDir, manuscriptDir)
		return nil, false
	}
	return state, true
}

func reportSectionError(err error, stdout io.Writer) int {
	if _, ok := err.(*schemas.SectionCollisionError); ok {
		fmt.Fprintf(stdout, "Section file error: %s\n", err.Error())
		return 1
	}
	fmt.Fprintf(stdout, "Error: %v\n", err)
	return 1
}

func cmdStateInit(manuscriptDir, domain string, stdout io.Writer) int {
	if existing, err := schemas.LoadState(manuscriptDir); err == nil && existing != nil {
		fmt.Fprintf(stdout, "State already exists at %s\n", schemas.StatePath(manuscriptDir))
		return 1
	}

	domainsDir, err := schemas.FindDomainsDir()
	if err != nil {
		fmt.Fprintf(stdout, "Domain config error: %v\n", err)
		return 1
	}
	// Fail fast on a typo'd/missing domain, not at first use -- mirrors
	// cmd_init's load_domain() call whose only purpose is this check.
	if _, err := schemas.LoadDomain(domainsDir, domain); err != nil {
		fmt.Fprintf(stdout, "Domain config error: %v\n", err)
		return 1
	}

	state := schemas.NewState(manuscriptDir, domain)
	path, err := schemas.SaveState(manuscriptDir, state)
	if err != nil {
		fmt.Fprintf(stdout, "Error saving state: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Initialized %s (domain: %s, phase: intake)\n", path, domain)
	return 0
}

func cmdStateStatus(manuscriptDir string, stdout io.Writer) int {
	state, ok := requireState(manuscriptDir, stdout)
	if !ok {
		return 1
	}
	return printJSON(stdout, state)
}

func cmdStateDiff(manuscriptDir string, stdout io.Writer) int {
	state, ok := requireState(manuscriptDir, stdout)
	if !ok {
		return 1
	}
	diff, err := schemas.DiffManuscript(manuscriptDir, state)
	if err != nil {
		return reportSectionError(err, stdout)
	}
	return printJSON(stdout, diff)
}

func cmdStateSnapshot(manuscriptDir string, stdout io.Writer) int {
	state, ok := requireState(manuscriptDir, stdout)
	if !ok {
		return 1
	}
	fingerprints, err := schemas.FingerprintManuscript(manuscriptDir)
	if err != nil {
		return reportSectionError(err, stdout)
	}
	state.SectionFingerprints = fingerprints
	if _, err := schemas.SaveState(manuscriptDir, state); err != nil {
		fmt.Fprintf(stdout, "Error saving state: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Snapshotted %d sections as assessed.\n", len(fingerprints))
	return 0
}

func cmdStatePhase(manuscriptDir, phase string, stdout io.Writer) int {
	if !schemas.IsValidPhase(phase) {
		fmt.Fprintf(stdout, "Unknown phase %s. Must be one of: %s\n", pyReprStr(phase), strings.Join(schemas.Phases, ", "))
		return 1
	}
	state, ok := requireState(manuscriptDir, stdout)
	if !ok {
		return 1
	}

	domainsDir, err := schemas.FindDomainsDir()
	if err != nil {
		fmt.Fprintf(stdout, "Domain config error: %v\n", err)
		return 1
	}
	domain, err := schemas.LoadDomain(domainsDir, state.DomainName())
	if err != nil {
		fmt.Fprintf(stdout, "Domain config error: %v\n", err)
		return 1
	}
	roundTrackedPhase := domain.RoundTrackedPhase()

	previous := state.Phase
	state.Phase = phase
	// Entering the domain's round-tracked phase (fiction: "developmental")
	// from anywhere else starts a new round -- mirrors cmd_phase exactly,
	// including reading the phase name from domain config rather than a
	// hardcoded string.
	if phase == roundTrackedPhase && previous != roundTrackedPhase {
		state.DevelopmentalRound++
	}
	if _, err := schemas.SaveState(manuscriptDir, state); err != nil {
		fmt.Fprintf(stdout, "Error saving state: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Phase: %s -> %s (developmental_round: %d)\n", previous, phase, state.DevelopmentalRound)
	return 0
}

// passKinds are the passes an author can actually run. Deliberately
// distinct from schemas.Phases -- see cmdStatePass for why.
var passKinds = []string{"developmental", "line", "continuity"}

// cmdStateStage sets the manuscript's draft stage, validated against its
// domain's own `draft_stages` vocabulary. See State.DraftStage for why
// this needs to be machine-readable rather than prose in the brief.
func cmdStateStage(manuscriptDir, stage string, stdout io.Writer) int {
	state, ok := requireState(manuscriptDir, stdout)
	if !ok {
		return 1
	}
	domainsDir, err := schemas.FindDomainsDir()
	if err != nil {
		fmt.Fprintf(stdout, "Domain config error: %v\n", err)
		return 1
	}
	domain, err := schemas.LoadDomain(domainsDir, state.DomainName())
	if err != nil {
		fmt.Fprintf(stdout, "Domain config error: %v\n", err)
		return 1
	}

	names := domain.DraftStageNames()
	valid := false
	for _, n := range names {
		if n == stage {
			valid = true
			break
		}
	}
	if !valid {
		fmt.Fprintf(stdout, "Unknown draft stage %s for domain %s. Must be one of: %s\n",
			pyReprStr(stage), pyReprStr(state.DomainName()), strings.Join(names, ", "))
		return 1
	}

	previous := state.DraftStage
	state.DraftStage = stage
	if _, err := schemas.SaveState(manuscriptDir, state); err != nil {
		fmt.Fprintf(stdout, "Error saving state: %v\n", err)
		return 1
	}
	if previous == "" {
		previous = "(unset)"
	}
	fmt.Fprintf(stdout, "Draft stage: %s -> %s\n", previous, stage)
	if impl := domain.DraftStageImplication(stage); impl != "" {
		fmt.Fprintf(stdout, "Severity implication: %s\n", impl)
	}
	return 0
}

// cmdStatePublished records which installments have shipped. Serial
// fiction has a constraint a novel does not -- once a chapter goes out
// the author does not revise it -- so a scene above this line cannot be
// moved or cut, which is the one fact the rendered outline most needs to
// show.
//
// Validated against the manuscript's real sections rather than accepting
// any string: a typo sets a boundary matching no section, which draws no
// line at all, and that reads as the feature being broken rather than the
// input being wrong.
func cmdStatePublished(manuscriptDir, stem string, stdout io.Writer) int {
	state, ok := requireState(manuscriptDir, stdout)
	if !ok {
		return 1
	}

	// "none" is how an author says nothing has shipped yet -- a serial
	// being drafted before launch, and the correct state for any novel.
	if stem == "none" {
		state.PublishedThrough = ""
		if _, err := schemas.SaveState(manuscriptDir, state); err != nil {
			fmt.Fprintf(stdout, "Error writing state: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, "Published boundary cleared -- nothing is marked as shipped.")
		return 0
	}

	paths, err := schemas.SectionFiles(manuscriptDir)
	if err != nil {
		return reportSectionError(err, stdout)
	}
	var stems []string
	found := false
	for _, path := range paths {
		s := stemOfPath(path)
		stems = append(stems, s)
		if s == stem {
			found = true
		}
	}
	if !found {
		fmt.Fprintf(stdout, "No section %s in %s. Sections are: %s\n",
			pyReprStr(stem), manuscriptDir, strings.Join(stems, ", "))
		return 1
	}

	state.PublishedThrough = stem
	if _, err := schemas.SaveState(manuscriptDir, state); err != nil {
		fmt.Fprintf(stdout, "Error writing state: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Published through %s. Everything up to and including it renders as shipped in Outline.md.\n", stem)
	return 0
}

// cmdStatePass records that a pass of the given kind completed, so
// status can report what has actually been run rather than only the
// current phase.
func cmdStatePass(manuscriptDir, kind string, stdout io.Writer) int {
	// Deliberately NOT validated against schemas.Phases. The pass kinds
	// and the phases are different sets: `continuity` is a real pass that
	// is explicitly not phase-gated (it tracks its own per-section
	// staleness), while `intake` and `complete` are phases nobody runs a
	// pass of. Validating against Phases rejected `continuity` and
	// accepted `intake`, both wrong.
	valid := false
	for _, k := range passKinds {
		if k == kind {
			valid = true
			break
		}
	}
	if !valid {
		fmt.Fprintf(stdout, "Unknown pass %s. Must be one of: %s\n", pyReprStr(kind), strings.Join(passKinds, ", "))
		return 1
	}
	state, ok := requireState(manuscriptDir, stdout)
	if !ok {
		return 1
	}
	if state.Passes == nil {
		state.Passes = map[string]int{}
	}
	state.Passes[kind]++
	if _, err := schemas.SaveState(manuscriptDir, state); err != nil {
		fmt.Fprintf(stdout, "Error saving state: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Recorded %s pass (total: %d)\n", kind, state.Passes[kind])
	return 0
}
