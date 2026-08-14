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
