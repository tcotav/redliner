package schemas

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DomainError mirrors domain_loader.py's DomainError.
type DomainError struct {
	Message string
}

func (e *DomainError) Error() string { return e.Message }

// Domain is a parsed domain.json. Represented as a generic map (mirroring
// how domain_loader.py just returns the parsed dict) rather than a fixed
// struct: domain.json carries fields this engine doesn't validate (e.g.
// fiction's "unit_name") but `domain show` still has to round-trip all
// of them, not just the ones listed in requiredKeys. Structural
// equivalence with the Python output, not byte-for-byte key order, is
// the actual requirement -- see TODO.md's JSON-key-order porting hazard.
type Domain map[string]interface{}

var requiredKeys = []string{
	"name", "display_name", "round_tracked_phase",
	"developmental_categories", "line_categories",
	"continuity", "brief_fields", "draft_stages",
}
var requiredContinuityKeys = []string{"entity_types", "sources", "categories"}
var requiredBriefFieldKeys = []string{"name", "label", "prompt"}
var requiredDraftStageKeys = []string{"name", "implication"}

// FindDomainsDir locates the domains/ directory without assuming a fixed
// nesting depth relative to the running binary. bin/redliner and
// cowork/redliner sit at different depths under their respective plugin
// roots -- bin/'s domains/ is a sibling of bin/ itself (one level above
// the binary), but cowork/ *is* its own plugin root once installed, so
// domains/ sits directly beside the binary (same directory). A fixed
// `parent.parent.parent` assumption is exactly the bug already found and
// fixed in domain_loader.py's Python original (see TODO.md's "v1 plan"
// note) -- this searches instead of assuming, so the same mistake can't
// recur in the port.
//
// Order: $REDLINER_DOMAINS_DIR override, else search near the binary's
// own directory, else a clear error naming everywhere it looked.
func FindDomainsDir() (string, error) {
	if override := os.Getenv("REDLINER_DOMAINS_DIR"); override != "" {
		if info, err := os.Stat(override); err == nil && info.IsDir() {
			return override, nil
		}
		return "", fmt.Errorf("REDLINER_DOMAINS_DIR=%s does not exist or is not a directory", override)
	}

	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("could not determine binary location: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return findDomainsDirFrom(filepath.Dir(exe))
}

// findDomainsDirFrom is FindDomainsDir's search, factored out so the
// walk-up logic is testable without faking os.Executable() (which Go
// doesn't support overriding). Checks startDir and up to 3 ancestors for
// a "domains" subdirectory, covering both known plugin-root depths:
// bin/'s domains/ is one level above the binary (ancestor 1), cowork/'s
// is beside it (ancestor 0, since cowork/ is its own plugin root).
func findDomainsDirFrom(startDir string) (string, error) {
	var tried []string
	dir := startDir
	for i := 0; i < 4; i++ {
		candidate := filepath.Join(dir, "domains")
		tried = append(tried, candidate)
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf(
		"no domains/ directory found near %s (tried: %s) -- set REDLINER_DOMAINS_DIR to override",
		startDir, strings.Join(tried, ", "),
	)
}

func DomainPath(domainsDir, name string) string {
	return filepath.Join(domainsDir, name, "domain.json")
}

// ListDomains mirrors domain_loader.py's list_domains: every subdirectory
// of domainsDir that has a domain.json, sorted by name. Returns an empty
// (not nil) slice if domainsDir doesn't exist, matching Python's `if not
// DOMAINS_DIR.is_dir(): return []`.
func ListDomains(domainsDir string) []string {
	entries, err := os.ReadDir(domainsDir)
	if err != nil {
		return []string{}
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if info, err := os.Stat(filepath.Join(domainsDir, e.Name(), "domain.json")); err == nil && !info.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names
}

// LoadDomain reads and validates one domain's config. Mirrors
// domain_loader.py's load_domain, including its required-key checks.
func LoadDomain(domainsDir, name string) (Domain, error) {
	path := DomainPath(domainsDir, name)
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			available := ListDomains(domainsDir)
			avail := "(none)"
			if len(available) > 0 {
				avail = strings.Join(available, ", ")
			}
			return nil, &DomainError{Message: fmt.Sprintf("No domain config at %s. Available domains: %s", path, avail)}
		}
		return nil, err
	}

	var domain Domain
	if err := json.Unmarshal(raw, &domain); err != nil {
		return nil, &DomainError{Message: fmt.Sprintf("%s: invalid JSON: %v", path, err)}
	}

	if missing := missingKeysMap(domain, requiredKeys); len(missing) > 0 {
		return nil, &DomainError{Message: fmt.Sprintf("%s: missing required keys %s", path, pyList(missing))}
	}

	continuity, ok := domain["continuity"].(map[string]interface{})
	if !ok {
		return nil, &DomainError{Message: fmt.Sprintf("%s: 'continuity' must be an object", path)}
	}
	if missing := missingKeysMap(continuity, requiredContinuityKeys); len(missing) > 0 {
		return nil, &DomainError{Message: fmt.Sprintf("%s: 'continuity' missing required keys %s", path, pyList(missing))}
	}

	briefFields, ok := domain["brief_fields"].([]interface{})
	if !ok || len(briefFields) == 0 {
		return nil, &DomainError{Message: fmt.Sprintf("%s: 'brief_fields' must be a non-empty list", path)}
	}
	for _, item := range briefFields {
		field, ok := item.(map[string]interface{})
		if !ok {
			return nil, &DomainError{Message: fmt.Sprintf("%s: brief_fields entry missing keys %s", path, pyList(requiredBriefFieldKeys))}
		}
		if missing := missingKeysMap(field, requiredBriefFieldKeys); len(missing) > 0 {
			return nil, &DomainError{Message: fmt.Sprintf("%s: brief_fields entry missing keys %s", path, pyList(missing))}
		}
	}

	draftStages, ok := domain["draft_stages"].([]interface{})
	if !ok || len(draftStages) == 0 {
		return nil, &DomainError{Message: fmt.Sprintf("%s: 'draft_stages' must be a non-empty list", path)}
	}
	for _, item := range draftStages {
		stage, ok := item.(map[string]interface{})
		if !ok {
			return nil, &DomainError{Message: fmt.Sprintf("%s: draft_stages entry missing keys %s", path, pyList(requiredDraftStageKeys))}
		}
		if missing := missingKeysMap(stage, requiredDraftStageKeys); len(missing) > 0 {
			return nil, &DomainError{Message: fmt.Sprintf("%s: draft_stages entry missing keys %s", path, pyList(missing))}
		}
	}

	return domain, nil
}

func missingKeysMap(m map[string]interface{}, required []string) []string {
	var missing []string
	for _, k := range required {
		if _, ok := m[k]; !ok {
			missing = append(missing, k)
		}
	}
	sort.Strings(missing)
	return missing
}

// --- Convenience accessors used by the CLI/MCP layers (Phase 3+) ---

func (d Domain) String(key string) string {
	s, _ := d[key].(string)
	return s
}

func (d Domain) stringSlice(key string) []string {
	raw, _ := d[key].([]interface{})
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// StringSet reads a []string-shaped field as a set, for the category-
// membership checks canon_schema.go/findings_schema.go's validators take
// as parameters (e.g. domain.StringSet("developmental_categories")).
func (d Domain) StringSet(key string) map[string]bool {
	out := map[string]bool{}
	for _, s := range d.stringSlice(key) {
		out[s] = true
	}
	return out
}

// Continuity returns the nested "continuity" object as a Domain too, so
// the same StringSet/String accessors work on it
// (domain.Continuity().StringSet("entity_types")).
func (d Domain) Continuity() Domain {
	c, _ := d["continuity"].(map[string]interface{})
	return Domain(c)
}

// RoundTrackedPhase is the domain's iterative phase (fiction:
// "developmental") -- entering it from elsewhere increments
// State.DevelopmentalRound. See redliner_state.py's cmd_phase.
func (d Domain) RoundTrackedPhase() string {
	return d.String("round_tracked_phase")
}

// DraftStageNames returns the domain's draft_stages `name` values, in
// declaration order. Used to validate `redliner state stage` against the
// domain's own vocabulary rather than a hardcoded list -- a design doc's
// stages aren't a novel's.
func (d Domain) DraftStageNames() []string {
	raw, ok := d["draft_stages"].([]interface{})
	if !ok {
		return nil
	}
	var out []string
	for _, entry := range raw {
		stage, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}
		if name, ok := stage["name"].(string); ok && name != "" {
			out = append(out, name)
		}
	}
	return out
}

// DraftStageImplication returns the severity implication recorded for a
// named stage, or "" if the stage isn't in this domain.
func (d Domain) DraftStageImplication(name string) string {
	raw, ok := d["draft_stages"].([]interface{})
	if !ok {
		return ""
	}
	for _, entry := range raw {
		stage, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}
		if n, _ := stage["name"].(string); n == name {
			impl, _ := stage["implication"].(string)
			return impl
		}
	}
	return ""
}
