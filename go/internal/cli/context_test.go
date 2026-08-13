package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// `redliner context` is a composite over existing operations, added to cut
// coordinator round trips. Its contract is "one call answers where the
// manuscript stands", so the test that matters is that every field a
// caller would otherwise have run a separate command for is present.
func TestContext_AnswersEverythingTheOrientationCallsDid(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer
	if code := RunState([]string{"init", dir, "fiction"}, &out); code != 0 {
		t.Skipf("state init unavailable in this environment: %s", out.String())
	}

	out.Reset()
	if code := RunContext([]string{dir}, &out); code != 0 {
		t.Fatalf("context failed: %s", out.String())
	}

	var got map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("context output is not JSON: %v\n%s", err, out.String())
	}
	for _, key := range []string{
		"manuscript_dir", "state", "domain", "sections", "diff", "canon", "files_present",
	} {
		if _, ok := got[key]; !ok {
			t.Errorf("missing %q — a caller would still need a separate command for it", key)
		}
	}

	// The domain block must carry the vocabulary, or callers still have to
	// run `domain show` -- the duplicated call this command exists to kill.
	domain, _ := got["domain"].(map[string]interface{})
	for _, key := range []string{"developmental_categories", "line_categories", "draft_stages", "brief_fields"} {
		if _, ok := domain[key]; !ok {
			t.Errorf("domain block missing %q", key)
		}
	}
}

func TestContext_UsageWithoutArgs(t *testing.T) {
	var out bytes.Buffer
	if code := RunContext(nil, &out); code == 0 {
		t.Error("expected non-zero exit with no manuscript_dir")
	}
	if !strings.Contains(out.String(), "redliner context") {
		t.Errorf("expected usage, got %q", out.String())
	}
}
