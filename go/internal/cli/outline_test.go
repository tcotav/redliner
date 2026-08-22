package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tcotav/redliner/go/internal/schemas"
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

// assertSectionCostsNothing asserts stem is absent from all four result
// slices and has no entry in CurrentHashes. A section whose text hasn't
// moved must cost nothing -- and "costs nothing" means absent everywhere,
// not just absent from NeedsRecording.
func assertSectionCostsNothing(t *testing.T, got OutlineStaleResult, stem string) {
	t.Helper()
	for name, list := range map[string][]string{
		"NeedsRecording":        got.NeedsRecording,
		"NeverRecorded":         got.NeverRecorded,
		"ChangedSinceRecording": got.ChangedSinceRecording,
		"OrphanedSections":      got.OrphanedSections,
	} {
		for _, s := range list {
			if s == stem {
				t.Errorf("%s contains %q, want absent -- unchanged sections must cost nothing", name, stem)
			}
		}
	}
	if hash, ok := got.CurrentHashes[stem]; ok {
		t.Errorf("CurrentHashes[%s] = %q, want no entry -- unchanged sections must cost nothing", stem, hash)
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
	if len(got.CurrentHashes) != len(got.NeedsRecording) {
		t.Errorf("CurrentHashes has %d entries, want exactly %d (one per needing section, nothing else)", len(got.CurrentHashes), len(got.NeedsRecording))
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
	assertSectionCostsNothing(t, got, "section_01")
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
	for _, stem := range got.NeedsRecording {
		if stem == "section_99" {
			t.Errorf("NeedsRecording = %v, must not contain the orphan -- an orphaned section must not double-count as needing recording", got.NeedsRecording)
		}
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

// writeOutlineSectionWithScenes writes a per-section file carrying real
// scene rows, for the join and render tests.
func writeOutlineSectionWithScenes(t *testing.T, dir, stem, hash string, scenes []map[string]interface{}) {
	t.Helper()
	if err := os.MkdirAll(OutlineSectionsDir(dir), 0o755); err != nil {
		t.Fatal(err)
	}
	rows := make([]interface{}, len(scenes))
	for i, s := range scenes {
		rows[i] = s
	}
	body := map[string]interface{}{
		"section":        stem,
		"section_sha256": hash,
		"scenes":         rows,
	}
	raw, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(OutlineSectionsDir(dir), stem+".json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func scene(order int, goal string) map[string]interface{} {
	return map[string]interface{}{
		"order": float64(order), "pov": "Mira", "anchor": "anchor text " + goal,
		"goal": goal, "conflict": "opposition", "outcome": "a change",
	}
}

func TestComputeOutlineJoin_SectionOrderAndCount(t *testing.T) {
	dir := newOutlineFixture(t, "section_01", "section_02")
	stale, err := ComputeOutlineStale(dir)
	if err != nil {
		t.Fatal(err)
	}
	// Written out of order on purpose -- the join must sort, not trust
	// directory iteration order.
	writeOutlineSectionWithScenes(t, dir, "section_02", stale.CurrentHashes["section_02"],
		[]map[string]interface{}{scene(1, "escape")})
	writeOutlineSectionWithScenes(t, dir, "section_01", stale.CurrentHashes["section_01"],
		[]map[string]interface{}{scene(1, "enter"), scene(2, "hide")})

	joined, err := ComputeOutlineJoin(dir)
	if err != nil {
		t.Fatal(err)
	}
	sections := joined["sections"].([]interface{})
	if len(sections) != 2 {
		t.Fatalf("joined %d sections, want 2", len(sections))
	}
	if got := sections[0].(map[string]interface{})["section"]; got != "section_01" {
		t.Errorf("first section = %v, want section_01 (manuscript order, not directory order)", got)
	}
	if got := joined["scene_count"]; got != 3 {
		t.Errorf("scene_count = %v, want 3", got)
	}
}

func TestComputeOutlineJoin_IncludesEverySectionNotJustStaleOnes(t *testing.T) {
	dir := newOutlineFixture(t, "section_01", "section_02")
	stale, err := ComputeOutlineStale(dir)
	if err != nil {
		t.Fatal(err)
	}
	writeOutlineSectionWithScenes(t, dir, "section_01", stale.CurrentHashes["section_01"],
		[]map[string]interface{}{scene(1, "enter")})
	writeOutlineSectionWithScenes(t, dir, "section_02", stale.CurrentHashes["section_02"],
		[]map[string]interface{}{scene(1, "escape")})

	joined, err := ComputeOutlineJoin(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(joined["sections"].([]interface{})) != 2 {
		t.Error("join must rebuild from every current section file, not only re-recorded ones")
	}
}

func TestComputeOutlineJoin_CarriesPublishedThrough(t *testing.T) {
	dir := newOutlineFixture(t, "section_01")
	stale, _ := ComputeOutlineStale(dir)
	writeOutlineSectionWithScenes(t, dir, "section_01", stale.CurrentHashes["section_01"],
		[]map[string]interface{}{scene(1, "enter")})

	joined, err := ComputeOutlineJoin(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, present := joined["published_through"]; present {
		t.Error("published_through present when state does not set it")
	}

	state, err := schemas.LoadState(dir)
	if err != nil {
		t.Fatal(err)
	}
	state.PublishedThrough = "section_01"
	if _, err := schemas.SaveState(dir, state); err != nil {
		t.Fatal(err)
	}

	joined, err = ComputeOutlineJoin(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := joined["published_through"]; got != "section_01" {
		t.Errorf("published_through = %v, want section_01", got)
	}
}

func TestRunOutlineJoin_WritesOutlineJSON(t *testing.T) {
	dir := newOutlineFixture(t, "section_01")
	stale, _ := ComputeOutlineStale(dir)
	writeOutlineSectionWithScenes(t, dir, "section_01", stale.CurrentHashes["section_01"],
		[]map[string]interface{}{scene(1, "enter")})

	var buf bytes.Buffer
	if code := RunOutline([]string{"join", dir}, &buf, &buf); code != 0 {
		t.Fatalf("exit %d: %s", code, buf.String())
	}
	raw, err := os.ReadFile(OutlinePath(dir))
	if err != nil {
		t.Fatalf("join wrote no outline.json: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("outline.json is not valid JSON: %v", err)
	}
	if parsed["scene_count"] != float64(1) {
		t.Errorf("scene_count = %v, want 1", parsed["scene_count"])
	}
}

func TestRunOutlineJoin_ErrorsWithNoSections(t *testing.T) {
	dir := newOutlineFixture(t, "section_01")
	var buf bytes.Buffer
	if code := RunOutline([]string{"join", dir}, &buf, &buf); code == 0 {
		t.Error("join with no recorded sections should fail, not write an empty outline")
	}
	if !strings.Contains(buf.String(), "outline stale") {
		t.Errorf("error should point at the command that fixes it, got: %s", buf.String())
	}
}
