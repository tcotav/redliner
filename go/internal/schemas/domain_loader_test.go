package schemas

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestListAndLoadDomain_RealConfigs(t *testing.T) {
	dir := domainsDir(t)

	names := ListDomains(dir)
	want := []string{"design-doc", "fiction", "serial-fiction"}
	sort.Strings(names)
	if len(names) != len(want) {
		t.Fatalf("ListDomains: got %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Errorf("ListDomains[%d]: got %q, want %q", i, names[i], want[i])
		}
	}

	fiction, err := LoadDomain(dir, "fiction")
	if err != nil {
		t.Fatalf("LoadDomain(fiction): %v", err)
	}
	if fiction.String("round_tracked_phase") != "developmental" {
		t.Errorf("round_tracked_phase: got %q, want developmental", fiction.RoundTrackedPhase())
	}
	devCats := fiction.StringSet("developmental_categories")
	if !devCats["plot"] || !devCats["pacing"] {
		t.Errorf("developmental_categories missing expected entries: %v", devCats)
	}
	continuity := fiction.Continuity()
	entityTypes := continuity.StringSet("entity_types")
	if !entityTypes["character"] || !entityTypes["place"] {
		t.Errorf("continuity entity_types missing expected entries: %v", entityTypes)
	}
}

func TestLoadDomain_Missing(t *testing.T) {
	dir := domainsDir(t)
	_, err := LoadDomain(dir, "not-a-real-domain")
	if err == nil {
		t.Fatal("expected DomainError for a nonexistent domain")
	}
	if _, ok := err.(*DomainError); !ok {
		t.Errorf("expected *DomainError, got %T: %v", err, err)
	}
}

func TestFindDomainsDir_EnvOverride(t *testing.T) {
	dir := domainsDir(t)
	t.Setenv("REDLINER_DOMAINS_DIR", dir)
	got, err := FindDomainsDir()
	if err != nil {
		t.Fatalf("FindDomainsDir: %v", err)
	}
	wantAbs, _ := filepath.Abs(dir)
	gotAbs, _ := filepath.Abs(got)
	if gotAbs != wantAbs {
		t.Errorf("FindDomainsDir: got %s, want %s", gotAbs, wantAbs)
	}
}

func TestFindDomainsDir_BadOverride(t *testing.T) {
	t.Setenv("REDLINER_DOMAINS_DIR", "/definitely/not/a/real/path")
	if _, err := FindDomainsDir(); err == nil {
		t.Fatal("expected an error for a nonexistent REDLINER_DOMAINS_DIR override")
	}
}

// TestFindDomainsDirFrom_BothPluginRootDepths exercises the actual
// walk-up search (not just the env override every other test uses),
// covering both real layouts: bin/'s domains/ is one level above the
// binary's own directory, but cowork/ is its own plugin root once
// installed, so its domains/ sits beside the binary instead. This is
// the exact ambiguity that broke domain_loader.py's original
// three-levels-up assumption (see TODO.md's "v1 plan" note) -- the
// fixed depth this replaces it with must handle both, not just one.
func TestFindDomainsDirFrom_BothPluginRootDepths(t *testing.T) {
	t.Run("cowork-style: domains beside the binary", func(t *testing.T) {
		root := t.TempDir()
		domains := filepath.Join(root, "domains")
		mkdirT(t, domains)
		binDir := root // binary sits directly in the plugin root

		got, err := findDomainsDirFrom(binDir)
		if err != nil {
			t.Fatalf("findDomainsDirFrom: %v", err)
		}
		if got != domains {
			t.Errorf("got %s, want %s", got, domains)
		}
	})

	t.Run("bin-style: domains one level above the binary", func(t *testing.T) {
		root := t.TempDir()
		domains := filepath.Join(root, "domains")
		mkdirT(t, domains)
		binDir := filepath.Join(root, "bin")
		mkdirT(t, binDir)

		got, err := findDomainsDirFrom(binDir)
		if err != nil {
			t.Fatalf("findDomainsDirFrom: %v", err)
		}
		if got != domains {
			t.Errorf("got %s, want %s", got, domains)
		}
	})

	t.Run("no domains/ anywhere nearby: clear error naming what it tried", func(t *testing.T) {
		root := t.TempDir()
		binDir := filepath.Join(root, "bin")
		mkdirT(t, binDir)

		_, err := findDomainsDirFrom(binDir)
		if err == nil {
			t.Fatal("expected an error when no domains/ exists near the search start")
		}
		if !strings.Contains(err.Error(), binDir) {
			t.Errorf("error should name the directory it searched from: %v", err)
		}
	})
}

func mkdirT(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", dir, err)
	}
}

func writeDomain(t *testing.T, domainsDir, name, body string) {
	t.Helper()
	dir := filepath.Join(domainsDir, name)
	mkdirT(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "domain.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// minimalDomainJSON builds a domain.json that satisfies every existing
// required key, with `extra` spliced in as additional top-level JSON
// (must end with a comma, or be empty).
func minimalDomainJSON(name, extra string) string {
	return `{
		"name": "` + name + `",
		"display_name": "` + name + `",
		"round_tracked_phase": "developmental",
		` + extra + `
		"developmental_categories": ["a"],
		"line_categories": ["b"],
		"continuity": {"entity_types": ["character"], "sources": ["narration"], "categories": ["timeline"]},
		"brief_fields": [{"name": "logline", "label": "Logline", "prompt": "?"}],
		"draft_stages": [{"name": "revised", "implication": "both layers"}]
	}`
}

func TestLoadDomain_OutlineBlockIsOptional(t *testing.T) {
	dir := t.TempDir()
	writeDomain(t, dir, "nooutline", minimalDomainJSON("nooutline", ""))

	d, err := LoadDomain(dir, "nooutline")
	if err != nil {
		t.Fatalf("a domain with no outline block must load: %v", err)
	}
	if d.HasOutline() {
		t.Error("HasOutline() true for a domain with no outline block")
	}
	if got := d.OutlineRowFields(); len(got) != 0 {
		t.Errorf("OutlineRowFields() = %v, want empty", got)
	}
	if got := d.OutlineSectionFields(); len(got) != 0 {
		t.Errorf("OutlineSectionFields() = %v, want empty", got)
	}
}

func TestLoadDomain_OutlineFieldsInDeclarationOrder(t *testing.T) {
	dir := t.TempDir()
	outline := `"outline": {
		"row_fields": [
			{"name": "goal", "prompt": "g"},
			{"name": "conflict", "prompt": "c"},
			{"name": "outcome", "prompt": "o"}
		],
		"section_fields": [{"name": "leaves_open", "prompt": "l"}]
	},`
	writeDomain(t, dir, "withoutline", minimalDomainJSON("withoutline", outline))

	d, err := LoadDomain(dir, "withoutline")
	if err != nil {
		t.Fatalf("valid outline block rejected: %v", err)
	}
	if !d.HasOutline() {
		t.Fatal("HasOutline() false for a domain that has one")
	}
	want := []string{"goal", "conflict", "outcome"}
	got := d.OutlineRowFields()
	if len(got) != len(want) {
		t.Fatalf("OutlineRowFields() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("OutlineRowFields()[%d] = %q, want %q (declaration order is load-bearing)", i, got[i], want[i])
		}
	}
	if got := d.OutlineSectionFields(); len(got) != 1 || got[0] != "leaves_open" {
		t.Errorf("OutlineSectionFields() = %v, want [leaves_open]", got)
	}
}

func TestLoadDomain_OutlineBlockMustBeWellFormedIfPresent(t *testing.T) {
	cases := map[string]string{
		"not an object":          `"outline": "goal/conflict/outcome",`,
		"empty row_fields":       `"outline": {"row_fields": []},`,
		"missing row_fields":     `"outline": {"section_fields": [{"name":"x","prompt":"p"}]},`,
		"row field missing name": `"outline": {"row_fields": [{"prompt": "p"}]},`,
	}
	for label, outline := range cases {
		t.Run(label, func(t *testing.T) {
			dir := t.TempDir()
			writeDomain(t, dir, "bad", minimalDomainJSON("bad", outline))
			if _, err := LoadDomain(dir, "bad"); err == nil {
				t.Errorf("malformed outline block (%s) loaded without error", label)
			}
		})
	}
}
