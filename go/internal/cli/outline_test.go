package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// newOutlineFixture builds a manuscript with the given sections and
// returns its directory. Section text is the stem itself, so hashes
// differ per section and are stable across runs.
func newOutlineFixture(t *testing.T, stems ...string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".redliner"), 0o755); err != nil {
		t.Fatal(err)
	}
	state := `{"manuscript_dir":"` + dir + `","domain":"fiction","phase":"developmental",` +
		`"developmental_round":1,"section_fingerprints":{},"created_at":"x"}`
	if err := os.WriteFile(filepath.Join(dir, ".redliner", "state.json"), []byte(state), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, stem := range stems {
		if err := os.WriteFile(filepath.Join(dir, stem+".txt"), []byte(stem+" body text\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// writeOutlineSection writes a per-section outline file with the given
// recorded hash (which may deliberately be stale).
func writeOutlineSection(t *testing.T, dir, stem, hash string) {
	t.Helper()
	if err := os.MkdirAll(OutlineSectionsDir(dir), 0o755); err != nil {
		t.Fatal(err)
	}
	body := map[string]interface{}{
		"section":        stem,
		"section_sha256": hash,
		"scenes":         []interface{}{},
	}
	raw, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(OutlineSectionsDir(dir), stem+".json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestComputeOutlineStale_NeverRecorded(t *testing.T) {
	dir := newOutlineFixture(t, "section_01", "section_02")

	got, err := ComputeOutlineStale(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.NeedsRecording) != 2 {
		t.Fatalf("NeedsRecording = %v, want both sections", got.NeedsRecording)
	}
	if got.NeedsRecording[0] != "section_01" || got.NeedsRecording[1] != "section_02" {
		t.Errorf("NeedsRecording = %v, want sorted [section_01 section_02]", got.NeedsRecording)
	}
	for _, stem := range got.NeedsRecording {
		if len(got.CurrentHashes[stem]) != 64 {
			t.Errorf("CurrentHashes[%s] = %q, want a 64-char digest", stem, got.CurrentHashes[stem])
		}
	}
}

func TestComputeOutlineStale_UnchangedSectionIsSkipped(t *testing.T) {
	dir := newOutlineFixture(t, "section_01")
	first, err := ComputeOutlineStale(dir)
	if err != nil {
		t.Fatal(err)
	}
	writeOutlineSection(t, dir, "section_01", first.CurrentHashes["section_01"])

	got, err := ComputeOutlineStale(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.NeedsRecording) != 0 {
		t.Errorf("NeedsRecording = %v, want empty -- an unchanged section must never be re-recorded", got.NeedsRecording)
	}
}

func TestComputeOutlineStale_ChangedSectionIsFlagged(t *testing.T) {
	dir := newOutlineFixture(t, "section_01")
	writeOutlineSection(t, dir, "section_01", "0000000000000000000000000000000000000000000000000000000000000000")

	got, err := ComputeOutlineStale(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.ChangedSinceRecording) != 1 || got.ChangedSinceRecording[0] != "section_01" {
		t.Errorf("ChangedSinceRecording = %v, want [section_01]", got.ChangedSinceRecording)
	}
	if len(got.NeverRecorded) != 0 {
		t.Errorf("NeverRecorded = %v, want empty", got.NeverRecorded)
	}
}

func TestComputeOutlineStale_OrphanedSectionIsReported(t *testing.T) {
	// A cut section's scenes must not still appear in the outline.
	dir := newOutlineFixture(t, "section_01")
	writeOutlineSection(t, dir, "section_99", "0000000000000000000000000000000000000000000000000000000000000000")

	got, err := ComputeOutlineStale(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.OrphanedSections) != 1 || got.OrphanedSections[0] != "section_99" {
		t.Errorf("OrphanedSections = %v, want [section_99]", got.OrphanedSections)
	}
}

func TestRunOutlineStale_PrintsJSON(t *testing.T) {
	dir := newOutlineFixture(t, "section_01")
	var buf bytes.Buffer
	if code := RunOutline([]string{"stale", dir}, &buf, &buf); code != 0 {
		t.Fatalf("exit %d: %s", code, buf.String())
	}
	var parsed OutlineStaleResult
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("stdout is not the JSON shape callers parse: %v\n%s", err, buf.String())
	}
	if len(parsed.NeedsRecording) != 1 {
		t.Errorf("needs_recording = %v, want one section", parsed.NeedsRecording)
	}
}
