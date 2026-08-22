package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
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
	dir := newOutlineFixture(t, "section_01", "section_02", "section_03", "section_04")
	stale, err := ComputeOutlineStale(dir)
	if err != nil {
		t.Fatal(err)
	}
	// Written in a scrambled, non-sorted order on purpose, and with
	// differing scene counts per section, so neither insertion order nor
	// a coincidental total could stand in for an actual sort. With four
	// sections there are 4! = 24 possible orderings, so a single call
	// that happens to come out sorted proves little on its own -- see
	// the repeated-call loop below.
	writeOutlineSectionWithScenes(t, dir, "section_04", stale.CurrentHashes["section_04"],
		[]map[string]interface{}{scene(1, "flee")})
	writeOutlineSectionWithScenes(t, dir, "section_02", stale.CurrentHashes["section_02"],
		[]map[string]interface{}{scene(1, "escape")})
	writeOutlineSectionWithScenes(t, dir, "section_03", stale.CurrentHashes["section_03"],
		[]map[string]interface{}{scene(1, "confront"), scene(2, "reveal"), scene(3, "decide")})
	writeOutlineSectionWithScenes(t, dir, "section_01", stale.CurrentHashes["section_01"],
		[]map[string]interface{}{scene(1, "enter"), scene(2, "hide")})

	want := []string{"section_01", "section_02", "section_03", "section_04"}

	// loadOutlineSections keys its result by a Go map, so section order
	// out of ComputeOutlineJoin depends on ComputeOutlineJoin actually
	// sorting rather than on map iteration order, which Go randomizes
	// per process but can still land in sorted order by chance -- with 4
	// elements, 1/24 of the time. A single call is not decisive proof of
	// a sort; calling repeatedly is the only way to drive that chance
	// down to something a broken implementation can't pass by luck.
	for i := 0; i < 20; i++ {
		joined, err := ComputeOutlineJoin(dir)
		if err != nil {
			t.Fatal(err)
		}
		sections := joined["sections"].([]interface{})
		if len(sections) != 4 {
			t.Fatalf("iteration %d: joined %d sections, want 4", i, len(sections))
		}
		got := make([]string, len(sections))
		for j, s := range sections {
			got[j] = s.(map[string]interface{})["section"].(string)
		}
		for j := range want {
			if got[j] != want[j] {
				t.Fatalf("iteration %d: section order = %v, want %v (manuscript order, not directory/map order)", i, got, want)
			}
		}
		if gotCount := joined["scene_count"]; gotCount != 7 {
			t.Errorf("iteration %d: scene_count = %v, want 7", i, gotCount)
		}
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

func TestRenderOutline_ScenesAndFields(t *testing.T) {
	joined := map[string]interface{}{
		"scene_count": 1,
		"sections": []interface{}{
			map[string]interface{}{
				"section": "section_01",
				"scenes": []interface{}{
					map[string]interface{}{
						"order": float64(1), "pov": "Mira", "anchor": "The gate was already open",
						"goal": "Get inside.", "conflict": "The rotation ran early.", "outcome": "She is seen.",
					},
				},
			},
		},
	}
	got := RenderOutline(joined, []string{"goal", "conflict", "outcome"}, nil)

	for _, want := range []string{
		"# Outline", "## section_01", "Mira", "The gate was already open",
		"Goal: Get inside.", "Conflict: The rotation ran early.", "Outcome: She is seen.",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rendered outline missing %q:\n%s", want, got)
		}
	}
}

// TestRenderOutline_RendersRowFieldsInDeclarationOrder guards a
// load-bearing property: declaration order of row_fields is the column
// order the author reads and the field order of a generated agent
// prompt. Two different orderings are asserted, not one -- a single
// ordering could be satisfied by an implementation that hardcodes
// goal/conflict/outcome and ignores the rowFields parameter entirely.
// The second ordering is what proves the parameter actually drives the
// output.
func TestRenderOutline_RendersRowFieldsInDeclarationOrder(t *testing.T) {
	joined := map[string]interface{}{
		"scene_count": 1,
		"sections": []interface{}{
			map[string]interface{}{
				"section": "section_01",
				"scenes": []interface{}{
					map[string]interface{}{
						"order": float64(1), "pov": "Mira", "anchor": "The gate was already open",
						"goal":     "GOALTEXTUNIQUE",
						"conflict": "CONFLICTTEXTUNIQUE",
						"outcome":  "OUTCOMETEXTUNIQUE",
					},
				},
			},
		},
	}

	for _, fields := range [][]string{
		{"goal", "conflict", "outcome"},
		{"outcome", "goal", "conflict"},
	} {
		got := RenderOutline(joined, fields, nil)

		var indices []int
		for _, field := range fields {
			label := titleCaseField(field) + ":"
			idx := strings.Index(got, label)
			if idx < 0 {
				t.Fatalf("field label %q not found for order %v:\n%s", label, fields, got)
			}
			indices = append(indices, idx)
		}
		for i := 1; i < len(indices); i++ {
			if indices[i-1] >= indices[i] {
				t.Errorf("fields not rendered in declared order %v (indices %v):\n%s", fields, indices, got)
			}
		}
	}
}

func TestRenderOutline_SectionFieldOnlyWhenConfigured(t *testing.T) {
	joined := map[string]interface{}{
		"scene_count": 0,
		"sections": []interface{}{
			map[string]interface{}{
				"section": "section_01", "leaves_open": "Whether the guard reports her.",
				"scenes": []interface{}{},
			},
		},
	}
	withField := RenderOutline(joined, []string{"goal"}, []string{"leaves_open"})
	if !strings.Contains(withField, "Whether the guard reports her.") {
		t.Errorf("configured section field not rendered:\n%s", withField)
	}
	withoutField := RenderOutline(joined, []string{"goal"}, nil)
	if strings.Contains(withoutField, "Whether the guard reports her.") {
		t.Errorf("section field rendered for a domain that configures none:\n%s", withoutField)
	}
}

func TestRenderOutline_PublishedBoundary(t *testing.T) {
	joined := map[string]interface{}{
		"scene_count":       2,
		"published_through": "section_01",
		"sections": []interface{}{
			map[string]interface{}{"section": "section_01", "scenes": []interface{}{scene(1, "enter")}},
			map[string]interface{}{"section": "section_02", "scenes": []interface{}{scene(1, "escape")}},
		},
	}
	got := RenderOutline(joined, []string{"goal"}, nil)

	if !strings.Contains(got, "published") {
		t.Fatalf("no published boundary rendered:\n%s", got)
	}
	if count := strings.Count(got, "Everything above this line is published"); count != 1 {
		t.Errorf("boundary text rendered %d times, want exactly 1:\n%s", count, got)
	}
	boundary := strings.Index(got, "Everything above this line is published")
	first := strings.Index(got, "## section_01")
	second := strings.Index(got, "## section_02")
	// Guard the comparison below: strings.Index returns -1 for a needle
	// that isn't present at all, and -1 < -1 is false, so an absent
	// boundary or heading could otherwise satisfy the range check by
	// accident instead of failing it.
	for name, idx := range map[string]int{
		"published-boundary text": boundary,
		"## section_01 heading":   first,
		"## section_02 heading":   second,
	} {
		if idx < 0 {
			t.Fatalf("%s not found in rendered output:\n%s", name, got)
		}
	}
	if boundary < first || boundary > second {
		t.Errorf("boundary at %d is not between section_01 (%d) and section_02 (%d):\n%s", boundary, first, second, got)
	}
}

func TestRenderOutline_NoBoundaryWhenNothingPublished(t *testing.T) {
	joined := map[string]interface{}{
		"scene_count": 0,
		"sections":    []interface{}{map[string]interface{}{"section": "section_01", "scenes": []interface{}{}}},
	}
	if got := RenderOutline(joined, []string{"goal"}, nil); strings.Contains(got, "published") {
		t.Errorf("published boundary drawn when nothing is published:\n%s", got)
	}
}

func TestRunOutlineRender_WritesToManuscriptDirNotDotRedliner(t *testing.T) {
	dir := newOutlineFixture(t, "section_01")
	stale, _ := ComputeOutlineStale(dir)
	writeOutlineSectionWithScenes(t, dir, "section_01", stale.CurrentHashes["section_01"],
		[]map[string]interface{}{scene(1, "enter")})

	var buf bytes.Buffer
	if code := RunOutline([]string{"render", dir}, &buf, &buf); code != 0 {
		t.Fatalf("exit %d: %s", code, buf.String())
	}

	// The visible-file rule: an author who cannot find the deliverable
	// got nothing for the run.
	if _, err := os.Stat(filepath.Join(dir, "Outline.md")); err != nil {
		t.Fatalf("Outline.md not in the manuscript directory: %v", err)
	}
	if _, err := os.Stat(filepath.Join(OutlineDir(dir), "Outline.md")); err == nil {
		t.Error("Outline.md written under .redliner/ -- human-readable output belongs beside the sections")
	}
	if !strings.Contains(buf.String(), filepath.Join(dir, "Outline.md")) {
		t.Errorf("render did not print the path the author needs: %s", buf.String())
	}
}

func TestRunOutlineRender_OutlineMdIsNotMistakenForASection(t *testing.T) {
	dir := newOutlineFixture(t, "section_01")
	stale, _ := ComputeOutlineStale(dir)
	writeOutlineSectionWithScenes(t, dir, "section_01", stale.CurrentHashes["section_01"],
		[]map[string]interface{}{scene(1, "enter")})
	var buf bytes.Buffer
	RunOutline([]string{"render", dir}, &buf, &buf)

	sections, err := schemas.SectionFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(sections) != 1 {
		t.Errorf("section discovery found %v -- Outline.md must not be picked up as manuscript text", sections)
	}
}

// TestMain points FindDomainsDir at the repo's real domains/ for this
// package's tests. Under `go test` os.Executable() is a throwaway temp
// binary with no domains/ nearby -- REDLINER_DOMAINS_DIR is the escape
// hatch designed for exactly this, and golden_test.go already relies on
// it for the same reason.
//
// golden_test.go's repoRoot(t) does this same path computation, but it
// takes a *testing.T for t.Helper()/t.Fatal() and TestMain only has a
// *testing.M -- there's no T to hand it here. Inlining the two lines is
// deliberate, not a missed reuse: the alternative is a near-duplicate
// helper with a different signature, which trades one duplication for
// another.
func TestMain(m *testing.M) {
	if os.Getenv("REDLINER_DOMAINS_DIR") == "" {
		_, thisFile, _, _ := runtime.Caller(0)
		os.Setenv("REDLINER_DOMAINS_DIR", filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "domains"))
	}
	os.Exit(m.Run())
}
