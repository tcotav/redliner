package schemas

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFingerprint_MatchesPythonGolden hashes the real "happy" fixture
// sections and checks the result against capture_baseline.py's real
// `bin/redliner_state.py status` output -- not a value re-derived by
// reasoning about the algorithm, the actual byte-identical output of
// the Python implementation this is a port of.
func TestFingerprint_MatchesPythonGolden(t *testing.T) {
	golden := loadGoldenJSON(t, "happy", "03_state_status")
	wantFingerprints, ok := golden["section_fingerprints"].(map[string]interface{})
	if !ok {
		t.Fatal("golden section_fingerprints missing or wrong shape")
	}
	if len(wantFingerprints) == 0 {
		t.Fatal("golden section_fingerprints is empty -- test would pass vacuously")
	}

	got, err := FingerprintManuscript(filepath.Join(fixturesDir(t), "happy"))
	if err != nil {
		t.Fatalf("FingerprintManuscript: %v", err)
	}

	for stem, wantRaw := range wantFingerprints {
		want := wantRaw.(map[string]interface{})
		fp, ok := got[stem]
		if !ok {
			t.Errorf("section %q: missing from Go output", stem)
			continue
		}
		if fp.SHA256 != want["sha256"] {
			t.Errorf("section %q sha256: got %s, want %s", stem, fp.SHA256, want["sha256"])
		}
		if float64(fp.Words) != want["words"] {
			t.Errorf("section %q words: got %d, want %v", stem, fp.Words, want["words"])
		}
	}
	if len(got) != len(wantFingerprints) {
		t.Errorf("section count: got %d, want %d", len(got), len(wantFingerprints))
	}
}

// TestFingerprint_CRLF is the load-bearing regression test for the
// porting hazard TODO.md calls out by name: Python's Path.read_text()
// does universal-newline translation, Go's os.ReadFile doesn't. Without
// NormalizeNewlines, this hash (and word count) would differ from
// Python's, and every CRLF-authored manuscript would false-flag as
// "changed" on its first diff after cutover.
func TestFingerprint_CRLF(t *testing.T) {
	wantSnapshot := loadJSONFile(t, filepath.Join(goldenDir(t), "crlf", "03_state_snapshot.json")).(map[string]interface{})
	stateDir := wantSnapshot["state_dir_snapshot"].(map[string]interface{})
	stateJSON := stateDir["state.json"].(map[string]interface{})
	wantFingerprints := stateJSON["section_fingerprints"].(map[string]interface{})
	want := wantFingerprints["section_01"].(map[string]interface{})

	fp, err := FingerprintSection(filepath.Join(fixturesDir(t), "crlf", "section_01.txt"))
	if err != nil {
		t.Fatalf("FingerprintSection: %v", err)
	}

	if fp.SHA256 != want["sha256"] {
		t.Errorf("CRLF section sha256: got %s, want %s (NormalizeNewlines not matching Python's universal-newline read)", fp.SHA256, want["sha256"])
	}
	if float64(fp.Words) != want["words"] {
		t.Errorf("CRLF section words: got %d, want %v", fp.Words, want["words"])
	}
}

// TestFingerprint_CRLFRegression_WithoutNormalization proves
// TestFingerprint_CRLF is actually load-bearing: hashing the fixture's
// raw CRLF bytes (bypassing NormalizeNewlines entirely) must produce a
// DIFFERENT hash than the golden value. If it ever matches, either the
// fixture stopped containing CRLF or NormalizeNewlines has become a
// no-op -- either way TestFingerprint_CRLF would no longer be checking
// anything real.
func TestFingerprint_CRLFRegression_WithoutNormalization(t *testing.T) {
	path := filepath.Join(fixturesDir(t), "crlf", "section_01.txt")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	if !bytes.Contains(raw, []byte("\r\n")) {
		t.Fatal("fixture no longer contains CRLF -- this test needs a CRLF fixture to be meaningful")
	}

	wantSnapshot := loadJSONFile(t, filepath.Join(goldenDir(t), "crlf", "03_state_snapshot.json")).(map[string]interface{})
	stateDir := wantSnapshot["state_dir_snapshot"].(map[string]interface{})
	stateJSON := stateDir["state.json"].(map[string]interface{})
	want := stateJSON["section_fingerprints"].(map[string]interface{})["section_01"].(map[string]interface{})

	unnormalizedSum := sha256.Sum256(raw)
	unnormalizedHash := hex.EncodeToString(unnormalizedSum[:])
	if unnormalizedHash == want["sha256"] {
		t.Fatal("un-normalized hash matches the golden hash -- CRLF has no effect here, TestFingerprint_CRLF would be vacuous")
	}
}

func TestSectionFiles_Collision(t *testing.T) {
	_, err := SectionFiles(filepath.Join(fixturesDir(t), "collision"))
	if err == nil {
		t.Fatal("expected SectionCollisionError, got nil")
	}
	var collisionErr *SectionCollisionError
	if ce, ok := err.(*SectionCollisionError); ok {
		collisionErr = ce
	} else {
		t.Fatalf("expected *SectionCollisionError, got %T: %v", err, err)
	}
	if len(collisionErr.Stems) != 1 || collisionErr.Stems[0] != "section_01" {
		t.Errorf("collision stems: got %v, want [section_01]", collisionErr.Stems)
	}
}

func TestLoadState_Happy(t *testing.T) {
	state, err := LoadState(filepath.Join(fixturesDir(t), "happy"))
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if state == nil {
		t.Fatal("LoadState returned nil for a fixture with real state.json")
	}
	if state.Domain != "fiction" {
		t.Errorf("Domain: got %q, want fiction", state.Domain)
	}
	if state.Phase != "developmental" {
		t.Errorf("Phase: got %q, want developmental", state.Phase)
	}
	if state.DevelopmentalRound != 1 {
		t.Errorf("DevelopmentalRound: got %d, want 1", state.DevelopmentalRound)
	}
	if len(state.SectionFingerprints) != 2 {
		t.Errorf("SectionFingerprints: got %d entries, want 2", len(state.SectionFingerprints))
	}
}

func TestLoadState_Missing(t *testing.T) {
	state, err := LoadState(filepath.Join(fixturesDir(t), "empty"))
	if err != nil {
		t.Fatalf("LoadState on a manuscript with no state: %v", err)
	}
	if state != nil {
		t.Errorf("expected nil state for a manuscript with no .redliner/, got %+v", state)
	}
}

func TestDiffManuscript_Happy_Unchanged(t *testing.T) {
	dir := filepath.Join(fixturesDir(t), "happy")
	state, err := LoadState(dir)
	if err != nil || state == nil {
		t.Fatalf("LoadState: %v, %v", state, err)
	}
	diff, err := DiffManuscript(dir, state)
	if err != nil {
		t.Fatalf("DiffManuscript: %v", err)
	}
	if diff.Verdict != "unchanged" {
		t.Errorf("verdict: got %q, want unchanged (fixture's state.json should already match its section text)", diff.Verdict)
	}
	if len(diff.Added) != 0 || len(diff.Removed) != 0 || len(diff.Changed) != 0 {
		t.Errorf("expected no added/removed/changed, got %+v", diff)
	}
}

func TestSaveState_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	state := NewState(dir, "fiction")
	state.SectionFingerprints["section_01"] = Fingerprint{SHA256: "abc123", Words: 42}

	path, err := SaveState(dir, state)
	if err != nil {
		t.Fatalf("SaveState: %v", err)
	}
	if path != StatePath(dir) {
		t.Errorf("SaveState path: got %s, want %s", path, StatePath(dir))
	}
	if state.UpdatedAt == "" {
		t.Error("SaveState should set UpdatedAt")
	}

	reloaded, err := LoadState(dir)
	if err != nil {
		t.Fatalf("LoadState after save: %v", err)
	}
	if reloaded == nil {
		t.Fatal("LoadState returned nil right after SaveState wrote a file")
	}
	if reloaded.Domain != "fiction" || reloaded.Phase != "intake" {
		t.Errorf("reloaded state mismatch: %+v", reloaded)
	}
	if fp := reloaded.SectionFingerprints["section_01"]; fp.SHA256 != "abc123" || fp.Words != 42 {
		t.Errorf("reloaded fingerprint mismatch: %+v", fp)
	}

	// Double-init-style check: a second State load must reflect the same
	// data (JSON round-trips cleanly, not just "doesn't error").
	secondPath, err := SaveState(dir, reloaded)
	if err != nil {
		t.Fatalf("second SaveState: %v", err)
	}
	if secondPath != path {
		t.Errorf("second SaveState path changed: got %s, want %s", secondPath, path)
	}
}

func TestNewState_JSONShape(t *testing.T) {
	state := NewState("some/dir", "")
	if state.Domain != DefaultDomain {
		t.Errorf("empty domain should default to %q, got %q", DefaultDomain, state.Domain)
	}
	if state.Phase != "intake" {
		t.Errorf("new state phase: got %q, want intake", state.Phase)
	}
	if state.SectionFingerprints == nil {
		t.Error("SectionFingerprints must be a non-nil empty map, or it marshals as JSON null instead of {}")
	}
}

func TestState_OutlineFieldsAreOptional(t *testing.T) {
	dir := t.TempDir()

	// A state file written before these fields existed must still load,
	// and must not gain the keys when saved back -- the harness goldens
	// compare the whole .redliner/ tree.
	legacy := `{"manuscript_dir":"m","domain":"fiction","phase":"intake",` +
		`"developmental_round":0,"section_fingerprints":{},"created_at":"x"}`
	if err := os.MkdirAll(StateDir(dir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(StatePath(dir), []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}

	state, err := LoadState(dir)
	if err != nil || state == nil {
		t.Fatalf("legacy state failed to load: %v", err)
	}
	if state.OutlineVersion != 0 || state.PublishedThrough != "" {
		t.Errorf("legacy state gained values: version=%d published=%q", state.OutlineVersion, state.PublishedThrough)
	}

	if _, err := SaveState(dir, state); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(StatePath(dir))
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"outline_version", "published_through"} {
		if strings.Contains(string(raw), key) {
			t.Errorf("unset %s was written to state.json -- it must be omitempty", key)
		}
	}
}

func TestState_OutlineFieldsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	state := NewState(dir, "serial-fiction")
	state.OutlineVersion = 4
	state.PublishedThrough = "section_11"
	if _, err := SaveState(dir, state); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadState(dir)
	if err != nil || loaded == nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.OutlineVersion != 4 {
		t.Errorf("OutlineVersion = %d, want 4", loaded.OutlineVersion)
	}
	if loaded.PublishedThrough != "section_11" {
		t.Errorf("PublishedThrough = %q, want section_11", loaded.PublishedThrough)
	}
}
