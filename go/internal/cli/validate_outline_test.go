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
	// design-doc opts out. A stray outline directory under such a
	// manuscript must not crash the walk.
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".redliner"), 0o755); err != nil {
		t.Fatal(err)
	}
	state := `{"manuscript_dir":"` + dir + `","domain":"design-doc","phase":"intake",` +
		`"developmental_round":0,"section_fingerprints":{},"created_at":"x"}`
	if err := os.WriteFile(filepath.Join(dir, ".redliner", "state.json"), []byte(state), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	code := ValidateManuscript(dir, filepath.Join(repoRoot(t), "domains"), &buf)
	if code != 0 && strings.Contains(buf.String(), "outline") {
		t.Errorf("validate failed on outline for a domain that configures none:\n%s", buf.String())
	}
}
