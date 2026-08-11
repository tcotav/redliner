package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// This file re-runs the exact operation sequences capture_baseline.py
// ran against the real bin/*.py scripts (see harness/README.md), but
// through cli.Dispatch in-process instead of a subprocess -- the actual
// differential check Phase 3 exists to do. New CLI shape (`redliner
// state status <dir>` instead of `redliner_state.py status <dir>`) per
// TODO.md's "v1 plan" decision, everything downstream of that argv
// translation is compared against the real Python-produced golden data.

func repoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not determine test file location")
	}
	// go/internal/cli/golden_test.go -> go/internal/cli -> go/internal -> go -> repo root
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
}

// Note: FindDomainsDir's ancestor search starts from os.Executable(),
// which under `go test` is a throwaway temp binary with no domains/
// nearby -- TestCLI_MatchesPythonGolden below sets REDLINER_DOMAINS_DIR
// as the designed escape hatch for exactly this. Real end-to-end
// verification of the search itself (not just this override) happens in
// schemas' own tests and at the Phase 5 live-install gate.

type step struct {
	label string
	args  func(dir string) []string
}

var fixtureScripts = map[string][]step{
	"happy": {
		{"01_domain_list", func(string) []string { return []string{"domain", "list"} }},
		{"02_domain_show_fiction", func(string) []string { return []string{"domain", "show", "fiction"} }},
		{"03_state_status", func(d string) []string { return []string{"state", "status", d} }},
		{"04_state_diff", func(d string) []string { return []string{"state", "diff", d} }},
		{"05_canon_stale", func(d string) []string { return []string{"canon", "stale", d} }},
		{"06_canon_reconcile", func(d string) []string { return []string{"canon", "reconcile", d} }},
		{"07_validate_findings", func(d string) []string { return []string{"validate", d} }},
		{"08_state_init_again", func(d string) []string { return []string{"state", "init", d} }},
		{"09_state_phase_intake", func(d string) []string { return []string{"state", "phase", d, "intake"} }},
		{"10_state_phase_developmental", func(d string) []string { return []string{"state", "phase", d, "developmental"} }},
		{"11_state_snapshot", func(d string) []string { return []string{"state", "snapshot", d} }},
	},
	"crlf": {
		{"01_state_init", func(d string) []string { return []string{"state", "init", d} }},
		{"02_state_status", func(d string) []string { return []string{"state", "status", d} }},
		{"03_state_snapshot", func(d string) []string { return []string{"state", "snapshot", d} }},
		{"04_state_status_after", func(d string) []string { return []string{"state", "status", d} }},
		{"05_state_diff_unchanged", func(d string) []string { return []string{"state", "diff", d} }},
	},
	"collision": {
		{"01_state_init", func(d string) []string { return []string{"state", "init", d} }},
		{"02_state_diff_collision", func(d string) []string { return []string{"state", "diff", d} }},
		{"03_state_snapshot_collision", func(d string) []string { return []string{"state", "snapshot", d} }},
		{"04_canon_stale_collision", func(d string) []string { return []string{"canon", "stale", d} }},
	},
	"empty": {
		{"01_state_status_no_state", func(d string) []string { return []string{"state", "status", d} }},
		{"02_domain_list", func(string) []string { return []string{"domain", "list"} }},
		{"03_validate_findings_no_redliner_dir", func(d string) []string { return []string{"validate", d} }},
	},
}

// fixtureOrder makes sub-test iteration deterministic.
var fixtureOrder = []string{"happy", "crlf", "collision", "empty"}

// knownDivergentStdout names the one step whose Go stdout intentionally
// differs from Python's: requireState's "no state yet" message now says
// `redliner state init <dir>` (this port's actual invocation syntax),
// not `redliner_state.py init <dir>` (Python's) -- see state.go's
// requireState comment. Every other non-JSON step's stdout, regardless
// of exit code, is compared exactly; this is the only named exception.
var knownDivergentStdout = map[string]bool{
	"empty/01_state_status_no_state": true,
}

var timestampKeys = map[string]bool{"created_at": true, "updated_at": true}

func stripTimestamps(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		out := map[string]interface{}{}
		for k, vv := range val {
			if timestampKeys[k] {
				continue
			}
			out[k] = stripTimestamps(vv)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(val))
		for i, vv := range val {
			out[i] = stripTimestamps(vv)
		}
		return out
	default:
		return val
	}
}

func loadGolden(t *testing.T, fixture, label string) map[string]interface{} {
	t.Helper()
	path := filepath.Join(repoRoot(t), "go", "harness", "golden", fixture, label+".json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading golden %s: %v", path, err)
	}
	var v map[string]interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("parsing golden %s: %v", path, err)
	}
	return v
}

// snapshotStateDir mirrors capture_baseline.py's snapshot_state_dir:
// every JSON file under .redliner/, parsed, timestamps stripped.
func snapshotStateDir(manuscriptDir string) map[string]interface{} {
	redlinerDir := filepath.Join(manuscriptDir, ".redliner")
	out := map[string]interface{}{}
	filepath.Walk(redlinerDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}
		rel, _ := filepath.Rel(redlinerDir, path)
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		var v interface{}
		if err := json.Unmarshal(raw, &v); err != nil {
			out[rel] = map[string]interface{}{"__unparseable__": string(raw)}
			return nil
		}
		out[rel] = stripTimestamps(v)
		return nil
	})
	return out
}

func TestCLI_MatchesPythonGolden(t *testing.T) {
	t.Setenv("REDLINER_DOMAINS_DIR", filepath.Join(repoRoot(t), "domains"))
	fixturesRoot := filepath.Join(repoRoot(t), "go", "harness", "fixtures")

	// Deliberately NOT t.TempDir(): several golden values embed the
	// manuscript's absolute path verbatim (state.json's manuscript_dir
	// field, the "Initialized <path>" and "OK <path>" messages) --
	// that's real captured Python behavior, not an artifact worth
	// normalizing away. Reusing capture_baseline.py's own working
	// directory (go/harness/.work/<fixture>, same convention, gitignored)
	// makes the embedded paths match exactly instead of chasing this
	// with path-substitution logic.
	workRoot := filepath.Join(repoRoot(t), "go", "harness", ".work")

	// The golden files embed workRoot verbatim (state.json's
	// manuscript_dir, "Initialized <path>"/"OK <path>" messages) -- they
	// were captured from *this* checkout's absolute path. Running from a
	// worktree, a second clone, or CI at a different path makes every
	// path-embedding subtest fail with a diff that looks exactly like a
	// port bug but isn't one. Fail loudly and specifically instead of
	// letting that happen silently.
	probe := loadGolden(t, "crlf", "01_state_init")
	if stdout, _ := probe["stdout"].(string); !strings.Contains(stdout, workRoot) {
		t.Skipf("golden data was captured from a different checkout path than %s -- "+
			"re-run `python3 go/harness/capture_baseline.py` from this checkout first", workRoot)
	}

	for _, fixture := range fixtureOrder {
		fixture := fixture
		t.Run(fixture, func(t *testing.T) {
			manuscriptDir := filepath.Join(workRoot, fixture)
			os.RemoveAll(manuscriptDir)
			t.Cleanup(func() { os.RemoveAll(manuscriptDir) })
			if err := copyDir(filepath.Join(fixturesRoot, fixture), manuscriptDir); err != nil {
				t.Fatalf("copying fixture: %v", err)
			}

			for _, s := range fixtureScripts[fixture] {
				s := s
				t.Run(s.label, func(t *testing.T) {
					var stdout, stderr bytes.Buffer
					args := s.args(manuscriptDir)
					exitCode := Dispatch(args, &stdout, &stderr)

					golden := loadGolden(t, fixture, s.label)
					wantExit := int(golden["exit_code"].(float64))
					if exitCode != wantExit {
						t.Errorf("exit code: got %d, want %d\nargs: %v\nstdout:\n%s", exitCode, wantExit, args, stdout.String())
					}

					if wantJSON, ok := golden["stdout_json"]; ok && wantJSON != nil {
						var gotJSON interface{}
						if err := json.Unmarshal(stdout.Bytes(), &gotJSON); err != nil {
							t.Fatalf("Go stdout is not valid JSON for a step whose Python golden was JSON: %v\nstdout:\n%s", err, stdout.String())
						}
						gotNorm := stripTimestamps(gotJSON)
						wantNorm := stripTimestamps(wantJSON)
						gotBytes, _ := json.Marshal(gotNorm)
						wantBytes, _ := json.Marshal(wantNorm)
						if !bytes.Equal(gotBytes, wantBytes) {
							t.Errorf("stdout_json mismatch\n got: %s\nwant: %s", gotBytes, wantBytes)
						}
					} else if !knownDivergentStdout[fixture+"/"+s.label] {
						// Non-JSON stdout, any exit code: compare text
						// exactly, per the harness's own stated
						// comparison rule for human-facing prints --
						// "Canon: N entities...", "Phase: X -> Y...",
						// "State already exists at...", "Section file
						// error: ...", "No .redliner/ under...". Only
						// the one step in knownDivergentStdout is exempt,
						// by name, not by exit code -- an exit-code-based
						// blanket exemption would silently stop checking
						// 5 of these strings that actually do match.
						wantStdout, _ := golden["stdout"].(string)
						if stdout.String() != wantStdout {
							t.Errorf("stdout mismatch\n got: %q\nwant: %q", stdout.String(), wantStdout)
						}
					}

					wantSnapshot, _ := golden["state_dir_snapshot"].(map[string]interface{})
					gotSnapshot := snapshotStateDir(manuscriptDir)
					wantBytes, _ := json.Marshal(stripTimestamps(wantSnapshot))
					gotBytes, _ := json.Marshal(stripTimestamps(gotSnapshot))
					if !bytes.Equal(gotBytes, wantBytes) {
						t.Errorf(".redliner/ tree mismatch after this step\n got: %s\nwant: %s", gotBytes, wantBytes)
					}
				})
			}
		})
	}
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, raw, info.Mode())
	})
}
