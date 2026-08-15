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

// The round counter only advances on entering the developmental phase,
// but continuity is explicitly not phase-gated and line can run more
// than once in a round. Two archives of the same pass inside one round
// therefore resolve to the same directory name, and the second used to
// overwrite the first -- destroying exactly the "before" this command
// exists to preserve, while reporting success both times.
func TestRoundsArchive_SecondArchiveInOneRoundKeepsTheFirst(t *testing.T) {
	dir := t.TempDir()
	stateDir := filepath.Join(dir, ".redliner")
	if err := os.MkdirAll(filepath.Join(stateDir, "canon"), 0o755); err != nil {
		t.Fatal(err)
	}
	state := `{"manuscript_dir":"` + dir + `","domain":"fiction","phase":"developmental",` +
		`"developmental_round":3,"section_fingerprints":{},"created_at":"x"}`
	if err := os.WriteFile(filepath.Join(stateDir, "state.json"), []byte(state), 0o644); err != nil {
		t.Fatal(err)
	}
	continuityPath := filepath.Join(stateDir, "canon", "continuity.json")
	writeContinuity := func(note string) {
		body := `{"contradictions":[{"id":"cont-001","note":"` + note + `"}]}`
		if err := os.WriteFile(continuityPath, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	var buf bytes.Buffer
	writeContinuity("first pass")
	if code := RunRounds([]string{"archive", dir, "continuity"}, &buf); code != 0 {
		t.Fatalf("first archive: %s", buf.String())
	}
	buf.Reset()
	writeContinuity("second pass")
	if code := RunRounds([]string{"archive", dir, "continuity"}, &buf); code != 0 {
		t.Fatalf("second archive: %s", buf.String())
	}

	// Both rounds' worth of continuity must still be on disk, and the
	// original must still say what it said.
	first, err := os.ReadFile(filepath.Join(stateDir, "rounds", "continuity-round3", "continuity.json"))
	if err != nil {
		t.Fatalf("the first archive is gone: %v", err)
	}
	if !strings.Contains(string(first), "first pass") {
		t.Errorf("the first archive was overwritten: %s", first)
	}
	second, err := os.ReadFile(filepath.Join(stateDir, "rounds", "continuity-round3.2", "continuity.json"))
	if err != nil {
		t.Fatalf("the second archive should land beside it, not on top of it: %v", err)
	}
	if !strings.Contains(string(second), "second pass") {
		t.Errorf("the second archive holds the wrong content: %s", second)
	}
}
