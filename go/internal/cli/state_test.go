package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tcotav/redliner/go/internal/schemas"
)

func TestStateStage_ValidatesAgainstDomainVocabulary(t *testing.T) {
	dir := t.TempDir()
	domains, _ := filepath.Abs(filepath.Join("..", "..", "..", "domains"))
	t.Setenv("REDLINER_DOMAINS_DIR", domains)

	var buf bytes.Buffer
	if code := RunState([]string{"init", dir, "fiction"}, &buf); code != 0 {
		t.Skipf("state init unavailable: %s", buf.String())
	}

	// A stage the domain doesn't define is rejected, and the message lists
	// the real vocabulary rather than a hardcoded list.
	buf.Reset()
	if code := RunState([]string{"stage", dir, "not-a-stage"}, &buf); code == 0 {
		t.Error("an undefined draft stage should be rejected")
	}
	if !strings.Contains(buf.String(), "exploratory / partial") {
		t.Errorf("rejection should list the domain's stages, got: %s", buf.String())
	}

	buf.Reset()
	if code := RunState([]string{"stage", dir, "revised"}, &buf); code != 0 {
		t.Fatalf("valid stage rejected: %s", buf.String())
	}
	state, err := schemas.LoadState(dir)
	if err != nil || state == nil {
		t.Fatalf("load state: %v", err)
	}
	if state.DraftStage != "revised" {
		t.Errorf("draft_stage = %q, want revised", state.DraftStage)
	}
}

func TestStatePass_CountsPassesNotPhases(t *testing.T) {
	// Pass kinds and phases are different sets: continuity is a real pass
	// that is not phase-gated, and intake is a phase nobody runs a pass of.
	dir := t.TempDir()
	domains, _ := filepath.Abs(filepath.Join("..", "..", "..", "domains"))
	t.Setenv("REDLINER_DOMAINS_DIR", domains)

	var buf bytes.Buffer
	if code := RunState([]string{"init", dir, "fiction"}, &buf); code != 0 {
		t.Skipf("state init unavailable: %s", buf.String())
	}

	buf.Reset()
	if code := RunState([]string{"pass", dir, "continuity"}, &buf); code != 0 {
		t.Errorf("continuity is a real pass and must be accepted: %s", buf.String())
	}
	buf.Reset()
	if code := RunState([]string{"pass", dir, "intake"}, &buf); code == 0 {
		t.Error("intake is a phase, not a pass, and must be rejected")
	}

	buf.Reset()
	RunState([]string{"pass", dir, "developmental"}, &buf)
	buf.Reset()
	RunState([]string{"pass", dir, "developmental"}, &buf)

	state, _ := schemas.LoadState(dir)
	if got := state.Passes["developmental"]; got != 2 {
		t.Errorf("developmental passes = %d, want 2", got)
	}
	if got := state.Passes["continuity"]; got != 1 {
		t.Errorf("continuity passes = %d, want 1", got)
	}
}

func TestStatePublished_SetsTheBoundary(t *testing.T) {
	dir := newOutlineFixture(t, "section_01", "section_02")

	var buf bytes.Buffer
	if code := RunState([]string{"published", dir, "section_01"}, &buf); code != 0 {
		t.Fatalf("exit %d: %s", code, buf.String())
	}
	state, err := schemas.LoadState(dir)
	if err != nil || state == nil {
		t.Fatal(err)
	}
	if state.PublishedThrough != "section_01" {
		t.Errorf("PublishedThrough = %q, want section_01", state.PublishedThrough)
	}
}

func TestStatePublished_RejectsAStemThatIsNotASection(t *testing.T) {
	// A typo here draws no line at all, and the failure looks like the
	// feature being broken rather than the input being wrong.
	dir := newOutlineFixture(t, "section_01")

	var buf bytes.Buffer
	if code := RunState([]string{"published", dir, "section_99"}, &buf); code == 0 {
		t.Fatal("a stem with no matching section file was accepted")
	}
	if !strings.Contains(buf.String(), "section_01") {
		t.Errorf("rejection should name the sections that DO exist: %s", buf.String())
	}
}

func TestStatePublished_NoneClearsIt(t *testing.T) {
	dir := newOutlineFixture(t, "section_01")
	var buf bytes.Buffer
	if code := RunState([]string{"published", dir, "section_01"}, &buf); code != 0 {
		t.Fatalf("set: %s", buf.String())
	}

	buf.Reset()
	if code := RunState([]string{"published", dir, "none"}, &buf); code != 0 {
		t.Fatalf("clear: %s", buf.String())
	}
	state, err := schemas.LoadState(dir)
	if err != nil || state == nil {
		t.Fatal(err)
	}
	if state.PublishedThrough != "" {
		t.Errorf("PublishedThrough = %q after clearing, want empty", state.PublishedThrough)
	}
}
