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
