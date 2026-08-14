package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDecisionsApply_RestoresWhatAPassOverwrote(t *testing.T) {
	// The failure this guards: findings files are rewritten wholesale by
	// the editors on each re-check, so an agent that ignores "preserve
	// author statuses" silently discards a decision the author made.
	// Decisions live where agents don't write, and are re-applied.
	dir := t.TempDir()
	findings := filepath.Join(dir, ".redliner", "findings")
	if err := os.MkdirAll(findings, 0o755); err != nil {
		t.Fatal(err)
	}

	write := func(path string, v interface{}) {
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write(filepath.Join(dir, ".redliner", "decisions.json"), map[string]interface{}{
		"decisions": []map[string]string{
			{"id": "line-section_01-001", "status": "wontfix", "set_by": "author",
				"at": "2026-08-14T12:00:00Z", "reason": "deliberate"},
			{"id": "line-section_01-999", "status": "wontfix", "set_by": "author",
				"at": "2026-08-14T12:00:00Z", "reason": "section was cut"},
		},
	})
	write(filepath.Join(findings, "line_section_01.json"), map[string]interface{}{
		"section": "section_01",
		"findings": []map[string]interface{}{
			{"id": "line-section_01-001", "status": "open", "category": "word_choice",
				"severity": "minor", "location": "para 1", "note": "n"},
		},
	})

	var buf bytes.Buffer
	if code := RunDecisions([]string{"apply", dir}, &buf); code != 0 {
		t.Fatalf("apply failed: %s", buf.String())
	}
	out := buf.String()
	if !strings.Contains(out, "1 restored") {
		t.Errorf("want 1 restored, got: %s", out)
	}
	// A decision whose finding no longer exists is reported, not an error:
	// sections get cut, and a decision applying to nothing forever would
	// otherwise be invisible.
	if !strings.Contains(out, "line-section_01-999") {
		t.Errorf("want the vanished finding reported, got: %s", out)
	}

	raw, _ := os.ReadFile(filepath.Join(findings, "line_section_01.json"))
	var doc map[string]interface{}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	f := doc["findings"].([]interface{})[0].(map[string]interface{})
	if f["status"] != "wontfix" {
		t.Errorf("status = %v, want wontfix", f["status"])
	}
	res, ok := f["resolution"].(map[string]interface{})
	if !ok || res["set_by"] != "author" || res["reason"] != "deliberate" {
		t.Errorf("resolution not restored: %v", f["resolution"])
	}
}
