package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeObservations(t *testing.T, dir, stem string, facts []map[string]interface{}) {
	t.Helper()
	obs := ObservationsDir(dir)
	if err := os.MkdirAll(obs, 0o755); err != nil {
		t.Fatal(err)
	}
	b, err := json.MarshalIndent(map[string]interface{}{"section": stem, "facts": facts}, "", " ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(obs, stem+".json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestBundleFacts_ShapeAndOrder(t *testing.T) {
	dir := t.TempDir()
	// Written out of order on purpose: section order must come from the
	// sort, not from directory listing luck.
	writeObservations(t, dir, "section_02", []map[string]interface{}{
		{"id": "fact-section_02-001", "entity": "Mira", "attribute": "eye_color", "value": "blue",
			"excerpt": "her blue eyes", "source": "narration", "confidence": "explicit", "entity_type": "character"},
	})
	writeObservations(t, dir, "section_01", []map[string]interface{}{
		{"id": "fact-section_01-003", "entity": "Mira", "attribute": "eye_color", "value": "green",
			"excerpt": "her green eyes", "source": "narration", "confidence": "explicit", "entity_type": "character"},
	})

	lines, err := BundleFacts(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"s1f003 | Mira | eye_color | green",
		"s2f001 | Mira | eye_color | blue",
	}
	if len(lines) != len(want) {
		t.Fatalf("got %d lines, want %d: %v", len(lines), len(want), lines)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("line %d:\n got %q\nwant %q", i, lines[i], want[i])
		}
	}
}

// The excerpt is the field most likely to creep back in "for context".
// It must not: beyond costing most of the bundle's size, a bundle
// carrying both a value and its quotation lets a reader find a seeded
// contradiction by spotting the mismatch between them, which is a much
// easier task than the entity join and would invalidate every recall
// number measured against this format.
func TestBundleFacts_CarriesOnlyTheFourColumns(t *testing.T) {
	dir := t.TempDir()
	writeObservations(t, dir, "section_01", []map[string]interface{}{
		{"id": "fact-section_01-001", "entity": "Mira", "attribute": "origin", "value": "Selkirk",
			"excerpt": "a girl out of Selkirk", "location": "paragraph 1",
			"source": "narration", "confidence": "explicit", "entity_type": "character"},
	})

	lines, err := BundleFacts(dir)
	if err != nil {
		t.Fatal(err)
	}
	line := lines[0]
	if strings.Count(line, " | ") != 3 {
		t.Fatalf("want exactly four columns, got %q", line)
	}
	for _, leaked := range []string{"a girl out of Selkirk", "paragraph 1", "narration", "explicit", "character"} {
		if strings.Contains(line, leaked) {
			t.Errorf("bundle leaked %q: %s", leaked, line)
		}
	}
}

func TestBundleFactID_PassesThroughUnexpectedShapes(t *testing.T) {
	for _, tc := range []struct{ stem, id, want string }{
		{"section_01", "fact-section_01-001", "s1f001"},
		{"section_12", "fact-section_12-107", "s12f107"},
		// A stem that isn't `section_NN`, or an id with no trailing
		// number, is left whole -- an unfamiliar id is still a usable
		// citation, a mangled one isn't.
		{"prologue", "fact-prologue-001", "s"},
		{"section_01", "weirdid", "weirdid"},
	} {
		got := bundleFactID(tc.stem, tc.id)
		if tc.want == "s" {
			if got != tc.id {
				t.Errorf("bundleFactID(%q,%q) = %q, want passthrough %q", tc.stem, tc.id, got, tc.id)
			}
			continue
		}
		if got != tc.want {
			t.Errorf("bundleFactID(%q,%q) = %q, want %q", tc.stem, tc.id, got, tc.want)
		}
	}
}

func TestBundleFacts_NoObservations(t *testing.T) {
	if _, err := BundleFacts(t.TempDir()); err != ErrNoObservations {
		t.Fatalf("want ErrNoObservations, got %v", err)
	}
}
