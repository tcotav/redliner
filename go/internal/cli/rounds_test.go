package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRoundsArchive_KeepsEachPassSeparately(t *testing.T) {
	dir := t.TempDir()
	stateDir := filepath.Join(dir, ".redliner")
	for _, sub := range []string{"findings", "canon"} {
		if err := os.MkdirAll(filepath.Join(stateDir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	state := `{"manuscript_dir":"` + dir + `","domain":"fiction","phase":"line",` +
		`"developmental_round":2,"section_fingerprints":{},"created_at":"x"}`
	if err := os.WriteFile(filepath.Join(stateDir, "state.json"), []byte(state), 0o644); err != nil {
		t.Fatal(err)
	}
	write := func(rel string) {
		if err := os.WriteFile(filepath.Join(stateDir, rel), []byte(`{"findings":[]}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("findings/developmental.json")
	write("findings/line_section_01.json")
	write("findings/line_section_02.json")
	write("canon/continuity.json")
	write("canon/collisions.json")

	var buf bytes.Buffer
	if code := RunRounds([]string{"archive", dir, "line"}, &buf); code != 0 {
		t.Fatalf("archive line: %s", buf.String())
	}
	// Round number comes from state, and only this pass's files are taken.
	got, err := os.ReadDir(filepath.Join(stateDir, "rounds", "line-round2"))
	if err != nil {
		t.Fatalf("expected line-round2 archive: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("line archive holds %d files, want 2 (not developmental's)", len(got))
	}

	// Continuity's artifacts live under canon/, not findings/ -- the
	// non-obvious case worth pinning.
	buf.Reset()
	if code := RunRounds([]string{"archive", dir, "continuity"}, &buf); code != 0 {
		t.Fatalf("archive continuity: %s", buf.String())
	}
	got, err = os.ReadDir(filepath.Join(stateDir, "rounds", "continuity-round2"))
	if err != nil || len(got) != 2 {
		t.Errorf("continuity archive should hold continuity.json + collisions.json, got %v (%v)", got, err)
	}

	buf.Reset()
	if code := RunRounds([]string{"archive", dir, "intake"}, &buf); code == 0 {
		t.Error("intake is not a pass and must be rejected")
	}

	buf.Reset()
	RunRounds([]string{"list", dir}, &buf)
	if !strings.Contains(buf.String(), "line-round2") || !strings.Contains(buf.String(), "continuity-round2") {
		t.Errorf("list should show both archives, got: %s", buf.String())
	}
}
