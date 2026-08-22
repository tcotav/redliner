package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeRawOutlineSection(t *testing.T, dir, stem, body string) {
	t.Helper()
	if err := os.MkdirAll(OutlineSectionsDir(dir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(OutlineSectionsDir(dir), stem+".json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestValidate_AcceptsWellFormedOutline(t *testing.T) {
	dir := newOutlineFixture(t, "section_01")
	stale, _ := ComputeOutlineStale(dir)
	writeRawOutlineSection(t, dir, "section_01", `{
		"section": "section_01",
		"section_sha256": "`+stale.CurrentHashes["section_01"]+`",
		"scenes": [{
			"order": 1, "pov": "Mira", "anchor": "The gate was open",
			"goal": "Get inside.", "conflict": "The rotation ran early.", "outcome": "She is seen."
		}]
	}`)

	var buf bytes.Buffer
	code := ValidateManuscript(dir, filepath.Join(repoRoot(t), "domains"), &buf)
	if code != 0 {
		t.Fatalf("valid outline rejected (exit %d):\n%s", code, buf.String())
	}
	if !strings.Contains(buf.String(), "sections/section_01.json") {
		t.Errorf("outline file was not validated at all -- a new file type under .redliner/ must not be silently skipped:\n%s", buf.String())
	}
}

func TestValidate_RejectsJudgmentInOutline(t *testing.T) {
	dir := newOutlineFixture(t, "section_01")
	stale, _ := ComputeOutlineStale(dir)
	writeRawOutlineSection(t, dir, "section_01", `{
		"section": "section_01",
		"section_sha256": "`+stale.CurrentHashes["section_01"]+`",
		"scenes": [{
			"order": 1, "pov": "Mira", "anchor": "The gate was open",
			"goal": "Get inside.", "conflict": "The rotation ran early.", "outcome": "She is seen.",
			"severity": "major"
		}]
	}`)

	var buf bytes.Buffer
	if code := ValidateManuscript(dir, filepath.Join(repoRoot(t), "domains"), &buf); code == 0 {
		t.Fatalf("outline carrying a severity passed validation:\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), "severity") {
		t.Errorf("failure does not name the offending key:\n%s", buf.String())
	}
}

func TestValidate_SkipsOutlineForDomainWithoutOne(t *testing.T) {
	// design-doc opts out. A real, malformed outline section on disk
	// under such a manuscript must still be ignored entirely -- not
	// just "no crash", but no validation performed at all, because the
	// domain never configured this layer.
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".redliner"), 0o755); err != nil {
		t.Fatal(err)
	}
	state := `{"manuscript_dir":"` + dir + `","domain":"design-doc","phase":"intake",` +
		`"developmental_round":0,"section_fingerprints":{},"created_at":"x"}`
	if err := os.WriteFile(filepath.Join(dir, ".redliner", "state.json"), []byte(state), 0o644); err != nil {
		t.Fatal(err)
	}
	writeRawOutlineSection(t, dir, "section_01", `{
		"section": "section_01",
		"section_sha256": "`+strings.Repeat("a", 64)+`",
		"scenes": [{
			"order": 1, "pov": "Mira", "anchor": "The gate was open",
			"severity": "major"
		}]
	}`)

	var buf bytes.Buffer
	code := ValidateManuscript(dir, filepath.Join(repoRoot(t), "domains"), &buf)
	if code != 0 {
		t.Errorf("validate failed on outline for a domain that configures none (exit %d):\n%s", code, buf.String())
	}
}

func TestValidate_OutlineFailureIsNotShortCircuitedByCanon(t *testing.T) {
	// Both a canon observation and an outline section are malformed
	// under the same manuscript. The run must report both, not just
	// whichever validateCanon happened to catch first -- an author
	// fixing findings one round at a time needs the full picture, not
	// a subset that depends on evaluation order.
	dir := newOutlineFixture(t, "section_01")
	stale, _ := ComputeOutlineStale(dir)

	if err := os.MkdirAll(ObservationsDir(dir), 0o755); err != nil {
		t.Fatal(err)
	}
	obsFile := filepath.Join(ObservationsDir(dir), "section_01.json")
	if err := os.WriteFile(obsFile, []byte(`{"facts": "not-a-list"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	writeRawOutlineSection(t, dir, "section_01", `{
		"section": "section_01",
		"section_sha256": "`+stale.CurrentHashes["section_01"]+`",
		"scenes": [{
			"order": 1, "pov": "Mira", "anchor": "The gate was open",
			"goal": "Get inside.", "conflict": "The rotation ran early.", "outcome": "She is seen.",
			"severity": "major"
		}]
	}`)

	var buf bytes.Buffer
	code := ValidateManuscript(dir, filepath.Join(repoRoot(t), "domains"), &buf)
	if code == 0 {
		t.Fatalf("manuscript with a malformed canon observation and a malformed outline section passed validation:\n%s", buf.String())
	}
	out := buf.String()
	if !strings.Contains(out, "canon/observations/section_01.json") {
		t.Errorf("malformed canon observation not reported -- was the walk short-circuited?:\n%s", out)
	}
	if !strings.Contains(out, "sections/section_01.json") {
		t.Errorf("malformed outline section not reported -- was the walk short-circuited?:\n%s", out)
	}
}
