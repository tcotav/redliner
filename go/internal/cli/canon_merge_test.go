package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeCanonJSON(t *testing.T, dir, name string, v interface{}) {
	t.Helper()
	canon := CanonDir(dir)
	if err := os.MkdirAll(canon, 0o755); err != nil {
		t.Fatal(err)
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(canon, name), append(b, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readContradictions(t *testing.T, dir string) []map[string]interface{} {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(CanonDir(dir), "continuity.json"))
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		Contradictions []map[string]interface{} `json:"contradictions"`
	}
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatal(err)
	}
	return parsed.Contradictions
}

func contradiction(id string, factIDs ...string) map[string]interface{} {
	ids := make([]interface{}, len(factIDs))
	for i, f := range factIDs {
		ids[i] = f
	}
	return map[string]interface{}{
		"id": id, "status": "open", "kind": "contradiction",
		"category": "character_attribute", "severity": "moderate",
		"fact_ids": ids, "note": "n",
	}
}

func TestMergeJoined_KeepsAdjudicatorEntriesAndOffsetsJoinerIDs(t *testing.T) {
	dir := t.TempDir()
	writeCanonJSON(t, dir, "continuity.json", map[string]interface{}{
		"contradictions": []interface{}{contradiction("cont-001", "f1", "f2")},
	})
	writeCanonJSON(t, dir, "joined.json", map[string]interface{}{
		"contradictions": []interface{}{
			contradiction("cont-001", "f3", "f4"),
			contradiction("cont-002", "f5", "f6"),
		},
	})

	added, skipped, err := MergeJoined(dir)
	if err != nil {
		t.Fatal(err)
	}
	if added != 2 || skipped != 0 {
		t.Fatalf("got added=%d skipped=%d, want 2/0", added, skipped)
	}

	got := readContradictions(t, dir)
	if len(got) != 3 {
		t.Fatalf("want 3 contradictions, got %d", len(got))
	}
	// The adjudicator's entry keeps its id; the joiner's are offset into
	// the 5NN range, which is what makes provenance readable at a glance.
	wantIDs := []string{"cont-001", "cont-501", "cont-502"}
	for i, want := range wantIDs {
		if got[i]["id"] != want {
			t.Errorf("contradiction %d: id = %v, want %v", i, got[i]["id"], want)
		}
	}
}

// Re-running a join must not duplicate what is already merged -- the
// joiner renumbers from scratch each run, so identity has to come from
// the facts cited, not from the id.
func TestMergeJoined_IsIdempotent(t *testing.T) {
	dir := t.TempDir()
	writeCanonJSON(t, dir, "joined.json", map[string]interface{}{
		"contradictions": []interface{}{contradiction("cont-001", "f3", "f4")},
	})

	if _, _, err := MergeJoined(dir); err != nil {
		t.Fatal(err)
	}
	added, skipped, err := MergeJoined(dir)
	if err != nil {
		t.Fatal(err)
	}
	if added != 0 || skipped != 1 {
		t.Fatalf("second merge: added=%d skipped=%d, want 0/1", added, skipped)
	}
	if got := readContradictions(t, dir); len(got) != 1 {
		t.Fatalf("want 1 contradiction after two merges, got %d", len(got))
	}
}

// Fact order within an entry is not meaningful, so two entries citing the
// same facts in a different order are the same finding.
func TestMergeJoined_DedupesRegardlessOfFactOrder(t *testing.T) {
	dir := t.TempDir()
	writeCanonJSON(t, dir, "continuity.json", map[string]interface{}{
		"contradictions": []interface{}{contradiction("cont-001", "f2", "f1")},
	})
	writeCanonJSON(t, dir, "joined.json", map[string]interface{}{
		"contradictions": []interface{}{contradiction("cont-001", "f1", "f2")},
	})
	added, skipped, err := MergeJoined(dir)
	if err != nil {
		t.Fatal(err)
	}
	if added != 0 || skipped != 1 {
		t.Fatalf("got added=%d skipped=%d, want 0/1", added, skipped)
	}
}

func TestMergeJoined_NoContinuityFileYet(t *testing.T) {
	dir := t.TempDir()
	writeCanonJSON(t, dir, "joined.json", map[string]interface{}{
		"contradictions": []interface{}{contradiction("cont-001", "f1", "f2")},
	})
	added, _, err := MergeJoined(dir)
	if err != nil {
		t.Fatal(err)
	}
	if added != 1 {
		t.Fatalf("want 1 added, got %d", added)
	}
	if got := readContradictions(t, dir); got[0]["id"] != "cont-501" {
		t.Fatalf("want cont-501, got %v", got[0]["id"])
	}
}

func TestCanonMerge_MissingJoinedFileIsAClearMessage(t *testing.T) {
	dir := t.TempDir()
	var buf bytes.Buffer
	if code := cmdCanonMerge(dir, &buf); code != 1 {
		t.Fatalf("want exit 1, got %d", code)
	}
	if !strings.Contains(buf.String(), "run the continuity joiner first") {
		t.Errorf("unhelpful message: %s", buf.String())
	}
}
