// Package schemas is the Go port of bin/schemas/*.py: per-manuscript
// state, domain config loading, and the findings/canon JSON validators.
// See the repo root TODO.md's "Port to a compiled language" section for
// the full phased plan and the porting hazards (CRLF hashing, JSON key
// order, timestamp format, MCP tool-description parity) this package is
// written against, and go/harness/ for the golden baselines it's
// verified against.
package schemas

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Phases a manuscript moves through. Order matches project_state.py's
// PHASES tuple (not load-bearing itself, but kept identical for anyone
// reading both side by side).
var Phases = []string{"intake", "developmental", "line", "complete"}

// IsValidPhase mirrors the `phase not in PHASES` check in
// redliner_state.py's cmd_phase / mcp_server.py's state_phase.
func IsValidPhase(phase string) bool {
	for _, p := range Phases {
		if p == phase {
			return true
		}
	}
	return false
}

const (
	StateDirname  = ".redliner"
	StateFilename = "state.json"
	DefaultDomain = "fiction"

	// A section whose word count moves by more than this fraction is
	// treated as rewritten rather than tweaked, forcing a full re-read
	// on the next diff. See project_state.py's identical constant for
	// the reasoning -- this is a tuning knob, not a translation detail.
	MajorWordcountDelta = 0.25
)

// SectionExtensions -- a manuscript's sections may be .txt or .md, never
// mixed for the same stem (see SectionCollisionError).
var SectionExtensions = []string{".txt", ".md"}

// SectionCollisionError mirrors project_state.py's SectionCollisionError:
// a section stem exists under more than one supported extension.
type SectionCollisionError struct {
	Stems []string
}

func (e *SectionCollisionError) Error() string {
	return fmt.Sprintf(
		"Ambiguous section files (same stem under more than one extension): %s. Each section must exist as exactly one of %s.",
		strings.Join(e.Stems, ", "),
		strings.Join(SectionExtensions, " or "),
	)
}

func StateDir(manuscriptDir string) string {
	return filepath.Join(manuscriptDir, StateDirname)
}

func StatePath(manuscriptDir string) string {
	return filepath.Join(StateDir(manuscriptDir), StateFilename)
}

// SectionFiles returns every section_*.txt/.md file in manuscriptDir,
// one path per stem, sorted by stem. A stem present under more than one
// extension is a SectionCollisionError, not a silent pick of one --
// mirrors project_state.py's section_files exactly.
func SectionFiles(manuscriptDir string) ([]string, error) {
	byStem := map[string]string{}
	collisions := map[string]bool{}
	for _, ext := range SectionExtensions {
		matches, err := filepath.Glob(filepath.Join(manuscriptDir, "section_*"+ext))
		if err != nil {
			return nil, err
		}
		for _, path := range matches {
			stem := stemOf(path)
			if _, exists := byStem[stem]; exists {
				collisions[stem] = true
			}
			byStem[stem] = path
		}
	}
	if len(collisions) > 0 {
		stems := make([]string, 0, len(collisions))
		for s := range collisions {
			stems = append(stems, s)
		}
		sort.Strings(stems)
		return nil, &SectionCollisionError{Stems: stems}
	}

	stems := make([]string, 0, len(byStem))
	for s := range byStem {
		stems = append(stems, s)
	}
	sort.Strings(stems)
	paths := make([]string, len(stems))
	for i, s := range stems {
		paths[i] = byStem[s]
	}
	return paths, nil
}

func stemOf(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// NormalizeNewlines converts CRLF and lone CR to LF, matching Python's
// Path.read_text() universal-newline translation. Go's os.ReadFile does
// no such translation -- without this, a CRLF-authored manuscript (a
// real scenario for this project's non-technical, plausibly-Windows
// audience) hashes differently under Go than under Python, and every
// section would false-flag as "changed" on the first diff after
// cutover. Verified against go/harness/fixtures/crlf's golden hash, not
// just reasoned about -- see project_state_test.go.
func NormalizeNewlines(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	return text
}

// Fingerprint is one section's content hash and word count, keyed by
// section stem in Manuscript-level maps. Field order matches
// project_state.py's fingerprint_section dict.
type Fingerprint struct {
	SHA256 string `json:"sha256"`
	Words  int    `json:"words"`
}

func FingerprintSection(path string) (Fingerprint, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Fingerprint{}, err
	}
	text := NormalizeNewlines(string(raw))
	sum := sha256.Sum256([]byte(text))
	return Fingerprint{
		SHA256: hex.EncodeToString(sum[:]),
		Words:  len(strings.Fields(text)),
	}, nil
}

func FingerprintManuscript(manuscriptDir string) (map[string]Fingerprint, error) {
	paths, err := SectionFiles(manuscriptDir)
	if err != nil {
		return nil, err
	}
	out := make(map[string]Fingerprint, len(paths))
	for _, path := range paths {
		fp, err := FingerprintSection(path)
		if err != nil {
			return nil, err
		}
		out[stemOf(path)] = fp
	}
	return out, nil
}

// State is a manuscript's `.redliner/state.json`. Struct fields (not a
// generic map) because this file's shape is entirely owned by redliner
// itself -- unlike domain.json, nothing external needs round-tripped
// unknown fields. See TODO.md's JSON-key-order hazard: Go's field order
// here is intentional, not an attempt to byte-match Python's dict
// insertion order, which nothing but redliner itself ever reads anyway.
type State struct {
	ManuscriptDir       string                 `json:"manuscript_dir"`
	Domain              string                 `json:"domain"`
	Phase               string                 `json:"phase"`
	DevelopmentalRound  int                    `json:"developmental_round"`
	SectionFingerprints map[string]Fingerprint `json:"section_fingerprints"`
	CreatedAt           string                 `json:"created_at"`
	UpdatedAt           string                 `json:"updated_at,omitempty"`

	// DraftStage is the manuscript's stage within its domain's
	// `draft_stages` vocabulary. It gates severity harder than anything
	// else in the tool -- at fiction's "exploratory / partial" the line
	// editors correctly return nothing at all -- and until 2026-08-14 it
	// lived only as prose inside brief.md, so no command could report it
	// and no gate could check it. An author could therefore pay N model
	// calls for a line pass that was always going to be empty, with
	// nothing explaining why. The brief still carries the human
	// explanation; this carries the machine-readable value.
	//
	// Optional: omitted when unset, so state written before this existed
	// stays valid and the harness fixtures' goldens don't move.
	DraftStage string `json:"draft_stage,omitempty"`

	// Passes counts completed passes by kind ("developmental", "line",
	// "continuity"), so status can answer "what have we actually run, and
	// how many times" rather than only "what phase are we in". Distinct
	// from DevelopmentalRound, which counts *rounds entered*, not passes
	// completed -- a round that was started and abandoned still bumps the
	// round counter.
	//
	// Optional, same reasoning as DraftStage.
	Passes map[string]int `json:"passes,omitempty"`
}

// DomainName defaults an empty/absent domain to DefaultDomain, mirroring
// the `state.get("domain", DEFAULT_DOMAIN)` pattern repeated across
// redliner_state.py, validate_findings.py, and mcp_server.py.
func (s *State) DomainName() string {
	if s.Domain == "" {
		return DefaultDomain
	}
	return s.Domain
}

// LoadState returns (nil, nil) if no state file exists yet -- mirrors
// project_state.py's load_state returning None, not raising.
func LoadState(manuscriptDir string) (*State, error) {
	raw, err := os.ReadFile(StatePath(manuscriptDir))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var state State
	if err := json.Unmarshal(raw, &state); err != nil {
		return nil, err
	}
	if state.SectionFingerprints == nil {
		state.SectionFingerprints = map[string]Fingerprint{}
	}
	return &state, nil
}

func SaveState(manuscriptDir string, state *State) (string, error) {
	if err := os.MkdirAll(StateDir(manuscriptDir), 0o755); err != nil {
		return "", err
	}
	state.UpdatedAt = nowISO()
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return "", err
	}
	path := StatePath(manuscriptDir)
	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func NewState(manuscriptDir, domain string) *State {
	if domain == "" {
		domain = DefaultDomain
	}
	return &State{
		ManuscriptDir:       manuscriptDir,
		Domain:              domain,
		Phase:               "intake",
		DevelopmentalRound:  0,
		SectionFingerprints: map[string]Fingerprint{},
		CreatedAt:           nowISO(),
	}
}

// nowISO deliberately doesn't try to byte-match Python's
// datetime.now(timezone.utc).isoformat() (microseconds, "+00:00") --
// nothing but redliner itself reads these timestamps, and the harness
// strips them before any comparison (see harness/normalize.py). RFC3339
// with nanosecond precision is the Go-idiomatic equivalent.
func nowISO() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

// DiffResult is diff_manuscript's returned verdict. See
// project_state.py's docstring for what each verdict value means.
type DiffResult struct {
	Verdict    string   `json:"verdict"`
	Added      []string `json:"added"`
	Removed    []string `json:"removed"`
	Changed    []string `json:"changed"`
	LargeDelta []string `json:"large_delta"`
}

func DiffManuscript(manuscriptDir string, state *State) (DiffResult, error) {
	previous := state.SectionFingerprints
	if previous == nil {
		previous = map[string]Fingerprint{}
	}
	current, err := FingerprintManuscript(manuscriptDir)
	if err != nil {
		return DiffResult{}, err
	}

	added := keysOnlyIn(current, previous)
	removed := keysOnlyIn(previous, current)

	var changed, largeDelta []string
	for _, name := range sortedIntersection(current, previous) {
		if current[name].SHA256 == previous[name].SHA256 {
			continue
		}
		changed = append(changed, name)

		before := previous[name].Words
		after := current[name].Words
		if before == 0 {
			largeDelta = append(largeDelta, name)
			continue
		}
		if float64(absInt(after-before))/float64(before) > MajorWordcountDelta {
			largeDelta = append(largeDelta, name)
		}
	}

	verdict := "unchanged"
	switch {
	case len(added) > 0 || len(removed) > 0 || len(largeDelta) > 0:
		verdict = "restructured"
	case len(changed) > 0:
		verdict = "targeted"
	}

	return DiffResult{
		Verdict:    verdict,
		Added:      orEmpty(added),
		Removed:    orEmpty(removed),
		Changed:    orEmpty(changed),
		LargeDelta: orEmpty(largeDelta),
	}, nil
}

func keysOnlyIn(a, b map[string]Fingerprint) []string {
	var out []string
	for k := range a {
		if _, ok := b[k]; !ok {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

func sortedIntersection(a, b map[string]Fingerprint) []string {
	var out []string
	for k := range a {
		if _, ok := b[k]; ok {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// orEmpty makes sure a nil slice marshals as JSON `[]`, not `null` --
// encoding/json's default for a nil slice is `null`, but every one of
// these fields corresponds to a Python list that's always `[]` when
// empty, never `None`. Getting this wrong would fail the harness's own
// "parsed JSON, timestamps stripped" comparison on any manuscript with
// no changes at all -- the single most common diff result there is.
func orEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
