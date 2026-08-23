package cli

import (
	"bytes"
	"os"
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
	// feature being broken rather than the input being wrong. Three real
	// sections, so a single-section fixture couldn't hide a message that
	// only echoes the rejected stem or names just one real section
	// instead of enumerating all of them -- see Task 9's "continuity"
	// assertion, which had this exact blind spot.
	dir := newOutlineFixture(t, "section_01", "section_02", "section_03")

	var buf bytes.Buffer
	if code := RunState([]string{"published", dir, "section_99"}, &buf); code == 0 {
		t.Fatal("a stem with no matching section file was accepted")
	}
	out := buf.String()
	for _, stem := range []string{"section_01", "section_02", "section_03"} {
		if !strings.Contains(out, stem) {
			t.Errorf("rejection should name the sections that DO exist, missing %s: %s", stem, out)
		}
	}
	if strings.Contains(out, "section_99") == false {
		t.Errorf("rejection should still echo the rejected stem: %s", out)
	}
	if !strings.Contains(out, "section_01, section_02, section_03") {
		t.Errorf("rejection should list real sections joined by \", \", got: %s", out)
	}
}

func TestStatePublished_NoneIsANoOpWhenNothingWasSet(t *testing.T) {
	// A never-published serial (or any novel) lands here constantly:
	// clearing an already-clear boundary must be a clean no-op, not an
	// error, and not a confusing claim about clearing something that was
	// never set.
	dir := newOutlineFixture(t, "section_01")

	var buf bytes.Buffer
	if code := RunState([]string{"published", dir, "none"}, &buf); code != 0 {
		t.Fatalf("clearing an unset boundary should be a no-op, not fail: %s", buf.String())
	}
	state, err := schemas.LoadState(dir)
	if err != nil || state == nil {
		t.Fatal(err)
	}
	if state.PublishedThrough != "" {
		t.Errorf("PublishedThrough = %q, want empty (nothing was ever set)", state.PublishedThrough)
	}
}

func TestStatePublished_RejectsSectionCollision(t *testing.T) {
	// section_01 exists under both .txt and .md -- SectionCollisionError,
	// not "no such section". The two failures need different fixes from
	// the author (delete a duplicate file vs. fix a typo), so the message
	// must actually distinguish them rather than falling through to the
	// generic "No section ... Sections are:" rejection.
	dir := newOutlineFixture(t, "section_01")
	if err := os.WriteFile(filepath.Join(dir, "section_01.md"), []byte("section_01 body text\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if code := RunState([]string{"published", dir, "section_01"}, &buf); code == 0 {
		t.Fatal("a colliding section stem was accepted")
	}
	out := buf.String()
	if !strings.Contains(out, "Ambiguous section files") {
		t.Errorf("collision should be reported as a collision, not a generic missing-section error: %s", out)
	}
	if strings.Contains(out, "Sections are:") {
		t.Errorf("collision should not fall through to the generic rejection path: %s", out)
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
