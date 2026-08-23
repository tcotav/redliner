# Scene Outliner Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a third redliner layer that records each manuscript section's scenes as goal/conflict/outcome rows, readable by the author as `Outline.md` and consumed by the developmental editor as a structural spine.

**Architecture:** Modeled on the existing continuity layer, not on a phase. One agent per section writes `.redliner/outline/sections/<stem>.json`; staleness is tracked per section by SHA-256 in the same way `canon stale` does it; a deterministic join builds `outline.json` and a deterministic renderer builds `Outline.md`. Every run that changes content archives a version under `.redliner/outline/versions/v<N>/`.

**Tech Stack:** Go 1.x (`go/internal/{schemas,cli,mcpserver}`), the `modelcontextprotocol/go-sdk` MCP server, JSON config under `domains/*/domain.json`, Markdown agent files under `agents/`, Markdown skills under `skills/`.

**Spec:** `docs/superpowers/specs/2026-08-22-scene-outliner-design.md`

## Global Constraints

These apply to every task. They are not restatements of taste — each one is a rule this codebase already enforces mechanically.

- **The recorder records; it never judges.** The outline schema has no `note`, `severity`, or `concern` field, and the validator rejects unknown keys outright. Same rule as `canon_schema.go`'s `factRequiredKeys`.
- **The `outline` block in `domain.json` is optional.** `design-doc` has none. `LoadDomain` must accept its absence without error, and every consumer must handle a nil/empty block.
- **`published_through` and `outline_version` in `state.json` are optional** (`omitempty`). Every existing manuscript lacks them, and the harness goldens must not move because of them.
- **The join and the render are deterministic Go code, never agent calls.** If either became a model call the per-run cost would stop being proportional to what changed, which is the spec's central cost argument.
- **Every `redliner <group> <command>` string that appears in any file under `skills/` must have a mapped MCP tool.** `go/internal/mcpserver/frontdoor_parity_test.go` reads the skill files as text and fails otherwise. This guard exists because five commands once shipped with no tool and the Cowork plugin could not complete a pass for two releases.
- **Human-readable output goes in the manuscript directory; machine state goes under `.redliner/`.** `Outline.md` sits beside the sections. `schemas.SectionFiles` globs `section_*` with a `.txt`/`.md` extension, so nothing else in that directory is mistaken for manuscript text.
- **Never delete anything under `.redliner/rounds/` or `.redliner/outline/versions/` without asking the author.**
- **Run `cd go && go test ./...` before every commit.** All four packages pass today; a task is not done if that changes.

---

### Task 1: Optional `outline` block in domain config

**Files:**
- Modify: `go/internal/schemas/domain_loader.go` (add validation after the `draft_stages` block, ~line 178; add accessors after `Continuity()`, ~line 232)
- Test: `go/internal/schemas/domain_loader_test.go`

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces:
  - `func (d Domain) Outline() Domain` — the nested `outline` object, or a nil `Domain` when absent.
  - `func (d Domain) HasOutline() bool`
  - `func (d Domain) OutlineRowFields() []string` — the `row_fields` names, in declaration order.
  - `func (d Domain) OutlineSectionFields() []string` — the `section_fields` names, in declaration order.

The block's shape, used by every later task:

```json
"outline": {
  "row_fields": [
    {"name": "goal", "prompt": "What was the driving character trying to achieve?"}
  ],
  "section_fields": [
    {"name": "leaves_open", "prompt": "What question does the chapter end on?"}
  ]
}
```

`section_fields` is itself optional within the block; `fiction` omits it.

- [ ] **Step 1: Write the failing tests**

Add to `go/internal/schemas/domain_loader_test.go`:

```go
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
		"not an object":        `"outline": "goal/conflict/outcome",`,
		"empty row_fields":     `"outline": {"row_fields": []},`,
		"missing row_fields":   `"outline": {"section_fields": [{"name":"x","prompt":"p"}]},`,
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
```

Add these two helpers to the same test file if they do not already exist (check first — the file may already have equivalents; reuse rather than duplicate):

```go
func writeDomain(t *testing.T, domainsDir, name, body string) {
	t.Helper()
	dir := filepath.Join(domainsDir, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
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
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd go && go test ./internal/schemas/ -run TestLoadDomain_Outline -v`
Expected: FAIL — `d.HasOutline undefined`, `d.OutlineRowFields undefined`, and the malformed cases load without error.

- [ ] **Step 3: Add the validation**

In `go/internal/schemas/domain_loader.go`, add near the other `required*Keys` vars (~line 34):

```go
var requiredOutlineFieldKeys = []string{"name", "prompt"}
```

Then in `LoadDomain`, immediately after the `draft_stages` validation loop and before `return domain, nil`:

```go
	// The outline block is optional -- design-doc has none, and every
	// manuscript created before the outline layer existed has none. Absent
	// is valid; present-but-malformed is not, because a domain whose
	// row_fields are wrong generates a broken agent file and a validator
	// that accepts the wrong shape, and neither failure is visible until
	// an author has paid for a pass.
	if outlineRaw, present := domain["outline"]; present {
		outline, ok := outlineRaw.(map[string]interface{})
		if !ok {
			return nil, &DomainError{Message: fmt.Sprintf("%s: 'outline' must be an object", path)}
		}
		rowFields, ok := outline["row_fields"].([]interface{})
		if !ok || len(rowFields) == 0 {
			return nil, &DomainError{Message: fmt.Sprintf("%s: 'outline.row_fields' must be a non-empty list", path)}
		}
		checkFields := func(key string, entries []interface{}) error {
			for _, item := range entries {
				field, ok := item.(map[string]interface{})
				if !ok {
					return &DomainError{Message: fmt.Sprintf("%s: outline.%s entry missing keys %s", path, key, pyList(requiredOutlineFieldKeys))}
				}
				if missing := missingKeysMap(field, requiredOutlineFieldKeys); len(missing) > 0 {
					return &DomainError{Message: fmt.Sprintf("%s: outline.%s entry missing keys %s", path, key, pyList(missing))}
				}
			}
			return nil
		}
		if err := checkFields("row_fields", rowFields); err != nil {
			return nil, err
		}
		// section_fields is optional within an optional block: fiction has
		// row fields but no section-level one.
		if sectionFieldsRaw, present := outline["section_fields"]; present {
			sectionFields, ok := sectionFieldsRaw.([]interface{})
			if !ok {
				return nil, &DomainError{Message: fmt.Sprintf("%s: 'outline.section_fields' must be a list", path)}
			}
			if err := checkFields("section_fields", sectionFields); err != nil {
				return nil, err
			}
		}
	}
```

- [ ] **Step 4: Add the accessors**

Append after `Continuity()` in the same file:

```go
// Outline returns the nested "outline" object as a Domain, or nil when
// the domain has none. A nil Domain is safe to call the other accessors
// on -- map reads on a nil map return zero values -- which is what lets
// every caller skip an explicit presence check.
func (d Domain) Outline() Domain {
	o, _ := d["outline"].(map[string]interface{})
	return Domain(o)
}

// HasOutline reports whether this domain configures an outline layer at
// all. design-doc does not; fiction and serial-fiction do.
func (d Domain) HasOutline() bool {
	return len(d.Outline()) > 0
}

// OutlineRowFields returns the per-scene field names in declaration
// order. Order is load-bearing: it is the column order of the rendered
// Outline.md and the field order the generated agent file lists.
func (d Domain) OutlineRowFields() []string {
	return d.Outline().namedFields("row_fields")
}

// OutlineSectionFields returns the per-section field names in
// declaration order (serial-fiction's "leaves_open"; empty for fiction).
func (d Domain) OutlineSectionFields() []string {
	return d.Outline().namedFields("section_fields")
}

func (d Domain) namedFields(key string) []string {
	raw, ok := d[key].([]interface{})
	if !ok {
		return nil
	}
	var out []string
	for _, entry := range raw {
		field, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}
		if name, ok := field["name"].(string); ok && name != "" {
			out = append(out, name)
		}
	}
	return out
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `cd go && go test ./internal/schemas/ -run TestLoadDomain_Outline -v`
Expected: PASS, all three tests.

- [ ] **Step 6: Run the whole suite**

Run: `cd go && go test ./...`
Expected: all four packages `ok`. No golden has moved yet — no `domain.json` has changed.

- [ ] **Step 7: Commit**

```bash
git add go/internal/schemas/domain_loader.go go/internal/schemas/domain_loader_test.go
git commit -m "Accept an optional outline block in domain config"
```

---

### Task 2: `outline` blocks in the fiction and serial-fiction domains

**Files:**
- Modify: `domains/fiction/domain.json`
- Modify: `domains/serial-fiction/domain.json`
- Modify: `go/harness/golden/happy/02_domain_show_fiction.json` (regenerated, not hand-edited)
- Test: `go/internal/schemas/domain_loader_test.go`

**Interfaces:**
- Consumes: `Domain.HasOutline()`, `Domain.OutlineRowFields()`, `Domain.OutlineSectionFields()` from Task 1.
- Produces: real `outline` blocks the later tasks read at runtime.

`design-doc` is deliberately left alone. Do not add a block to it.

- [ ] **Step 1: Write the failing test**

Add to `go/internal/schemas/domain_loader_test.go`:

```go
// TestRealDomains_OutlineConfiguration pins the shipped domains' actual
// outline configuration, because the whole layer's per-domain behavior
// is these three lists and nothing else.
func TestRealDomains_OutlineConfiguration(t *testing.T) {
	domainsDir := filepath.Join(repoRootForSchemas(t), "domains")

	fiction, err := LoadDomain(domainsDir, "fiction")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := fiction.OutlineRowFields(), []string{"goal", "conflict", "outcome"}; !equalStrings(got, want) {
		t.Errorf("fiction row fields = %v, want %v", got, want)
	}
	if got := fiction.OutlineSectionFields(); len(got) != 0 {
		t.Errorf("fiction section fields = %v, want none -- a novel has no installment boundary", got)
	}

	serial, err := LoadDomain(domainsDir, "serial-fiction")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := serial.OutlineRowFields(), []string{"goal", "conflict", "outcome"}; !equalStrings(got, want) {
		t.Errorf("serial-fiction row fields = %v, want %v", got, want)
	}
	if got, want := serial.OutlineSectionFields(), []string{"leaves_open"}; !equalStrings(got, want) {
		t.Errorf("serial-fiction section fields = %v, want %v", got, want)
	}

	designDoc, err := LoadDomain(domainsDir, "design-doc")
	if err != nil {
		t.Fatal(err)
	}
	if designDoc.HasOutline() {
		t.Error("design-doc must have no outline block -- it opts out of the layer")
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
```

If `repoRootForSchemas` does not exist in the schemas test package, add it (mirroring `cli/golden_test.go`'s `repoRoot`):

```go
func repoRootForSchemas(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not determine test file location")
	}
	// go/internal/schemas/x_test.go -> go/internal/schemas -> go/internal -> go -> repo root
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd go && go test ./internal/schemas/ -run TestRealDomains_OutlineConfiguration -v`
Expected: FAIL — `fiction row fields = [], want [goal conflict outcome]`.

- [ ] **Step 3: Add the block to fiction**

In `domains/fiction/domain.json`, insert after the `continuity` object and before `brief_fields`:

```json
  "outline": {
    "row_fields": [
      {"name": "goal", "prompt": "What was the driving character trying to achieve in this scene?"},
      {"name": "conflict", "prompt": "What opposed them?"},
      {"name": "outcome", "prompt": "What changed as a result? If nothing changed, say so plainly."}
    ]
  },
```

- [ ] **Step 4: Add the block to serial-fiction**

In `domains/serial-fiction/domain.json`, insert in the same position:

```json
  "outline": {
    "row_fields": [
      {"name": "goal", "prompt": "What was the driving character trying to achieve in this scene?"},
      {"name": "conflict", "prompt": "What opposed them?"},
      {"name": "outcome", "prompt": "What changed as a result? If nothing changed, say so plainly."}
    ],
    "section_fields": [
      {"name": "leaves_open", "prompt": "What question does this chapter end on? Record the open thread, not a judgment about how strong the hook is."}
    ]
  },
```

- [ ] **Step 5: Run the test to verify it passes**

Run: `cd go && go test ./internal/schemas/ -run TestRealDomains_OutlineConfiguration -v`
Expected: PASS.

- [ ] **Step 6: Regenerate the golden baseline**

`domain show fiction` prints the whole parsed `domain.json`, so `go/harness/golden/happy/02_domain_show_fiction.json` now disagrees with reality. Regenerate rather than hand-editing — the golden's authority comes from being Python-produced.

Run:
```bash
cd go/harness && python3 capture_baseline.py --self-check && python3 capture_baseline.py
```
Expected: `--self-check` reports the harness is consistent, then the capture rewrites `golden/`.

Then confirm only the intended golden moved:
```bash
git diff --stat go/harness/golden/
```
Expected: only `happy/02_domain_show_fiction.json` changed. If any other golden moved, stop and investigate — nothing else in this task should touch state or canon output.

- [ ] **Step 7: Run the whole suite**

Run: `cd go && go test ./...`
Expected: all four packages `ok`, including `TestCLI_MatchesPythonGolden`.

- [ ] **Step 8: Commit**

```bash
git add domains/fiction/domain.json domains/serial-fiction/domain.json \
        go/harness/golden/happy/02_domain_show_fiction.json \
        go/internal/schemas/domain_loader_test.go
git commit -m "Configure the outline layer for fiction and serial-fiction"
```

---

### Task 3: `outline_version` and `published_through` in state

**Files:**
- Modify: `go/internal/schemas/project_state.go` (the `State` struct, ~line 175)
- Test: `go/internal/schemas/project_state_test.go`

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: `State.OutlineVersion int` (JSON `outline_version,omitempty`) and `State.PublishedThrough string` (JSON `published_through,omitempty`).

- [ ] **Step 1: Write the failing test**

Add to `go/internal/schemas/project_state_test.go`:

```go
func TestState_OutlineFieldsAreOptional(t *testing.T) {
	dir := t.TempDir()

	// A state file written before these fields existed must still load,
	// and must not gain the keys when saved back -- the harness goldens
	// compare the whole .redliner/ tree.
	legacy := `{"manuscript_dir":"m","domain":"fiction","phase":"intake",` +
		`"developmental_round":0,"section_fingerprints":{},"created_at":"x"}`
	if err := os.MkdirAll(StateDir(dir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(StatePath(dir), []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}

	state, err := LoadState(dir)
	if err != nil || state == nil {
		t.Fatalf("legacy state failed to load: %v", err)
	}
	if state.OutlineVersion != 0 || state.PublishedThrough != "" {
		t.Errorf("legacy state gained values: version=%d published=%q", state.OutlineVersion, state.PublishedThrough)
	}

	if _, err := SaveState(dir, state); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(StatePath(dir))
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"outline_version", "published_through"} {
		if strings.Contains(string(raw), key) {
			t.Errorf("unset %s was written to state.json -- it must be omitempty", key)
		}
	}
}

func TestState_OutlineFieldsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	state := NewState(dir, "serial-fiction")
	state.OutlineVersion = 4
	state.PublishedThrough = "section_11"
	if _, err := SaveState(dir, state); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadState(dir)
	if err != nil || loaded == nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.OutlineVersion != 4 {
		t.Errorf("OutlineVersion = %d, want 4", loaded.OutlineVersion)
	}
	if loaded.PublishedThrough != "section_11" {
		t.Errorf("PublishedThrough = %q, want section_11", loaded.PublishedThrough)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd go && go test ./internal/schemas/ -run TestState_Outline -v`
Expected: FAIL — `state.OutlineVersion undefined`.

- [ ] **Step 3: Add the fields**

In `go/internal/schemas/project_state.go`, add to the `State` struct after `Passes`:

```go
	// OutlineVersion is the monotonic counter for the outline layer's own
	// version archive under .redliner/outline/versions/v<N>/. Deliberately
	// NOT DevelopmentalRound: the outline runs an order of magnitude more
	// often than rounds turn over (the author's loop is write a chapter,
	// outline, write the next, outline -- a loop that need never run
	// `assess`), so keying versions to the round counter would produce no
	// history at all for the layer's primary workflow.
	//
	// Optional, same reasoning as DraftStage: zero means "no versions
	// archived yet" and the key stays out of state written before this
	// existed.
	OutlineVersion int `json:"outline_version,omitempty"`

	// PublishedThrough names the last section already released to readers
	// (e.g. "section_11"). Serial fiction has a constraint a novel does
	// not: once a chapter goes out, it is fixed, so a scene above that
	// line cannot be moved or cut -- which is the single most load-bearing
	// fact in a view whose purpose is deciding what to move and what to
	// cut. Used here only to draw a visible line in Outline.md; see
	// TODO.md, "The developmental pass doesn't know which chapters are
	// locked", for the larger use that is deliberately not built yet.
	//
	// A section boundary, not a scene one: publication happens per
	// installment, and there is no such thing as half a chapter being
	// live. Empty/absent means nothing is published -- correct for
	// `fiction`, and for a serial being drafted before launch.
	PublishedThrough string `json:"published_through,omitempty"`
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd go && go test ./internal/schemas/ -run TestState_Outline -v`
Expected: PASS.

- [ ] **Step 5: Run the whole suite**

Run: `cd go && go test ./...`
Expected: all `ok`. The goldens must not move — that is what the `omitempty` test is guarding.

- [ ] **Step 6: Commit**

```bash
git add go/internal/schemas/project_state.go go/internal/schemas/project_state_test.go
git commit -m "Add outline_version and published_through to state"
```

---

### Task 4: The outline section validator

**Files:**
- Create: `go/internal/schemas/outline_schema.go`
- Test: `go/internal/schemas/outline_schema_test.go`

**Interfaces:**
- Consumes: `Domain.OutlineRowFields()`, `Domain.OutlineSectionFields()` from Task 1.
- Produces: `func ValidateOutlineSection(reportRaw interface{}, rowFields, sectionFields []string) []string` — returns human-readable error strings, empty slice when valid. Same contract as `ValidateObservations`.

The validated shape (row fields shown are fiction's; the validator takes them as parameters):

```json
{
  "section": "section_03",
  "section_sha256": "<64 hex chars>",
  "leaves_open": "Whether the guard reports her.",
  "scenes": [
    {
      "order": 1,
      "pov": "Mira",
      "anchor": "The gate was already open when she",
      "goal": "Get inside the compound before the shift change.",
      "conflict": "The guard rotation ran early; she has no cover story.",
      "outcome": "She gets in, but is seen — the guard now knows her face."
    }
  ]
}
```

- [ ] **Step 1: Write the failing tests**

Create `go/internal/schemas/outline_schema_test.go`:

```go
package schemas

import (
	"strings"
	"testing"
)

var testRowFields = []string{"goal", "conflict", "outcome"}

func validOutlineSection() map[string]interface{} {
	return map[string]interface{}{
		"section":        "section_03",
		"section_sha256": strings.Repeat("a", 64),
		"scenes": []interface{}{
			map[string]interface{}{
				"order":    float64(1),
				"pov":      "Mira",
				"anchor":   "The gate was already open when she",
				"goal":     "Get inside before the shift change.",
				"conflict": "The rotation ran early.",
				"outcome":  "She gets in, but is seen.",
			},
		},
	}
}

func TestValidateOutlineSection_AcceptsValid(t *testing.T) {
	if errs := ValidateOutlineSection(validOutlineSection(), testRowFields, nil); len(errs) != 0 {
		t.Errorf("valid section rejected: %v", errs)
	}
}

func TestValidateOutlineSection_RejectsJudgmentKeys(t *testing.T) {
	// The point of the whole schema: the recorder has nowhere to put an
	// opinion. Same rule canon_schema.go enforces on facts.
	for _, key := range []string{"note", "severity", "concern", "suggestion"} {
		report := validOutlineSection()
		report["scenes"].([]interface{})[0].(map[string]interface{})[key] = "this scene is weak"
		errs := ValidateOutlineSection(report, testRowFields, nil)
		if len(errs) == 0 {
			t.Errorf("scene with %q key accepted -- the recorder must not be able to judge", key)
			continue
		}
		if !strings.Contains(strings.Join(errs, " "), key) {
			t.Errorf("error for %q key does not name it: %v", key, errs)
		}
	}
}

func TestValidateOutlineSection_RequiresEveryConfiguredRowField(t *testing.T) {
	for _, field := range testRowFields {
		report := validOutlineSection()
		delete(report["scenes"].([]interface{})[0].(map[string]interface{}), field)
		if errs := ValidateOutlineSection(report, testRowFields, nil); len(errs) == 0 {
			t.Errorf("scene missing %q accepted", field)
		}
	}
}

func TestValidateOutlineSection_RejectsBlankRowField(t *testing.T) {
	report := validOutlineSection()
	report["scenes"].([]interface{})[0].(map[string]interface{})["outcome"] = "   "
	if errs := ValidateOutlineSection(report, testRowFields, nil); len(errs) == 0 {
		t.Error(`blank outcome accepted -- "nothing changed" must be written out, not left empty`)
	}
}

func TestValidateOutlineSection_OrderMustBeSequentialFromOne(t *testing.T) {
	report := validOutlineSection()
	scenes := report["scenes"].([]interface{})
	second := map[string]interface{}{
		"order": float64(3), "pov": "Mira", "anchor": "Later that night",
		"goal": "g", "conflict": "c", "outcome": "o",
	}
	report["scenes"] = append(scenes, second)
	errs := ValidateOutlineSection(report, testRowFields, nil)
	if len(errs) == 0 {
		t.Error("non-sequential order accepted -- order is the row's only identity within a section")
	}
}

func TestValidateOutlineSection_SectionFieldsRequiredWhenConfigured(t *testing.T) {
	report := validOutlineSection()
	errs := ValidateOutlineSection(report, testRowFields, []string{"leaves_open"})
	if len(errs) == 0 {
		t.Error("missing configured section field accepted")
	}

	report["leaves_open"] = "Whether the guard reports her."
	if errs := ValidateOutlineSection(report, testRowFields, []string{"leaves_open"}); len(errs) != 0 {
		t.Errorf("section field present but still rejected: %v", errs)
	}
}

func TestValidateOutlineSection_RejectsUnconfiguredSectionField(t *testing.T) {
	// fiction has no section-level fields; a file carrying one means the
	// wrong domain's agent wrote it.
	report := validOutlineSection()
	report["leaves_open"] = "Whether the guard reports her."
	if errs := ValidateOutlineSection(report, testRowFields, nil); len(errs) == 0 {
		t.Error("section field accepted for a domain that configures none")
	}
}

func TestValidateOutlineSection_RejectsBadHash(t *testing.T) {
	report := validOutlineSection()
	report["section_sha256"] = "not-a-hash"
	if errs := ValidateOutlineSection(report, testRowFields, nil); len(errs) == 0 {
		t.Error("malformed section_sha256 accepted -- staleness detection depends on it")
	}
}

func TestValidateOutlineSection_AllowsEmptyScenes(t *testing.T) {
	// A section can legitimately hold no scenes yet (a stub chapter file).
	// That is a recording, not an error.
	report := validOutlineSection()
	report["scenes"] = []interface{}{}
	if errs := ValidateOutlineSection(report, testRowFields, nil); len(errs) != 0 {
		t.Errorf("empty scenes rejected: %v", errs)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd go && go test ./internal/schemas/ -run TestValidateOutlineSection -v`
Expected: FAIL to compile — `undefined: ValidateOutlineSection`.

- [ ] **Step 3: Write the validator**

Create `go/internal/schemas/outline_schema.go`:

```go
package schemas

import (
	"fmt"
	"regexp"
	"sort"
)

// sceneFixedKeys are the keys every scene row carries regardless of
// domain. The domain's own row_fields are added to this set at
// validation time.
//
// `order` is positional, not a durable id: scene boundaries are the
// recorder's judgment and can shift between runs even on unchanged text,
// so nothing downstream may treat "section_03 scene 2" as denoting the
// same scene it did last week. `anchor` exists for exactly that reason --
// it is how a human finds the scene in the prose when ids cannot be
// trusted.
var sceneFixedKeys = []string{"order", "pov", "anchor"}

var sha256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// ValidateOutlineSection validates one section's recorded scenes.
// rowFields/sectionFields come from the manuscript's domain config
// (Domain.OutlineRowFields / Domain.OutlineSectionFields).
//
// Modeled on ValidateObservations, including the rule that matters most:
// there is no optional-key extension point. A scene carrying a `note` or
// `severity` is rejected outright, because the moment the recorder can
// editorialize it has become a second developmental editor and the
// recorder/judge split this layer depends on is gone.
func ValidateOutlineSection(reportRaw interface{}, rowFields, sectionFields []string) []string {
	report, ok := asObject(reportRaw)
	if !ok {
		return []string{"outline section: not a JSON object"}
	}

	var errors []string

	if isBlank(report["section"]) {
		errors = append(errors, "missing/empty 'section'")
	}
	hash := asString(report["section_sha256"])
	if !sha256Pattern.MatchString(hash) {
		errors = append(errors, fmt.Sprintf("section_sha256 %s is not a 64-character lowercase hex digest", pyRepr(report["section_sha256"])))
	}

	// Section-level keys: the fixed two, plus whatever this domain
	// configures. Anything else is the wrong domain's agent writing here.
	allowedTop := map[string]bool{"section": true, "section_sha256": true, "scenes": true}
	for _, f := range sectionFields {
		allowedTop[f] = true
		if isBlank(report[f]) {
			errors = append(errors, fmt.Sprintf("missing/empty section field %s", pyRepr(f)))
		}
	}
	var extraTop []string
	for k := range report {
		if !allowedTop[k] {
			extraTop = append(extraTop, k)
		}
	}
	if len(extraTop) > 0 {
		sort.Strings(extraTop)
		errors = append(errors, fmt.Sprintf("unexpected keys %s — the outline records, it does not judge", pyList(extraTop)))
	}

	scenesRaw, present := report["scenes"]
	if !present {
		return append(errors, "missing 'scenes'")
	}
	scenes, ok := scenesRaw.([]interface{})
	if !ok {
		return append(errors, "'scenes' is not a list")
	}

	allowedScene := map[string]bool{}
	for _, k := range sceneFixedKeys {
		allowedScene[k] = true
	}
	for _, f := range rowFields {
		allowedScene[f] = true
	}

	for i, sceneRaw := range scenes {
		prefix := fmt.Sprintf("scenes[%d]", i)
		scene, ok := asObject(sceneRaw)
		if !ok {
			errors = append(errors, prefix+": not an object")
			continue
		}

		var missing []string
		for _, k := range sceneFixedKeys {
			if _, present := scene[k]; !present {
				missing = append(missing, k)
			}
		}
		for _, f := range rowFields {
			if _, present := scene[f]; !present {
				missing = append(missing, f)
			}
		}
		if len(missing) > 0 {
			sort.Strings(missing)
			errors = append(errors, fmt.Sprintf("%s: missing keys %s", prefix, pyList(missing)))
		}

		var extra []string
		for k := range scene {
			if !allowedScene[k] {
				extra = append(extra, k)
			}
		}
		if len(extra) > 0 {
			sort.Strings(extra)
			errors = append(errors, fmt.Sprintf("%s: unexpected keys %s — the outline records, it does not judge", prefix, pyList(extra)))
		}

		// Order is 1-based and sequential. It is the row's only identity
		// within a section, so a gap or a repeat makes the join ambiguous.
		order, isNumber := scene["order"].(float64)
		if !isNumber || int(order) != i+1 {
			errors = append(errors, fmt.Sprintf("%s: order %s must be %d (1-based, sequential, matching position)", prefix, pyRepr(scene["order"]), i+1))
		}

		// pov and anchor are always required non-blank; a blank anchor
		// makes the scene unfindable in the prose.
		for _, k := range []string{"pov", "anchor"} {
			if _, present := scene[k]; present && isBlank(scene[k]) {
				errors = append(errors, fmt.Sprintf("%s: missing/empty %s", prefix, pyRepr(k)))
			}
		}
		// A blank row field is not the same as "nothing happened" -- the
		// recorder is required to write that out in words, because an
		// empty outcome and an outcome of "nothing changed" mean opposite
		// things to the author deciding what to cut.
		for _, f := range rowFields {
			if _, present := scene[f]; present && isBlank(scene[f]) {
				errors = append(errors, fmt.Sprintf("%s: missing/empty %s — if nothing changed, record that in words", prefix, pyRepr(f)))
			}
		}
	}

	return errors
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd go && go test ./internal/schemas/ -run TestValidateOutlineSection -v`
Expected: PASS, all nine tests.

- [ ] **Step 5: Run the whole suite and commit**

Run: `cd go && go test ./...`
Expected: all `ok`.

```bash
git add go/internal/schemas/outline_schema.go go/internal/schemas/outline_schema_test.go
git commit -m "Add the outline section validator"
```

---

### Task 5: `redliner outline stale`

**Files:**
- Create: `go/internal/cli/outline.go`
- Modify: `go/internal/cli/dispatch.go` (add the `outline` case and its usage line)
- Test: `go/internal/cli/outline_test.go`

**Interfaces:**
- Consumes: `schemas.SectionFiles`, `schemas.FingerprintSection`, `schemas.StateDir` (all existing).
- Produces:
  - `func OutlineDir(manuscriptDir string) string` → `<dir>/.redliner/outline`
  - `func OutlineSectionsDir(manuscriptDir string) string` → `<dir>/.redliner/outline/sections`
  - `type OutlineStaleResult struct { NeedsRecording, NeverRecorded, ChangedSinceRecording []string; CurrentHashes map[string]string; OrphanedSections []string }`
  - `func ComputeOutlineStale(manuscriptDir string) (OutlineStaleResult, error)`
  - `func RunOutline(args []string, stdout, stderr io.Writer) int`

This deliberately mirrors `ComputeStale` in `canon.go` rather than sharing code with it. The two read different directories and will diverge (the outline layer gains version archiving that continuity has no equivalent of); a shared generic helper would be one abstraction serving two things that only look alike today.

- [ ] **Step 1: Write the failing test**

Create `go/internal/cli/outline_test.go`:

```go
package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// newOutlineFixture builds a manuscript with the given sections and
// returns its directory. Section text is the stem itself, so hashes
// differ per section and are stable across runs.
func newOutlineFixture(t *testing.T, stems ...string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".redliner"), 0o755); err != nil {
		t.Fatal(err)
	}
	state := `{"manuscript_dir":"` + dir + `","domain":"fiction","phase":"developmental",` +
		`"developmental_round":1,"section_fingerprints":{},"created_at":"x"}`
	if err := os.WriteFile(filepath.Join(dir, ".redliner", "state.json"), []byte(state), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, stem := range stems {
		if err := os.WriteFile(filepath.Join(dir, stem+".txt"), []byte(stem+" body text\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// writeOutlineSection writes a per-section outline file with the given
// recorded hash (which may deliberately be stale).
func writeOutlineSection(t *testing.T, dir, stem, hash string) {
	t.Helper()
	if err := os.MkdirAll(OutlineSectionsDir(dir), 0o755); err != nil {
		t.Fatal(err)
	}
	body := map[string]interface{}{
		"section":        stem,
		"section_sha256": hash,
		"scenes":         []interface{}{},
	}
	raw, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(OutlineSectionsDir(dir), stem+".json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestComputeOutlineStale_NeverRecorded(t *testing.T) {
	dir := newOutlineFixture(t, "section_01", "section_02")

	got, err := ComputeOutlineStale(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.NeedsRecording) != 2 {
		t.Fatalf("NeedsRecording = %v, want both sections", got.NeedsRecording)
	}
	if got.NeedsRecording[0] != "section_01" || got.NeedsRecording[1] != "section_02" {
		t.Errorf("NeedsRecording = %v, want sorted [section_01 section_02]", got.NeedsRecording)
	}
	for _, stem := range got.NeedsRecording {
		if len(got.CurrentHashes[stem]) != 64 {
			t.Errorf("CurrentHashes[%s] = %q, want a 64-char digest", stem, got.CurrentHashes[stem])
		}
	}
}

func TestComputeOutlineStale_UnchangedSectionIsSkipped(t *testing.T) {
	dir := newOutlineFixture(t, "section_01")
	first, err := ComputeOutlineStale(dir)
	if err != nil {
		t.Fatal(err)
	}
	writeOutlineSection(t, dir, "section_01", first.CurrentHashes["section_01"])

	got, err := ComputeOutlineStale(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.NeedsRecording) != 0 {
		t.Errorf("NeedsRecording = %v, want empty -- an unchanged section must never be re-recorded", got.NeedsRecording)
	}
}

func TestComputeOutlineStale_ChangedSectionIsFlagged(t *testing.T) {
	dir := newOutlineFixture(t, "section_01")
	writeOutlineSection(t, dir, "section_01", "0000000000000000000000000000000000000000000000000000000000000000")

	got, err := ComputeOutlineStale(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.ChangedSinceRecording) != 1 || got.ChangedSinceRecording[0] != "section_01" {
		t.Errorf("ChangedSinceRecording = %v, want [section_01]", got.ChangedSinceRecording)
	}
	if len(got.NeverRecorded) != 0 {
		t.Errorf("NeverRecorded = %v, want empty", got.NeverRecorded)
	}
}

func TestComputeOutlineStale_OrphanedSectionIsReported(t *testing.T) {
	// A cut section's scenes must not still appear in the outline.
	dir := newOutlineFixture(t, "section_01")
	writeOutlineSection(t, dir, "section_99", "0000000000000000000000000000000000000000000000000000000000000000")

	got, err := ComputeOutlineStale(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.OrphanedSections) != 1 || got.OrphanedSections[0] != "section_99" {
		t.Errorf("OrphanedSections = %v, want [section_99]", got.OrphanedSections)
	}
}

func TestRunOutlineStale_PrintsJSON(t *testing.T) {
	dir := newOutlineFixture(t, "section_01")
	var buf bytes.Buffer
	if code := RunOutline([]string{"stale", dir}, &buf, &buf); code != 0 {
		t.Fatalf("exit %d: %s", code, buf.String())
	}
	var parsed OutlineStaleResult
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("stdout is not the JSON shape callers parse: %v\n%s", err, buf.String())
	}
	if len(parsed.NeedsRecording) != 1 {
		t.Errorf("needs_recording = %v, want one section", parsed.NeedsRecording)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd go && go test ./internal/cli/ -run Outline -v`
Expected: FAIL to compile — `undefined: OutlineSectionsDir`, `undefined: ComputeOutlineStale`, `undefined: RunOutline`.

- [ ] **Step 3: Write the command**

Create `go/internal/cli/outline.go`:

```go
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tcotav/redliner/go/internal/schemas"
)

const outlineUsage = `Usage:
  redliner outline stale    <manuscript_dir>   # which sections need re-recording`

func RunOutline(args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 {
		fmt.Fprintln(stdout, outlineUsage)
		return 1
	}
	command, manuscriptDir := args[0], args[1]
	info, err := os.Stat(manuscriptDir)
	if err != nil || !info.IsDir() {
		fmt.Fprintf(stdout, "No such directory: %s\n", manuscriptDir)
		return 1
	}

	switch command {
	case "stale":
		return cmdOutlineStale(manuscriptDir, stdout)
	default:
		fmt.Fprintf(stdout, "Unknown command %s\n%s\n", pyReprStr(command), outlineUsage)
		return 1
	}
}

// OutlineDir and OutlineSectionsDir are exported for reuse by
// internal/mcpserver, same as ObservationsDir/CanonDir in canon.go.
func OutlineDir(manuscriptDir string) string {
	return filepath.Join(schemas.StateDir(manuscriptDir), "outline")
}

func OutlineSectionsDir(manuscriptDir string) string {
	return filepath.Join(OutlineDir(manuscriptDir), "sections")
}

// loadOutlineSections reads every *.json under outline/sections/, keyed
// by section stem. Mirrors canon.go's loadObservations.
func loadOutlineSections(manuscriptDir string) (map[string]map[string]interface{}, error) {
	dir := OutlineSectionsDir(manuscriptDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]map[string]interface{}{}, nil
		}
		return nil, err
	}

	out := map[string]map[string]interface{}{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		var parsed map[string]interface{}
		if err := json.Unmarshal(raw, &parsed); err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		out[stemOfPath(e.Name())] = parsed
	}
	return out, nil
}

// OutlineStaleResult is the JSON shape the run skill reads to decide
// which sections to Task the outliner agent on. Field names deliberately
// differ from canon's ("recording", not "extraction") so a skill author
// reading either output knows which layer produced it.
type OutlineStaleResult struct {
	NeedsRecording        []string          `json:"needs_recording"`
	NeverRecorded         []string          `json:"never_recorded"`
	ChangedSinceRecording []string          `json:"changed_since_recording"`
	CurrentHashes         map[string]string `json:"current_hashes"`
	OrphanedSections      []string          `json:"orphaned_sections"`
}

// ComputeOutlineStale is the whole reason this layer can be re-run after
// every chapter: it costs one agent call per section whose text actually
// moved, and nothing for the rest.
//
// Deliberately a parallel implementation of canon.go's ComputeStale
// rather than a shared generic over both directories. The two read
// different trees and are already diverging (this layer archives a
// version per run; continuity has no equivalent), and one abstraction
// serving two things that merely look alike is how the next change to
// either becomes a change to both.
func ComputeOutlineStale(manuscriptDir string) (OutlineStaleResult, error) {
	recorded, err := loadOutlineSections(manuscriptDir)
	if err != nil {
		return OutlineStaleResult{}, err
	}

	sections, err := schemas.SectionFiles(manuscriptDir)
	if err != nil {
		return OutlineStaleResult{}, err
	}

	var missing, stale []string
	currentHashes := map[string]string{}
	sectionStems := map[string]bool{}

	for _, path := range sections {
		stem := stemOfPath(path)
		sectionStems[stem] = true
		fp, err := schemas.FingerprintSection(path)
		if err != nil {
			return OutlineStaleResult{}, err
		}
		existing, ok := recorded[stem]
		if !ok {
			missing = append(missing, stem)
			currentHashes[stem] = fp.SHA256
			continue
		}
		recordedHash, _ := existing["section_sha256"].(string)
		if recordedHash != fp.SHA256 {
			stale = append(stale, stem)
			currentHashes[stem] = fp.SHA256
		}
	}

	needs := append(append([]string{}, missing...), stale...)
	sort.Strings(needs)

	var orphaned []string
	for stem := range recorded {
		if !sectionStems[stem] {
			orphaned = append(orphaned, stem)
		}
	}
	sort.Strings(orphaned)

	return OutlineStaleResult{
		NeedsRecording:        orEmptyStrings(needs),
		NeverRecorded:         orEmptyStrings(missing),
		ChangedSinceRecording: orEmptyStrings(stale),
		CurrentHashes:         currentHashes,
		OrphanedSections:      orEmptyStrings(orphaned),
	}, nil
}

func cmdOutlineStale(manuscriptDir string, stdout io.Writer) int {
	result, err := ComputeOutlineStale(manuscriptDir)
	if err != nil {
		if _, ok := err.(*schemas.SectionCollisionError); ok {
			return reportSectionError(err, stdout)
		}
		fmt.Fprintf(stdout, "Error reading outline sections: %v\n", err)
		return 1
	}
	return printJSON(stdout, result)
}
```

- [ ] **Step 4: Wire it into the dispatcher**

In `go/internal/cli/dispatch.go`, add to the `usage` string after the `canon` line:

```
  redliner outline stale|join|render|versions <manuscript_dir>   # scene-level view of the plot
```

and add to the `switch` after the `canon` case:

```go
	case "outline":
		return RunOutline(args[1:], stdout, stderr)
```

(The usage line names all four subcommands now; the remaining three arrive in Tasks 6–8. Naming them here keeps one edit to the usage text rather than four.)

- [ ] **Step 5: Run the tests to verify they pass**

Run: `cd go && go test ./internal/cli/ -run Outline -v`
Expected: PASS, all five tests.

- [ ] **Step 6: Run the whole suite and commit**

Run: `cd go && go test ./...`
Expected: all `ok`.

```bash
git add go/internal/cli/outline.go go/internal/cli/outline_test.go go/internal/cli/dispatch.go
git commit -m "Add redliner outline stale"
```

---

### Task 6: `redliner outline join`

**Files:**
- Modify: `go/internal/cli/outline.go`
- Test: `go/internal/cli/outline_test.go`

**Interfaces:**
- Consumes: `loadOutlineSections`, `OutlineDir` from Task 5.
- Produces:
  - `func OutlinePath(manuscriptDir string) string` → `<dir>/.redliner/outline/outline.json`
  - `func ComputeOutlineJoin(manuscriptDir string) (map[string]interface{}, error)` — the joined document, built from **every** current per-section file.
  - `var ErrNoOutlineSections = errors.New(...)`

The joined shape:

```json
{
  "sections": [
    {"section": "section_01", "section_sha256": "...", "scenes": [...]},
    {"section": "section_02", "section_sha256": "...", "scenes": [...]}
  ],
  "scene_count": 7,
  "published_through": "section_11"
}
```

`published_through` is copied from state and omitted when unset. Sections appear in section order (sorted by stem), which is manuscript order.

- [ ] **Step 1: Write the failing test**

Append to `go/internal/cli/outline_test.go`:

```go
// writeOutlineSectionWithScenes writes a per-section file carrying real
// scene rows, for the join and render tests.
func writeOutlineSectionWithScenes(t *testing.T, dir, stem, hash string, scenes []map[string]interface{}) {
	t.Helper()
	if err := os.MkdirAll(OutlineSectionsDir(dir), 0o755); err != nil {
		t.Fatal(err)
	}
	rows := make([]interface{}, len(scenes))
	for i, s := range scenes {
		rows[i] = s
	}
	body := map[string]interface{}{
		"section":        stem,
		"section_sha256": hash,
		"scenes":         rows,
	}
	raw, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(OutlineSectionsDir(dir), stem+".json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func scene(order int, goal string) map[string]interface{} {
	return map[string]interface{}{
		"order": float64(order), "pov": "Mira", "anchor": "anchor text " + goal,
		"goal": goal, "conflict": "opposition", "outcome": "a change",
	}
}

func TestComputeOutlineJoin_SectionOrderAndCount(t *testing.T) {
	dir := newOutlineFixture(t, "section_01", "section_02")
	stale, err := ComputeOutlineStale(dir)
	if err != nil {
		t.Fatal(err)
	}
	// Written out of order on purpose -- the join must sort, not trust
	// directory iteration order.
	writeOutlineSectionWithScenes(t, dir, "section_02", stale.CurrentHashes["section_02"],
		[]map[string]interface{}{scene(1, "escape")})
	writeOutlineSectionWithScenes(t, dir, "section_01", stale.CurrentHashes["section_01"],
		[]map[string]interface{}{scene(1, "enter"), scene(2, "hide")})

	joined, err := ComputeOutlineJoin(dir)
	if err != nil {
		t.Fatal(err)
	}
	sections := joined["sections"].([]interface{})
	if len(sections) != 2 {
		t.Fatalf("joined %d sections, want 2", len(sections))
	}
	if got := sections[0].(map[string]interface{})["section"]; got != "section_01" {
		t.Errorf("first section = %v, want section_01 (manuscript order, not directory order)", got)
	}
	if got := joined["scene_count"]; got != 3 {
		t.Errorf("scene_count = %v, want 3", got)
	}
}

func TestComputeOutlineJoin_IncludesEverySectionNotJustStaleOnes(t *testing.T) {
	dir := newOutlineFixture(t, "section_01", "section_02")
	stale, err := ComputeOutlineStale(dir)
	if err != nil {
		t.Fatal(err)
	}
	writeOutlineSectionWithScenes(t, dir, "section_01", stale.CurrentHashes["section_01"],
		[]map[string]interface{}{scene(1, "enter")})
	writeOutlineSectionWithScenes(t, dir, "section_02", stale.CurrentHashes["section_02"],
		[]map[string]interface{}{scene(1, "escape")})

	joined, err := ComputeOutlineJoin(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(joined["sections"].([]interface{})) != 2 {
		t.Error("join must rebuild from every current section file, not only re-recorded ones")
	}
}

func TestComputeOutlineJoin_CarriesPublishedThrough(t *testing.T) {
	dir := newOutlineFixture(t, "section_01")
	stale, _ := ComputeOutlineStale(dir)
	writeOutlineSectionWithScenes(t, dir, "section_01", stale.CurrentHashes["section_01"],
		[]map[string]interface{}{scene(1, "enter")})

	joined, err := ComputeOutlineJoin(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, present := joined["published_through"]; present {
		t.Error("published_through present when state does not set it")
	}

	state, err := schemas.LoadState(dir)
	if err != nil {
		t.Fatal(err)
	}
	state.PublishedThrough = "section_01"
	if _, err := schemas.SaveState(dir, state); err != nil {
		t.Fatal(err)
	}

	joined, err = ComputeOutlineJoin(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := joined["published_through"]; got != "section_01" {
		t.Errorf("published_through = %v, want section_01", got)
	}
}

func TestRunOutlineJoin_WritesOutlineJSON(t *testing.T) {
	dir := newOutlineFixture(t, "section_01")
	stale, _ := ComputeOutlineStale(dir)
	writeOutlineSectionWithScenes(t, dir, "section_01", stale.CurrentHashes["section_01"],
		[]map[string]interface{}{scene(1, "enter")})

	var buf bytes.Buffer
	if code := RunOutline([]string{"join", dir}, &buf, &buf); code != 0 {
		t.Fatalf("exit %d: %s", code, buf.String())
	}
	raw, err := os.ReadFile(OutlinePath(dir))
	if err != nil {
		t.Fatalf("join wrote no outline.json: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("outline.json is not valid JSON: %v", err)
	}
	if parsed["scene_count"] != float64(1) {
		t.Errorf("scene_count = %v, want 1", parsed["scene_count"])
	}
}

func TestRunOutlineJoin_ErrorsWithNoSections(t *testing.T) {
	dir := newOutlineFixture(t, "section_01")
	var buf bytes.Buffer
	if code := RunOutline([]string{"join", dir}, &buf, &buf); code == 0 {
		t.Error("join with no recorded sections should fail, not write an empty outline")
	}
	if !strings.Contains(buf.String(), "outline stale") {
		t.Errorf("error should point at the command that fixes it, got: %s", buf.String())
	}
}
```

Add `"strings"` and the `schemas` import to the test file's import block if not already present:

```go
	"strings"

	"github.com/tcotav/redliner/go/internal/schemas"
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd go && go test ./internal/cli/ -run TestComputeOutlineJoin -v`
Expected: FAIL to compile — `undefined: ComputeOutlineJoin`, `undefined: OutlinePath`.

- [ ] **Step 3: Implement the join**

In `go/internal/cli/outline.go`, add `"errors"` to the imports, extend the usage constant:

```go
const outlineUsage = `Usage:
  redliner outline stale    <manuscript_dir>   # which sections need re-recording
  redliner outline join     <manuscript_dir>   # rebuild outline.json from every section file`
```

add the `join` case to `RunOutline`'s switch:

```go
	case "join":
		return cmdOutlineJoin(manuscriptDir, stdout)
```

and append:

```go
func OutlinePath(manuscriptDir string) string {
	return filepath.Join(OutlineDir(manuscriptDir), "outline.json")
}

var ErrNoOutlineSections = errors.New("no outline sections recorded")

// ComputeOutlineJoin rebuilds the whole outline from every current
// per-section file, not only the ones re-recorded this run -- same
// contract as canon reconcile. Deterministic: no model call, no prose
// read. That is what makes the render below free enough to run after
// every chapter.
func ComputeOutlineJoin(manuscriptDir string) (map[string]interface{}, error) {
	recorded, err := loadOutlineSections(manuscriptDir)
	if err != nil {
		return nil, err
	}
	if len(recorded) == 0 {
		return nil, ErrNoOutlineSections
	}

	stems := make([]string, 0, len(recorded))
	for stem := range recorded {
		stems = append(stems, stem)
	}
	// Sorted by stem, which is manuscript order -- never map iteration
	// order, which would make the join non-deterministic and every
	// version archive spuriously different from the last.
	sort.Strings(stems)

	sections := make([]interface{}, 0, len(stems))
	sceneCount := 0
	for _, stem := range stems {
		section := recorded[stem]
		if scenes, ok := section["scenes"].([]interface{}); ok {
			sceneCount += len(scenes)
		}
		sections = append(sections, section)
	}

	joined := map[string]interface{}{
		"sections":    sections,
		"scene_count": sceneCount,
	}

	// published_through travels with the joined document so the renderer
	// (and anything reading outline.json later) doesn't need state too.
	if state, err := schemas.LoadState(manuscriptDir); err == nil && state != nil && state.PublishedThrough != "" {
		joined["published_through"] = state.PublishedThrough
	}

	return joined, nil
}

func cmdOutlineJoin(manuscriptDir string, stdout io.Writer) int {
	joined, err := ComputeOutlineJoin(manuscriptDir)
	if err != nil {
		if err == ErrNoOutlineSections {
			fmt.Fprintf(stdout, "No outline sections in %s. Run `redliner outline stale` and record them first.\n", OutlineSectionsDir(manuscriptDir))
			return 1
		}
		fmt.Fprintf(stdout, "Error reading outline sections: %v\n", err)
		return 1
	}

	if err := os.MkdirAll(OutlineDir(manuscriptDir), 0o755); err != nil {
		fmt.Fprintf(stdout, "Error creating %s: %v\n", OutlineDir(manuscriptDir), err)
		return 1
	}
	raw, err := json.MarshalIndent(joined, "", "  ")
	if err != nil {
		fmt.Fprintf(stdout, "Error encoding outline: %v\n", err)
		return 1
	}
	if err := os.WriteFile(OutlinePath(manuscriptDir), append(raw, '\n'), 0o644); err != nil {
		fmt.Fprintf(stdout, "Error writing %s: %v\n", OutlinePath(manuscriptDir), err)
		return 1
	}

	fmt.Fprintf(stdout, "Joined %d section(s), %v scene(s) → %s\n",
		len(joined["sections"].([]interface{})), joined["scene_count"], OutlinePath(manuscriptDir))
	return 0
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd go && go test ./internal/cli/ -run Outline -v`
Expected: PASS, all ten tests.

- [ ] **Step 5: Run the whole suite and commit**

Run: `cd go && go test ./...`
Expected: all `ok`.

```bash
git add go/internal/cli/outline.go go/internal/cli/outline_test.go
git commit -m "Add redliner outline join"
```

---

### Task 7: `redliner outline render`

**Files:**
- Modify: `go/internal/cli/outline.go`
- Test: `go/internal/cli/outline_test.go`

**Interfaces:**
- Consumes: `ComputeOutlineJoin`, `OutlinePath` from Task 6; `Domain.OutlineRowFields()`/`OutlineSectionFields()` from Task 1.
- Produces:
  - `func RenderedOutlinePath(manuscriptDir string) string` → `<manuscriptDir>/Outline.md`
  - `func RenderOutline(joined map[string]interface{}, rowFields, sectionFields []string) string`

`Outline.md` goes in the manuscript directory, not under `.redliner/`. `schemas.SectionFiles` globs `section_*` + `.txt`/`.md`, so it is never mistaken for manuscript text — the same reason the editorial letters live there.

Target output, for a serial with `published_through: "section_01"`:

```markdown
# Outline

7 scenes across 3 sections.

## section_01

Leaves open: Whether the guard reports her.

1. **Mira** — "The gate was already open when she"
   - Goal: Get inside the compound before the shift change.
   - Conflict: The guard rotation ran early; she has no cover story.
   - Outcome: She gets in, but is seen — the guard now knows her face.

---

*Everything above this line is published. Those scenes can't be moved or cut.*

---

## section_02
...
```

- [ ] **Step 1: Write the failing test**

Append to `go/internal/cli/outline_test.go`:

```go
func TestRenderOutline_ScenesAndFields(t *testing.T) {
	joined := map[string]interface{}{
		"scene_count": 1,
		"sections": []interface{}{
			map[string]interface{}{
				"section": "section_01",
				"scenes": []interface{}{
					map[string]interface{}{
						"order": float64(1), "pov": "Mira", "anchor": "The gate was already open",
						"goal": "Get inside.", "conflict": "The rotation ran early.", "outcome": "She is seen.",
					},
				},
			},
		},
	}
	got := RenderOutline(joined, []string{"goal", "conflict", "outcome"}, nil)

	for _, want := range []string{
		"# Outline", "## section_01", "Mira", "The gate was already open",
		"Goal: Get inside.", "Conflict: The rotation ran early.", "Outcome: She is seen.",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rendered outline missing %q:\n%s", want, got)
		}
	}
}

func TestRenderOutline_SectionFieldOnlyWhenConfigured(t *testing.T) {
	joined := map[string]interface{}{
		"scene_count": 0,
		"sections": []interface{}{
			map[string]interface{}{
				"section": "section_01", "leaves_open": "Whether the guard reports her.",
				"scenes": []interface{}{},
			},
		},
	}
	withField := RenderOutline(joined, []string{"goal"}, []string{"leaves_open"})
	if !strings.Contains(withField, "Whether the guard reports her.") {
		t.Errorf("configured section field not rendered:\n%s", withField)
	}
	withoutField := RenderOutline(joined, []string{"goal"}, nil)
	if strings.Contains(withoutField, "Whether the guard reports her.") {
		t.Errorf("section field rendered for a domain that configures none:\n%s", withoutField)
	}
}

func TestRenderOutline_PublishedBoundary(t *testing.T) {
	joined := map[string]interface{}{
		"scene_count":       0,
		"published_through": "section_01",
		"sections": []interface{}{
			map[string]interface{}{"section": "section_01", "scenes": []interface{}{}},
			map[string]interface{}{"section": "section_02", "scenes": []interface{}{}},
		},
	}
	got := RenderOutline(joined, []string{"goal"}, nil)

	if !strings.Contains(got, "published") {
		t.Fatalf("no published boundary rendered:\n%s", got)
	}
	boundary := strings.Index(got, "Everything above this line is published")
	first := strings.Index(got, "## section_01")
	second := strings.Index(got, "## section_02")
	if boundary < first || boundary > second {
		t.Errorf("boundary at %d is not between section_01 (%d) and section_02 (%d):\n%s", boundary, first, second, got)
	}
}

func TestRenderOutline_NoBoundaryWhenNothingPublished(t *testing.T) {
	joined := map[string]interface{}{
		"scene_count": 0,
		"sections":    []interface{}{map[string]interface{}{"section": "section_01", "scenes": []interface{}{}}},
	}
	if got := RenderOutline(joined, []string{"goal"}, nil); strings.Contains(got, "published") {
		t.Errorf("published boundary drawn when nothing is published:\n%s", got)
	}
}

func TestRunOutlineRender_WritesToManuscriptDirNotDotRedliner(t *testing.T) {
	dir := newOutlineFixture(t, "section_01")
	stale, _ := ComputeOutlineStale(dir)
	writeOutlineSectionWithScenes(t, dir, "section_01", stale.CurrentHashes["section_01"],
		[]map[string]interface{}{scene(1, "enter")})

	var buf bytes.Buffer
	if code := RunOutline([]string{"render", dir}, &buf, &buf); code != 0 {
		t.Fatalf("exit %d: %s", code, buf.String())
	}

	// The visible-file rule: an author who cannot find the deliverable
	// got nothing for the run.
	if _, err := os.Stat(filepath.Join(dir, "Outline.md")); err != nil {
		t.Fatalf("Outline.md not in the manuscript directory: %v", err)
	}
	if _, err := os.Stat(filepath.Join(OutlineDir(dir), "Outline.md")); err == nil {
		t.Error("Outline.md written under .redliner/ -- human-readable output belongs beside the sections")
	}
	if !strings.Contains(buf.String(), filepath.Join(dir, "Outline.md")) {
		t.Errorf("render did not print the path the author needs: %s", buf.String())
	}
}

func TestRunOutlineRender_OutlineMdIsNotMistakenForASection(t *testing.T) {
	dir := newOutlineFixture(t, "section_01")
	stale, _ := ComputeOutlineStale(dir)
	writeOutlineSectionWithScenes(t, dir, "section_01", stale.CurrentHashes["section_01"],
		[]map[string]interface{}{scene(1, "enter")})
	var buf bytes.Buffer
	RunOutline([]string{"render", dir}, &buf, &buf)

	sections, err := schemas.SectionFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(sections) != 1 {
		t.Errorf("section discovery found %v -- Outline.md must not be picked up as manuscript text", sections)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd go && go test ./internal/cli/ -run TestRenderOutline -v`
Expected: FAIL to compile — `undefined: RenderOutline`.

- [ ] **Step 3: Implement the renderer**

In `go/internal/cli/outline.go`, extend the usage constant with:

```
  redliner outline render   <manuscript_dir>   # write the author-readable Outline.md
```

add the case:

```go
	case "render":
		return cmdOutlineRender(manuscriptDir, stdout)
```

and append:

```go
// RenderedOutlinePath is deliberately in the manuscript directory, not
// under .redliner/. Same rule the editorial letters follow: hidden
// storage for machine state, visible files for anything a human reads.
// `.redliner` is a dotfile directory Finder hides by default, and an
// author who cannot find the outline got nothing for the run.
//
// Safe to sit beside the chapters: schemas.SectionFiles globs
// `section_*` with a .txt/.md extension, so nothing named this way is
// discovered as manuscript text.
func RenderedOutlinePath(manuscriptDir string) string {
	return filepath.Join(manuscriptDir, "Outline.md")
}

// titleCaseField turns a config field name into a display label
// ("leaves_open" -> "Leaves open"). Deliberately minimal: the field
// names are authored in domain.json by whoever designs the domain, so
// they are already readable words.
func titleCaseField(name string) string {
	words := strings.Split(name, "_")
	if len(words) == 0 || words[0] == "" {
		return name
	}
	words[0] = strings.ToUpper(words[0][:1]) + words[0][1:]
	return strings.Join(words, " ")
}

// RenderOutline builds the author-facing Markdown. Pure: takes the
// joined document and the domain's field lists, returns a string. No
// model call and no file I/O -- keeping this deterministic is what makes
// the per-run cost proportional to what the author changed rather than
// fixed, which is the whole argument for re-running after every chapter.
func RenderOutline(joined map[string]interface{}, rowFields, sectionFields []string) string {
	var b strings.Builder
	b.WriteString("# Outline\n\n")

	sections, _ := joined["sections"].([]interface{})
	fmt.Fprintf(&b, "%v scene(s) across %d section(s).\n", joined["scene_count"], len(sections))

	publishedThrough, _ := joined["published_through"].(string)

	for _, sectionRaw := range sections {
		section, ok := sectionRaw.(map[string]interface{})
		if !ok {
			continue
		}
		stem, _ := section["section"].(string)
		fmt.Fprintf(&b, "\n## %s\n", stem)

		for _, field := range sectionFields {
			if value, ok := section[field].(string); ok && value != "" {
				fmt.Fprintf(&b, "\n%s: %s\n", titleCaseField(field), value)
			}
		}

		scenes, _ := section["scenes"].([]interface{})
		if len(scenes) == 0 {
			b.WriteString("\n*No scenes recorded.*\n")
		}
		for _, sceneRaw := range scenes {
			scene, ok := sceneRaw.(map[string]interface{})
			if !ok {
				continue
			}
			order, _ := scene["order"].(float64)
			pov, _ := scene["pov"].(string)
			anchor, _ := scene["anchor"].(string)
			fmt.Fprintf(&b, "\n%d. **%s** — \"%s\"\n", int(order), pov, anchor)
			for _, field := range rowFields {
				value, _ := scene[field].(string)
				fmt.Fprintf(&b, "   - %s: %s\n", titleCaseField(field), value)
			}
		}

		// The line goes *after* the last published section. A scene above
		// it cannot be moved or cut at all, which is the one fact this
		// whole view exists to serve.
		if publishedThrough != "" && stem == publishedThrough {
			b.WriteString("\n---\n\n")
			b.WriteString("*Everything above this line is published. Those scenes can't be moved or cut.*\n\n")
			b.WriteString("---\n")
		}
	}

	return b.String()
}

func cmdOutlineRender(manuscriptDir string, stdout io.Writer) int {
	joined, err := ComputeOutlineJoin(manuscriptDir)
	if err != nil {
		if err == ErrNoOutlineSections {
			fmt.Fprintf(stdout, "No outline sections in %s. Run `redliner outline stale` and record them first.\n", OutlineSectionsDir(manuscriptDir))
			return 1
		}
		fmt.Fprintf(stdout, "Error reading outline sections: %v\n", err)
		return 1
	}

	rowFields, sectionFields, err := outlineFieldsFor(manuscriptDir)
	if err != nil {
		fmt.Fprintf(stdout, "Domain config error: %v\n", err)
		return 1
	}

	path := RenderedOutlinePath(manuscriptDir)
	if err := os.WriteFile(path, []byte(RenderOutline(joined, rowFields, sectionFields)), 0o644); err != nil {
		fmt.Fprintf(stdout, "Error writing %s: %v\n", path, err)
		return 1
	}
	// Print the absolute path: telling an author only that "the outline is
	// written" is how a run ends with them unable to find its one
	// deliverable.
	abs, absErr := filepath.Abs(path)
	if absErr != nil {
		abs = path
	}
	fmt.Fprintf(stdout, "Wrote %s\n", abs)
	return 0
}

// outlineFieldsFor resolves the manuscript's domain and returns its
// configured outline fields. A domain with no outline block yields empty
// lists rather than an error -- the caller has already decided the
// layer applies.
func outlineFieldsFor(manuscriptDir string) ([]string, []string, error) {
	domainsDir, err := schemas.FindDomainsDir()
	if err != nil {
		return nil, nil, err
	}
	state, _ := schemas.LoadState(manuscriptDir)
	name := schemas.DefaultDomain
	if state != nil {
		name = state.DomainName()
	}
	domain, err := schemas.LoadDomain(domainsDir, name)
	if err != nil {
		return nil, nil, err
	}
	return domain.OutlineRowFields(), domain.OutlineSectionFields(), nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd go && go test ./internal/cli/ -run Outline -v`

Note: the two `RunOutline([]string{"render", ...})` tests call `schemas.FindDomainsDir()`, which under `go test` searches from a temp binary. Set the documented escape hatch when running them:

```bash
cd go && REDLINER_DOMAINS_DIR="$(git rev-parse --show-toplevel)/domains" go test ./internal/cli/ -run Outline -v
```

Expected: PASS, all sixteen tests.

- [ ] **Step 5: Make the env var unnecessary for the suite**

The whole-suite run must not depend on the caller exporting a variable. Add to `go/internal/cli/outline_test.go`:

```go
// TestMain points FindDomainsDir at the repo's real domains/ for this
// package's tests. Under `go test` os.Executable() is a throwaway temp
// binary with no domains/ nearby -- REDLINER_DOMAINS_DIR is the escape
// hatch designed for exactly this, and golden_test.go already relies on
// it for the same reason.
func TestMain(m *testing.M) {
	if os.Getenv("REDLINER_DOMAINS_DIR") == "" {
		os.Setenv("REDLINER_DOMAINS_DIR", filepath.Join(repoRoot2(), "domains"))
	}
	os.Exit(m.Run())
}

func repoRoot2() string {
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
}
```

Add `"runtime"` to the test imports.

**If `golden_test.go` (or any other file in package `cli`) already defines `TestMain`**, do not add a second one — Go allows only one per package. In that case, add the two lines that set the variable into the existing `TestMain` instead, and skip `repoRoot2` in favor of the existing `repoRoot` helper.

- [ ] **Step 6: Run the whole suite and commit**

Run: `cd go && go test ./...`
Expected: all `ok`, with no environment variable set by the caller.

```bash
git add go/internal/cli/outline.go go/internal/cli/outline_test.go
git commit -m "Add redliner outline render"
```

---

### Task 8: Per-run version archive and `redliner outline versions`

**Files:**
- Modify: `go/internal/cli/outline.go`
- Test: `go/internal/cli/outline_test.go`

**Interfaces:**
- Consumes: `ComputeOutlineJoin`, `OutlinePath`, `RenderOutline`, `RenderedOutlinePath` from Tasks 6–7; `State.OutlineVersion` from Task 3.
- Produces:
  - `func OutlineVersionsDir(manuscriptDir string) string` → `<dir>/.redliner/outline/versions`
  - `func ArchiveOutlineVersion(manuscriptDir string, changedSections []string) (string, bool, error)` — returns the version directory, whether anything was archived, and any error. Archives nothing (and returns `false`) when the joined content matches the newest existing version.
  - `type OutlineVersionMeta struct { Version int; ChangedSections []string }`

Version directory contents: `outline.json`, `Outline.md`, `meta.json`.

- [ ] **Step 1: Write the failing test**

Append to `go/internal/cli/outline_test.go`:

```go
func TestArchiveOutlineVersion_FirstRunCreatesV1(t *testing.T) {
	dir := newOutlineFixture(t, "section_01")
	stale, _ := ComputeOutlineStale(dir)
	writeOutlineSectionWithScenes(t, dir, "section_01", stale.CurrentHashes["section_01"],
		[]map[string]interface{}{scene(1, "enter")})

	path, archived, err := ArchiveOutlineVersion(dir, []string{"section_01"})
	if err != nil {
		t.Fatal(err)
	}
	if !archived {
		t.Fatal("first run archived nothing")
	}
	if filepath.Base(path) != "v1" {
		t.Errorf("first version directory = %s, want v1", filepath.Base(path))
	}
	for _, name := range []string{"outline.json", "Outline.md", "meta.json"} {
		if _, err := os.Stat(filepath.Join(path, name)); err != nil {
			t.Errorf("version is missing %s: %v", name, err)
		}
	}

	// The rendered Markdown is what makes a version readable by a person
	// at all -- JSON alone would mean hand-reading a hidden directory.
	raw, err := os.ReadFile(filepath.Join(path, "Outline.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "# Outline") {
		t.Errorf("archived Outline.md is not the rendered view:\n%s", raw)
	}

	state, err := schemas.LoadState(dir)
	if err != nil {
		t.Fatal(err)
	}
	if state.OutlineVersion != 1 {
		t.Errorf("state.OutlineVersion = %d, want 1", state.OutlineVersion)
	}
}

func TestArchiveOutlineVersion_NoOpRunArchivesNothing(t *testing.T) {
	dir := newOutlineFixture(t, "section_01")
	stale, _ := ComputeOutlineStale(dir)
	writeOutlineSectionWithScenes(t, dir, "section_01", stale.CurrentHashes["section_01"],
		[]map[string]interface{}{scene(1, "enter")})

	if _, archived, err := ArchiveOutlineVersion(dir, []string{"section_01"}); err != nil || !archived {
		t.Fatalf("first archive: archived=%v err=%v", archived, err)
	}
	_, archived, err := ArchiveOutlineVersion(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if archived {
		t.Error("unchanged content archived a second version -- a no-op run must archive nothing")
	}

	entries, err := os.ReadDir(OutlineVersionsDir(dir))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("versions dir holds %d entries, want 1", len(entries))
	}
}

func TestArchiveOutlineVersion_ChangedContentCreatesV2(t *testing.T) {
	dir := newOutlineFixture(t, "section_01")
	stale, _ := ComputeOutlineStale(dir)
	writeOutlineSectionWithScenes(t, dir, "section_01", stale.CurrentHashes["section_01"],
		[]map[string]interface{}{scene(1, "enter")})
	if _, _, err := ArchiveOutlineVersion(dir, []string{"section_01"}); err != nil {
		t.Fatal(err)
	}

	writeOutlineSectionWithScenes(t, dir, "section_01", stale.CurrentHashes["section_01"],
		[]map[string]interface{}{scene(1, "enter"), scene(2, "flee")})
	path, archived, err := ArchiveOutlineVersion(dir, []string{"section_01"})
	if err != nil {
		t.Fatal(err)
	}
	if !archived || filepath.Base(path) != "v2" {
		t.Fatalf("second archive: archived=%v path=%s, want v2", archived, path)
	}

	// v1 must still hold the old content -- that is the entire point.
	oldRaw, err := os.ReadFile(filepath.Join(OutlineVersionsDir(dir), "v1", "outline.json"))
	if err != nil {
		t.Fatal(err)
	}
	var old map[string]interface{}
	if err := json.Unmarshal(oldRaw, &old); err != nil {
		t.Fatal(err)
	}
	if old["scene_count"] != float64(1) {
		t.Errorf("v1 scene_count = %v, want 1 -- the earlier version was overwritten", old["scene_count"])
	}
}

func TestArchiveOutlineVersion_MetaRecordsChangedSections(t *testing.T) {
	dir := newOutlineFixture(t, "section_01")
	stale, _ := ComputeOutlineStale(dir)
	writeOutlineSectionWithScenes(t, dir, "section_01", stale.CurrentHashes["section_01"],
		[]map[string]interface{}{scene(1, "enter")})

	path, _, err := ArchiveOutlineVersion(dir, []string{"section_01"})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(path, "meta.json"))
	if err != nil {
		t.Fatal(err)
	}
	var meta OutlineVersionMeta
	if err := json.Unmarshal(raw, &meta); err != nil {
		t.Fatal(err)
	}
	if meta.Version != 1 {
		t.Errorf("meta.Version = %d, want 1", meta.Version)
	}
	if len(meta.ChangedSections) != 1 || meta.ChangedSections[0] != "section_01" {
		t.Errorf("meta.ChangedSections = %v, want [section_01]", meta.ChangedSections)
	}
}

func TestRunOutlineVersions_ListsWhatIsKept(t *testing.T) {
	dir := newOutlineFixture(t, "section_01")
	stale, _ := ComputeOutlineStale(dir)
	writeOutlineSectionWithScenes(t, dir, "section_01", stale.CurrentHashes["section_01"],
		[]map[string]interface{}{scene(1, "enter")})
	if _, _, err := ArchiveOutlineVersion(dir, []string{"section_01"}); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if code := RunOutline([]string{"versions", dir}, &buf, &buf); code != 0 {
		t.Fatalf("exit %d: %s", code, buf.String())
	}
	out := buf.String()
	if !strings.Contains(out, "v1") {
		t.Errorf("listing does not name the version: %s", out)
	}
	if !strings.Contains(out, "section_01") {
		t.Errorf("listing does not say what changed: %s", out)
	}
	// Reading a version means opening its archived Markdown, so the path
	// has to be printed.
	if !strings.Contains(out, filepath.Join("v1", "Outline.md")) {
		t.Errorf("listing does not print the readable path: %s", out)
	}
}

func TestRunOutlineVersions_EmptyIsNotAnError(t *testing.T) {
	dir := newOutlineFixture(t, "section_01")
	var buf bytes.Buffer
	if code := RunOutline([]string{"versions", dir}, &buf, &buf); code != 0 {
		t.Errorf("exit %d for a manuscript with no versions yet: %s", code, buf.String())
	}
	if !strings.Contains(buf.String(), "No outline versions") {
		t.Errorf("unhelpful empty listing: %s", buf.String())
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd go && go test ./internal/cli/ -run TestArchiveOutlineVersion -v`
Expected: FAIL to compile — `undefined: ArchiveOutlineVersion`.

- [ ] **Step 3: Implement version archiving**

In `go/internal/cli/outline.go`, extend the usage constant with:

```
  redliner outline versions <manuscript_dir>   # list archived outline versions
```

add the case:

```go
	case "versions":
		return cmdOutlineVersions(manuscriptDir, stdout)
```

and append:

```go
func OutlineVersionsDir(manuscriptDir string) string {
	return filepath.Join(OutlineDir(manuscriptDir), "versions")
}

// OutlineVersionMeta is one archived version's small sidecar. Timestamps
// are RFC3339, same as state's, and are informational only -- version
// ordering comes from the counter, never from mtime.
type OutlineVersionMeta struct {
	Version         int      `json:"version"`
	ArchivedAt      string   `json:"archived_at"`
	ChangedSections []string `json:"changed_sections"`
	SceneCount      int      `json:"scene_count"`
}

// ArchiveOutlineVersion writes a new version when the joined outline
// differs from the newest archived one, and does nothing otherwise.
//
// This cadence is the point. Keyed to the developmental round the way
// continuity is, the layer would produce no history at all for its
// primary workflow -- the author's loop is write a chapter, outline,
// write the next, outline, a loop that need never run `assess`. Every
// one of those runs would overwrite outline.json with nothing kept.
//
// Both the JSON and the rendered Markdown are archived. The Markdown is
// what makes a version readable by a person; without it, "see version 4"
// means hand-reading JSON inside a hidden directory. It costs a file
// copy because the render is deterministic.
func ArchiveOutlineVersion(manuscriptDir string, changedSections []string) (string, bool, error) {
	joined, err := ComputeOutlineJoin(manuscriptDir)
	if err != nil {
		return "", false, err
	}
	raw, err := json.MarshalIndent(joined, "", "  ")
	if err != nil {
		return "", false, err
	}
	raw = append(raw, '\n')

	state, err := schemas.LoadState(manuscriptDir)
	if err != nil {
		return "", false, err
	}
	if state == nil {
		return "", false, fmt.Errorf("no state in %s", manuscriptDir)
	}

	// Compare against the newest archived version rather than against
	// outline.json, which the caller may already have rewritten this run.
	if state.OutlineVersion > 0 {
		previous := filepath.Join(OutlineVersionsDir(manuscriptDir), fmt.Sprintf("v%d", state.OutlineVersion), "outline.json")
		if existing, err := os.ReadFile(previous); err == nil && string(existing) == string(raw) {
			return "", false, nil
		}
	}

	next := state.OutlineVersion + 1
	dest := filepath.Join(OutlineVersionsDir(manuscriptDir), fmt.Sprintf("v%d", next))
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return "", false, err
	}
	if err := os.WriteFile(filepath.Join(dest, "outline.json"), raw, 0o644); err != nil {
		return "", false, err
	}

	rowFields, sectionFields, err := outlineFieldsFor(manuscriptDir)
	if err != nil {
		return "", false, err
	}
	rendered := RenderOutline(joined, rowFields, sectionFields)
	if err := os.WriteFile(filepath.Join(dest, "Outline.md"), []byte(rendered), 0o644); err != nil {
		return "", false, err
	}

	sceneCount := 0
	if n, ok := joined["scene_count"].(int); ok {
		sceneCount = n
	}
	meta := OutlineVersionMeta{
		Version:         next,
		ArchivedAt:      schemas.NowISO(),
		ChangedSections: orEmptyStrings(changedSections),
		SceneCount:      sceneCount,
	}
	metaRaw, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return "", false, err
	}
	if err := os.WriteFile(filepath.Join(dest, "meta.json"), append(metaRaw, '\n'), 0o644); err != nil {
		return "", false, err
	}

	state.OutlineVersion = next
	if _, err := schemas.SaveState(manuscriptDir, state); err != nil {
		return "", false, err
	}
	return dest, true, nil
}

func cmdOutlineVersions(manuscriptDir string, stdout io.Writer) int {
	entries, err := os.ReadDir(OutlineVersionsDir(manuscriptDir))
	if err != nil || len(entries) == 0 {
		fmt.Fprintln(stdout, "No outline versions archived yet.")
		return 0
	}

	type row struct {
		meta OutlineVersionMeta
		path string
	}
	var rows []row
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(OutlineVersionsDir(manuscriptDir), e.Name())
		metaRaw, err := os.ReadFile(filepath.Join(dir, "meta.json"))
		if err != nil {
			continue
		}
		var meta OutlineVersionMeta
		if err := json.Unmarshal(metaRaw, &meta); err != nil {
			continue
		}
		rows = append(rows, row{meta: meta, path: dir})
	}
	if len(rows) == 0 {
		fmt.Fprintln(stdout, "No outline versions archived yet.")
		return 0
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].meta.Version < rows[j].meta.Version })

	fmt.Fprintf(stdout, "Archived outline versions (%d):\n", len(rows))
	for _, r := range rows {
		changed := "no sections re-recorded"
		if len(r.meta.ChangedSections) > 0 {
			changed = "changed: " + strings.Join(r.meta.ChangedSections, ", ")
		}
		fmt.Fprintf(stdout, "  v%-4d %s  %d scene(s), %s\n", r.meta.Version, r.meta.ArchivedAt, r.meta.SceneCount, changed)
		// Print the readable path, not just the version: reading a version
		// means opening this file.
		fmt.Fprintf(stdout, "         %s\n", filepath.Join(r.path, "Outline.md"))
	}
	return 0
}
```

- [ ] **Step 4: Export `NowISO` from schemas**

`ArchiveOutlineVersion` needs the same timestamp format state uses. In `go/internal/schemas/project_state.go`, add beside the existing unexported `nowISO`:

```go
// NowISO is nowISO exported for callers outside this package that write
// their own timestamped sidecars (the outline layer's version meta).
// Same format as state's, so anything reading both sees one convention.
func NowISO() string { return nowISO() }
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `cd go && go test ./internal/cli/ -run Outline -v`
Expected: PASS, all twenty-two tests.

- [ ] **Step 6: Run the whole suite and commit**

Run: `cd go && go test ./...`
Expected: all `ok`.

```bash
git add go/internal/cli/outline.go go/internal/cli/outline_test.go go/internal/schemas/project_state.go
git commit -m "Archive an outline version per changed run, and list them"
```

---

### Task 9: Round archiving — split archive kinds from pass kinds

**Files:**
- Modify: `go/internal/cli/rounds.go:88-146` (`cmdRoundsArchive`)
- Modify: `go/internal/cli/state.go:210` (`passKinds`)
- Test: `go/internal/cli/rounds_test.go`

**Interfaces:**
- Consumes: `OutlinePath` from Task 6.
- Produces: `var archiveKinds = []string{"developmental", "line", "continuity", "outline"}` in `rounds.go`; `passKinds` in `state.go` stays `{"developmental", "line", "continuity"}`.

`state pass outline` must remain unavailable. The outline refreshes automatically inside every `assess`, so recording it as a completed pass would make `status` report "outline: run" permanently — a constant, and therefore no signal. The informative report is per-section staleness, which `status` already does for continuity.

- [ ] **Step 1: Write the failing test**

Append to `go/internal/cli/rounds_test.go`:

```go
func TestRoundsArchive_OutlineIsAnArchiveKind(t *testing.T) {
	dir := t.TempDir()
	stateDir := filepath.Join(dir, ".redliner")
	if err := os.MkdirAll(filepath.Join(stateDir, "outline"), 0o755); err != nil {
		t.Fatal(err)
	}
	state := `{"manuscript_dir":"` + dir + `","domain":"fiction","phase":"developmental",` +
		`"developmental_round":3,"section_fingerprints":{},"created_at":"x"}`
	if err := os.WriteFile(filepath.Join(stateDir, "state.json"), []byte(state), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, "outline", "outline.json"), []byte(`{"sections":[],"scene_count":0}`), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if code := RunRounds([]string{"archive", dir, "outline"}, &buf); code != 0 {
		t.Fatalf("archive outline: %s", buf.String())
	}
	got, err := os.ReadDir(filepath.Join(stateDir, "rounds", "outline-round3"))
	if err != nil {
		t.Fatalf("expected outline-round3 archive: %v", err)
	}
	if len(got) != 1 || got[0].Name() != "outline.json" {
		t.Errorf("outline archive holds %v, want [outline.json]", got)
	}
}

func TestStatePass_OutlineIsDeliberatelyUnavailable(t *testing.T) {
	// The outline refreshes automatically inside every assess, so
	// recording it as a completed pass would make status report
	// "outline: run" permanently -- a constant, not a signal. Per-section
	// staleness is the informative report.
	dir := t.TempDir()
	stateDir := filepath.Join(dir, ".redliner")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	state := `{"manuscript_dir":"` + dir + `","domain":"fiction","phase":"developmental",` +
		`"developmental_round":1,"section_fingerprints":{},"created_at":"x"}`
	if err := os.WriteFile(filepath.Join(stateDir, "state.json"), []byte(state), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if code := RunState([]string{"pass", dir, "outline"}, &buf); code == 0 {
		t.Error("`state pass outline` succeeded -- it must stay unavailable")
	}
	if !strings.Contains(buf.String(), "continuity") {
		t.Errorf("rejection should name the kinds that ARE valid: %s", buf.String())
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd go && go test ./internal/cli/ -run "TestRoundsArchive_Outline|TestStatePass_Outline" -v`
Expected: `TestRoundsArchive_OutlineIsAnArchiveKind` FAILs with `Unknown pass 'outline'`. `TestStatePass_OutlineIsDeliberatelyUnavailable` may already pass — that is fine, it is a regression guard for the split you are about to make.

- [ ] **Step 3: Split the two lists**

In `go/internal/cli/rounds.go`, add above `cmdRoundsArchive`:

```go
// archiveKinds are the artifact sets `rounds archive` knows how to keep.
// Deliberately a different list from state.go's passKinds, which governs
// `state pass`: "outline" is an archive kind but not a pass kind. The
// outline refreshes automatically inside every assess, so recording it
// as a completed pass would make status report "outline: run"
// permanently -- a constant, and therefore no signal at all. What status
// should report for this layer is per-section staleness, the way it
// already does for continuity.
var archiveKinds = []string{"developmental", "line", "continuity", "outline"}
```

In `cmdRoundsArchive`, change the validation loop and its error message from `passKinds` to `archiveKinds`:

```go
	valid := false
	for _, k := range archiveKinds {
		if k == pass {
			valid = true
		}
	}
	if !valid {
		fmt.Fprintf(stdout, "Unknown pass %s. Must be one of: %s\n", pyReprStr(pass), strings.Join(archiveKinds, ", "))
		return 1
	}
```

and add the `outline` case to the `switch pass` source-selection block:

```go
	case "outline":
		sources, _ = filepath.Glob(filepath.Join(stateDir, "outline", "outline.json"))
```

Leave `passKinds` in `state.go` exactly as it is.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd go && go test ./internal/cli/ -run "TestRoundsArchive|TestStatePass" -v`
Expected: PASS, including the existing `TestRoundsArchive_KeepsEachPassSeparately`.

- [ ] **Step 5: Run the whole suite and commit**

Run: `cd go && go test ./...`
Expected: all `ok`.

```bash
git add go/internal/cli/rounds.go go/internal/cli/rounds_test.go
git commit -m "Make outline an archive kind without making it a pass kind"
```

---

### Task 10: Validate outline files in `redliner validate`

**Files:**
- Modify: `go/internal/cli/validate.go` (`ValidateManuscript`, ~line 168; add a `validateOutline` helper beside `validateCanon`)
- Test: `go/internal/cli/validate_outline_test.go`

**Interfaces:**
- Consumes: `schemas.ValidateOutlineSection` (Task 4), `Domain.OutlineRowFields()`/`OutlineSectionFields()` (Task 1), `OutlineSectionsDir` (Task 5).
- Produces: outline files included in the `OK`/`FAIL` walk, so a bad recording stops a pass instead of feeding the developmental editor.

`ValidateManuscript` walks everything under `.redliner/`. A new file type there needs a schema or it is silently ignored — which for this layer would mean an invalid outline reaching the developmental editor unnoticed.

- [ ] **Step 1: Write the failing test**

Create `go/internal/cli/validate_outline_test.go`:

```go
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
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd go && go test ./internal/cli/ -run TestValidate_ -v`
Expected: `TestValidate_AcceptsWellFormedOutline` FAILs — the outline file is not mentioned in the output at all. `TestValidate_RejectsJudgmentInOutline` FAILs — exit code 0.

- [ ] **Step 3: Wire the outline into validation**

In `go/internal/cli/validate.go`, in `ValidateManuscript`, change the line that calls `validateCanon` to also run the outline check:

```go
	ok := validateCanon(stdout, manuscriptDir, redlinerPath, domain)
	ok = validateOutline(stdout, redlinerPath, domain) && ok
```

and add beside `validateCanon`:

```go
// validateOutline checks every recorded outline section against the
// domain's configured fields. A domain with no outline block validates
// nothing here -- design-doc opts out of the layer entirely, and a
// manuscript created before this layer existed simply has no directory.
//
// Wired in because ValidateManuscript walks everything under .redliner/:
// a new file type with no schema is not "unchecked", it is silently
// accepted, and an invalid outline reaching the developmental editor as
// its structural spine is exactly the failure that would look like a
// bad pass rather than a bad file.
func validateOutline(stdout io.Writer, redlinerPath string, domain schemas.Domain) bool {
	if !domain.HasOutline() {
		return true
	}
	sectionsPath := filepath.Join(redlinerPath, "outline", "sections")
	if info, err := os.Stat(sectionsPath); err != nil || !info.IsDir() {
		return true
	}

	rowFields := domain.OutlineRowFields()
	sectionFields := domain.OutlineSectionFields()

	files, _ := filepath.Glob(filepath.Join(sectionsPath, "*.json"))
	sort.Strings(files)

	ok := true
	for _, file := range files {
		report := loadJSON(file)
		errs := schemas.ValidateOutlineSection(report, rowFields, sectionFields)
		ok = checkFile(stdout, file, errs) && ok
	}
	return ok
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd go && go test ./internal/cli/ -run TestValidate_ -v`
Expected: PASS, all three tests.

- [ ] **Step 5: Run the whole suite and commit**

Run: `cd go && go test ./...`
Expected: all `ok`, including `TestCLI_MatchesPythonGolden` — the `happy` fixture has no outline directory, so `07_validate_findings`'s output must be unchanged. If it moved, `validateOutline` is printing when it should be returning early.

```bash
git add go/internal/cli/validate.go go/internal/cli/validate_outline_test.go
git commit -m "Validate recorded outline sections"
```

---

### Task 11: MCP tools for the outline commands

**Files:**
- Modify: `go/internal/mcpserver/server.go` (registration ~line 59, handlers at the end)
- Modify: `go/internal/mcpserver/descriptions.go` (Go-only section, ~line 95)
- Modify: `go/internal/mcpserver/frontdoor_parity_test.go` (`commandToTool`, ~line 47)
- Test: `go/internal/mcpserver/server_test.go`

**Interfaces:**
- Consumes: `cli.ComputeOutlineStale`, `cli.ComputeOutlineJoin`, `cli.RunOutline`, `cli.ArchiveOutlineVersion` from Tasks 5–8.
- Produces: MCP tools `outline_stale`, `outline_join`, `outline_render`, `outline_versions`.

Without these, the Cowork variant cannot follow the same skill prose the CLI variant does. That is not hypothetical: five commands once shipped with no tool and Cowork could not complete a pass for two releases.

- [ ] **Step 1: Write the failing test**

Append to `go/internal/mcpserver/server_test.go`:

```go
func TestOutlineToolsAreRegistered(t *testing.T) {
	srv := NewServer(filepath.Join(repoRootForParity(t), "domains"))
	tools := map[string]bool{}
	for _, tl := range listToolsForParity(t, srv) {
		tools[tl.Name] = true
	}
	for _, name := range []string{"outline_stale", "outline_join", "outline_render", "outline_versions"} {
		if !tools[name] {
			t.Errorf("MCP server exposes no %q tool -- the Cowork front door cannot follow the outline skill prose without it", name)
		}
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd go && go test ./internal/mcpserver/ -run TestOutlineToolsAreRegistered -v`
Expected: FAIL — all four tools missing.

- [ ] **Step 3: Add the descriptions**

Append to the Go-only section of `go/internal/mcpserver/descriptions.go`:

```go
const descOutlineStale = `Which sections need (re-)recording for the outline, plus their current SHA-256 hashes and any orphaned outline files whose section no longer exists. Cheap and deterministic — call it before recording, and skip any section it does not list.`

const descOutlineJoin = `Rebuild .redliner/outline/outline.json from every current per-section outline file, in manuscript order. Deterministic; no model call.`

const descOutlineRender = `Write the author-readable Outline.md into the manuscript directory (not .redliner/), rendering every recorded scene and the published boundary. Deterministic; no model call.`

const descOutlineVersions = `List the archived outline versions under .redliner/outline/versions/, newest counter last, with the date, what changed, and the path to each version's readable Outline.md.`
```

- [ ] **Step 4: Register the tools and add the handlers**

In `go/internal/mcpserver/server.go`, add after the `canon_merge` registration:

```go
	mcp.AddTool(srv, &mcp.Tool{Name: "outline_stale", Description: descOutlineStale}, s.outlineStale)
	mcp.AddTool(srv, &mcp.Tool{Name: "outline_join", Description: descOutlineJoin}, s.outlineJoin)
	mcp.AddTool(srv, &mcp.Tool{Name: "outline_render", Description: descOutlineRender}, s.outlineRender)
	mcp.AddTool(srv, &mcp.Tool{Name: "outline_versions", Description: descOutlineVersions}, s.outlineVersions)
```

and append the handlers:

```go
// --- outline_* -- the scene outliner layer. outlineStale returns the
// computed struct directly (like canonStale); the other three run the
// CLI command and return its human-readable stdout, because their whole
// value is a written file plus the path to it. ---

func (s *redlinerServer) outlineStale(_ context.Context, _ *mcp.CallToolRequest, in manuscriptDirInput) (*mcp.CallToolResult, any, error) {
	result, err := cli.ComputeOutlineStale(in.ManuscriptDir)
	if err != nil {
		if ce, ok := err.(*schemas.SectionCollisionError); ok {
			return nil, errorResult("Section file error: %s", ce.Error()), nil
		}
		return nil, nil, err
	}
	return nil, result, nil
}

func (s *redlinerServer) outlineJoin(_ context.Context, _ *mcp.CallToolRequest, in manuscriptDirInput) (*mcp.CallToolResult, any, error) {
	return runOutlineCommand("join", in.ManuscriptDir)
}

func (s *redlinerServer) outlineRender(_ context.Context, _ *mcp.CallToolRequest, in manuscriptDirInput) (*mcp.CallToolResult, any, error) {
	return runOutlineCommand("render", in.ManuscriptDir)
}

func (s *redlinerServer) outlineVersions(_ context.Context, _ *mcp.CallToolRequest, in manuscriptDirInput) (*mcp.CallToolResult, any, error) {
	return runOutlineCommand("versions", in.ManuscriptDir)
}

func runOutlineCommand(command, manuscriptDir string) (*mcp.CallToolResult, any, error) {
	var out bytes.Buffer
	code := cli.RunOutline([]string{command, manuscriptDir}, &out, &out)
	if code != 0 {
		return nil, errorResult("%s", strings.TrimSpace(out.String())), nil
	}
	return nil, map[string]any{"output": strings.TrimSpace(out.String())}, nil
}
```

Add `"bytes"` to `server.go`'s imports if not already present.

- [ ] **Step 5: Map the commands in the parity guard**

In `go/internal/mcpserver/frontdoor_parity_test.go`, add to `commandToTool`:

```go
	"outline stale":    "outline_stale",
	"outline join":     "outline_join",
	"outline render":   "outline_render",
	"outline versions": "outline_versions",
```

This must land before Task 12 writes those command strings into `skills/run/SKILL.md`, or `TestEverySkillCommandHasAnMCPTool` fails on that task instead of this one.

- [ ] **Step 6: Run the tests to verify they pass**

Run: `cd go && go test ./internal/mcpserver/ -v`
Expected: PASS, including `TestEverySkillCommandHasAnMCPTool`.

- [ ] **Step 7: Run the whole suite and commit**

Run: `cd go && go test ./...`
Expected: all `ok`.

```bash
git add go/internal/mcpserver/
git commit -m "Expose the outline commands as MCP tools"
```

---

### Task 12: The outliner agent files

**Files:**
- Create: `skills/new-domain/reference/templates/outliner.md`
- Create: `agents/fiction-outliner.md`
- Create: `agents/serial-fiction-outliner.md`
- Modify: `skills/new-domain/SKILL.md` (Step 6 heading and body, ~line 155; Step 7's verification list, ~line 185)

**Interfaces:**
- Consumes: the `outline` block shape from Task 1; the validated file shape from Task 4.
- Produces: `redliner:<domain>-outliner` subagents, which Task 13's skill prose Tasks by name.

Per TODO.md's B1 decision, domains get concrete generated agent files rather than one generic prompt with runtime-injected vocabulary. The template is the source; the two concrete files are what actually run.

- [ ] **Step 1: Write the fiction outliner**

Create `agents/fiction-outliner.md`:

```markdown
---
name: fiction-outliner
description: Records the scenes in a single section of a manuscript as goal/conflict/outcome rows. Use during outline passes, once per section. Records only — does not judge, rate, or suggest.
tools: Read, Write
model: inherit
---

You record the scenes in one section of a manuscript. You are a
recorder, not an editor.

## Your one job

Write down what each scene in this section is *for*, with enough
specificity that someone deciding whether to cut or move it could decide
from your rows alone.

You will notice things that look weak — a scene where nothing happens, a
goal that repeats last chapter's, dialogue that goes nowhere. **Record
them flatly and move on.** A scene whose outcome is "nothing changes"
is a legitimate recording, and it is the single most useful row you can
write, because it is exactly what the author is looking for. Writing
"nothing changes" is your job. Writing "consider cutting this" is not.

The output schema has no field for an opinion — no `note`, no
`severity`, no `concern`, no `suggestion`. That is deliberate, and the
validator rejects files with extra keys. The developmental pass reads
your rows and does the judging; it has the whole manuscript and the
author's brief, and you have one section.

## Finding scene boundaries

A scene is a continuous unit of action in one place and time with one
driving intention. A new scene starts when the location changes, when
time jumps, or when the POV moves to a different character — most often
marked by a section break or a white-line gap, but not always.

A section may hold one scene or six. Do not force a count.

If two candidate boundaries are equally defensible, pick the one that
produces the more useful row — the split where each half has its own
goal and its own outcome.

## What to record per scene

- **`pov`** — whose viewpoint the scene is in. A name, not a description.
- **`anchor`** — the scene's first few words, copied verbatim from the
  text. This is how a human finds the scene again; it must match the
  prose exactly.
- **`goal`** — what the driving character was trying to achieve. Concrete
  and specific to this scene: "get inside the compound before the shift
  change," not "advance her plan."
- **`conflict`** — what opposed the goal. Another character, a physical
  obstacle, the character's own reluctance, or a fact they did not know.
- **`outcome`** — what changed as a result. State changes plainly. If
  nothing changed, write that: "Nothing changes; she leaves as she
  arrived."

Write each as one sentence. Two if the scene genuinely needs it. This is
a view the author scans, not prose they read.

## What to do

1. Read the section file you are given.
2. Write the outline file to the given output path.

You will also be given the section's SHA-256 hash — copy it into
`section_sha256` exactly. It is how the pipeline knows to skip
re-recording an unchanged section later, which is what makes this layer
cheap enough to re-run after every chapter.

## Output format

Write **only** valid JSON to the given path (no markdown fences, no
commentary in the file):

```json
{
  "section": "section_03",
  "section_sha256": "<the hash you were given>",
  "scenes": [
    {
      "order": 1,
      "pov": "Mira",
      "anchor": "The gate was already open when she",
      "goal": "Get inside the compound before the shift change.",
      "conflict": "The guard rotation ran early; she has no cover story.",
      "outcome": "She gets in, but is seen — the guard now knows her face."
    }
  ]
}
```

`order` starts at 1 and increases by 1, matching the scenes' order in the
section. A section with no scenes yet (a stub file) gets `"scenes": []`
— that is a valid recording, not an error.

## Absolute rule

Read the section. Never modify it, never overwrite a `section_*` file,
and never "fix" anything you find in the prose. Your only write is the
one JSON file at the path you are given.
```

- [ ] **Step 2: Write the serial-fiction outliner**

Create `agents/serial-fiction-outliner.md` as a copy of the file above with these three differences:

Front matter:

```markdown
---
name: serial-fiction-outliner
description: Records the scenes in a single chapter of a serialized work as goal/conflict/outcome rows, plus what the chapter leaves open. Use during outline passes, once per chapter. Records only — does not judge, rate, or suggest.
tools: Read, Write
model: inherit
---
```

Add this section immediately after "What to record per scene":

```markdown
## What to record per chapter

- **`leaves_open`** — what question this chapter ends on. The unresolved
  thread a reader carries into the gap before the next installment.

This is a recording, not a rating. Write "The chapter ends with the guard
having seen her face and no indication whether he reported it." Do not
write "strong hook" or "the ending is weak" — the developmental pass
judges hook strength against the author's stated expectations in the
brief, which you have not read.

It exists because cutting a scene from a serial has a consequence a novel
does not have: it can gut the beat the installment ends on. An author
scanning the outline to decide what to cut needs that visible at a
glance.
```

And extend the output example's top level:

```json
{
  "section": "section_03",
  "section_sha256": "<the hash you were given>",
  "leaves_open": "Whether the guard reports her.",
  "scenes": [ ... ]
}
```

- [ ] **Step 3: Write the template**

Create `skills/new-domain/reference/templates/outliner.md` as the fiction file with the domain-specific parts replaced by the same placeholder convention the other six templates use. Read one of them first — `skills/new-domain/reference/templates/continuity-extractor.md` — and match its placeholder syntax exactly rather than inventing a second convention.

The parts that vary by domain:
- the `name`/`description` front matter,
- the "unit" noun (section / chapter),
- the `row_fields` list under "What to record per scene", generated from the domain's `outline.row_fields` names and prompts,
- the optional "What to record per chapter" section, generated from `outline.section_fields`, and omitted entirely when the domain configures none,
- the output-format example, which must show exactly the configured fields.

- [ ] **Step 4: Update the new-domain skill**

In `skills/new-domain/SKILL.md`:

- Change the Step 6 heading from "Generate the six agent files" to "Generate the agent files".
- In Step 6's body, add the outliner as a conditional seventh: generated **only** when the domain's `domain.json` has an `outline` block, from `reference/templates/outliner.md`, named `agents/<domain>-outliner.md`.
- Add to the Step 3 or Step 5 interview (wherever `domain.json` is designed) a short prompt asking whether this domain wants an outline layer, and if so what its row fields are — with the guidance that the fields must be recordable facts about a unit, never ratings, and that three to four is the working range.
- In Step 7's verification list, add the outliner file to what gets checked when the domain has one.

Then search for any other place that says "six": `grep -rn "six agent" skills/ README.md` and update each hit. `skills/run/SKILL.md`'s "Which subagent to Task" section lists six roles by name and needs `outliner` added — that edit belongs to Task 13, not here.

- [ ] **Step 5: Verify the agent files are well-formed**

There is no compiler for Markdown agent files, so check the two things that actually break at runtime — the front matter parses and the name matches the filename:

```bash
for f in agents/fiction-outliner.md agents/serial-fiction-outliner.md; do
  echo "== $f"
  head -6 "$f"
done
```
Expected: each shows `---`, a `name:` matching its filename stem, a `description:`, `tools: Read, Write`, `model: inherit`, `---`.

Then confirm no agent file claims a tool it should not have:
```bash
grep -l "tools:" agents/*outliner*.md | xargs grep "tools:"
```
Expected: `tools: Read, Write` for both. An outliner with `Edit` could modify the manuscript, which the absolute rule forbids.

- [ ] **Step 6: Run the suite and commit**

Run: `cd go && go test ./...`
Expected: all `ok`. (Nothing in Go changed, but the parity guard reads `skills/`, and Step 4 edited a skill file.)

```bash
git add agents/fiction-outliner.md agents/serial-fiction-outliner.md \
        skills/new-domain/reference/templates/outliner.md skills/new-domain/SKILL.md
git commit -m "Add the outliner agent files and generate them for new domains"
```

---

### Task 13: Wire the outline into the run skill

**Files:**
- Modify: `skills/run/SKILL.md` — the "Which subagent to Task" list (~line 124), the `assess` flow (~line 219), a new `/redliner:run outline` section (before `## /redliner:run continuity`, ~line 540), and `## /redliner:run status` (~line 650)

**Interfaces:**
- Consumes: every command from Tasks 5–9 and the agents from Task 12.
- Produces: the prose an agent actually follows. This is the deliverable the author experiences; everything before it is machinery.

- [ ] **Step 1: Add the outliner to the subagent-name list**

In "Which subagent to Task", change "This holds for all six roles" to "all seven roles" and add `outliner` to the list:

```
`developmental-editor`, `line-editor`, `editorial-aggregator`,
`continuity-extractor`, `continuity-adjudicator`, `continuity-joiner`,
`outliner`.
```

Add one sentence after the list:

> The `outliner` role exists only for domains whose `domain.json` has an
> `outline` block — `fiction` and `serial-fiction` do, `design-doc` does
> not. On a domain without one, skip every outline step below.

- [ ] **Step 2: Add the `/redliner:run outline` section**

Insert immediately before `## /redliner:run continuity`:

```markdown
## `/redliner:run outline`

A scene-level view of the plot: what each scene's driving character was
trying to achieve, what opposed them, and what changed. It exists to
answer two questions without rereading the book — can this scene move,
and can it be cut.

Callable directly, and also the first thing `assess` refreshes (see
above). This section is the one definition both refer to.

Like continuity and unlike the two editing phases, this is **not
phase-gated**: recording is judgment-free, so it is safe any time after
intake — including on chapter three of a draft nowhere near a
developmental pass. It tracks its own staleness per section.

**Skip this entirely on a domain with no `outline` block.**

1. Check which sections need re-recording (`redliner outline stale
   <dir>`, or the `outline_stale` tool). The result drives everything
   below:
   - `needs_recording` — sections to (re-)record this run.
   - `current_hashes` — each of those sections' current SHA-256, keyed by
     stem. The outliner needs this exact value; don't compute it
     yourself or reuse a stale one.
   - `orphaned_sections` — outline files whose section no longer exists.
     Delete `.redliner/outline/sections/<stem>.json` for each before
     joining. A cut section's scenes must not still appear in the
     outline.
2. If `needs_recording` is empty, skip to step 5 — nothing has changed
   since the last recording. **Say so in one line rather than silently
   doing nothing**; an author who just asked for an outline and got no
   output cannot tell "nothing changed" from "it failed".
3. For each section in `needs_recording`, Task the
   `redliner:<domain>-outliner` subagent with: the manuscript directory,
   that section's file path, its hash from `current_hashes`, and output
   path `.redliner/outline/sections/<section_stem>.json`. Sections share
   no state — parallel is fine, sequential keeps the transcript
   readable.
4. Validate — stop and report errors rather than joining from bad
   recordings.
5. Join (`redliner outline join <dir>`), then render (`redliner outline
   render <dir>`). Both are deterministic commands, not agent calls.
   The join rebuilds `outline.json` from **every** current section file,
   not only the ones just re-recorded.
6. Archive a version if anything changed. A run that changed nothing
   archives nothing.
7. Report: the scene count, which sections were re-recorded, and
   **the absolute path to `Outline.md`**. The rendered outline is the
   author's deliverable, and it lives in the manuscript folder beside
   the chapters — not in `.redliner/`, which Finder hides by default.
   Telling them only that "the outline is written" is how a run ends
   with the author unable to find its one output.

### Why re-running this is cheap

Worth saying to the author once, because "regenerate the outline" sounds
expensive and is not.

A re-run costs a deterministic staleness check (free), **one agent call
per section whose text actually changed**, then a deterministic join and
render (free). Writing chapter 12 and re-running is one call — chapters
1–11 are never opened. So outlining after every chapter and outlining
after five chapters cost the same in total; they differ only in when the
author sees the view.

Encourage the frequent version. It is the workflow this layer was built
for.

### Version history

Every outline run whose content actually changed archives a version to
`.redliner/outline/versions/v<N>/`, holding the JSON, the rendered
`Outline.md`, and a small `meta.json` recording the date and which
sections changed.

`redliner outline versions <dir>` lists them, and reading an old one
means opening its archived `Outline.md` — whose path the listing prints.
Use this when the author asks what the outline looked like before.

**Never delete anything under `versions/` without asking the author
first**, same rule as `rounds/`. A no-op run archives nothing, so the
growth is bounded by how often the text actually changes.

What this does *not* yet answer is what changed *between* two versions.
That's a diff tool, deliberately not built yet — see TODO.md. Say that
plainly rather than hand-diffing two archived files into a summary the
author might take as authoritative.
```

- [ ] **Step 3: Insert the refresh into the `assess` flow**

This is the step with a real ordering hazard. The existing flow is: step 1 moves to the developmental phase and increments the round counter; step 2 archives the previous round before clearing anything; step 3 tasks the developmental editor.

Renumber so the outline work lands **between** the existing steps 2 and 3, and add to the new step 3:

```markdown
3. **Archive the outline, then refresh it.** In that order.

   Archive first: `redliner rounds archive <dir> outline`. Then run the
   **outline** steps below in full (recording, join, render, version
   archive).

   The order is load-bearing and the failure is silent. Refreshing
   overwrites `outline.json`, and unlike `continuity.json` — which is
   deterministically rebuildable from the per-section observations —
   the outline is a join of agent output. Overwrite it without archiving
   and that round's recorded scene structure is gone for good, leaving
   a hole in the version history the author may later want to look back
   through. This is the same failure step 2 exists to prevent for
   findings: *every pass rewrites in place, so clearing without
   archiving leaves nothing to compare a later round against.*

   Do this even if the author "just ran the outline." It is hash-driven
   and idempotent, so on an unchanged manuscript it costs almost
   nothing — and a stale outline handed to the developmental editor
   produces confident findings reasoned from a structure the prose no
   longer has, which looks exactly like a good pass. Never treat a
   fresh outline as a precondition the author is trusted to have met.

   **Skip this step entirely on a domain with no `outline` block.**
```

Then, in the step that tasks the developmental editor (now step 4), add:

```markdown
   On a domain with an outline, give the subagent the path to
   `.redliner/outline/outline.json` as well, and tell it this is a
   structural spine to read **alongside** the prose, never instead of
   it. It saves re-deriving scene structure from the text every round
   and makes arc-level questions legible; it is not a substitute input,
   and a developmental pass that reads only the outline is not a
   developmental pass.
```

Finally, in the closing step that archives and records passes, add `redliner rounds archive <dir> outline` alongside the developmental and continuity archives, with this note:

```markdown
   Archiving the outline in both places is not redundant: this one
   preserves the completed round, and step 3's is the safety net for a
   round that ended without reaching here. `freeArchiveDir` suffixes
   `.2`, `.3` for exactly this case.

   Do **not** run `redliner state pass <dir> outline` — that kind
   deliberately does not exist. The outline refreshes automatically
   inside every assess, so recording it as a completed pass would make
   `status` report it as run permanently, which is a constant rather
   than a signal. What `status` reports for this layer is staleness.
```

- [ ] **Step 4: Extend `status`**

In `## /redliner:run status`, add to the list of what to show:

```markdown
- **Whether any sections need re-recording for the outline**, the same
  way continuity staleness is reported, and how many versions are
  archived. On a domain with no `outline` block, show nothing — not an
  empty section, which reads as a broken feature rather than an
  inapplicable one.
```

- [ ] **Step 5: Verify the parity guard still passes**

This step exists because Step 2 and Step 3 wrote `redliner outline <command>` strings into a skill file, and the guard reads skill files as text.

Run: `cd go && go test ./internal/mcpserver/ -run TestEverySkillCommandHasAnMCPTool -v`
Expected: PASS. If it fails naming an `outline` command, the mapping added in Task 11 Step 5 is missing or misspelled — fix the map, not the skill prose.

- [ ] **Step 6: Run the whole suite and commit**

Run: `cd go && go test ./...`
Expected: all `ok`.

```bash
git add skills/run/SKILL.md
git commit -m "Wire the outline layer into the run skill"
```

---

### Task 14: Ask for the published boundary at intake

**Files:**
- Modify: `skills/intake/SKILL.md`

**Interfaces:**
- Consumes: `State.PublishedThrough` from Task 3.
- Produces: the field being set on real manuscripts, without which the boundary never renders.

- [ ] **Step 1: Read the skill and find where domain-specific fields are asked**

Run: `grep -n "brief_fields\|domain\|serial" skills/intake/SKILL.md | head -30`

The brief fields come from `domain.json`. `published_through` is **not** a brief field — it lives in state, changes over time as chapters ship, and is machine-read. Find where the skill writes state (or where it would need to) rather than adding it to the brief-field loop.

- [ ] **Step 2: Add the question**

Add a step, asked **only** when the manuscript's domain is `serial-fiction`:

```markdown
## Published installments (serial domains only)

Ask only if the domain is `serial-fiction`:

> Which chapters have already gone out to readers?

Record the last published section's stem in state's `published_through`
(e.g. `section_11`). Leave it unset if nothing has been published yet —
a serial being drafted before launch, which is a normal answer.

This draws a visible line in `Outline.md` between what has shipped and
what is still movable. It matters because a serial has a constraint a
novel does not: once a chapter goes out, the author does not revise it,
so a scene above that line cannot be moved or cut at all. That is the
single most load-bearing fact in a view whose purpose is deciding what
to move and what to cut.

It's a section boundary, not a scene one — publication happens per
installment, and there is no such thing as half a chapter being live.

Tell the author it changes as chapters ship, and that they should say so
when it does. A stale boundary is worse than none: it shows scenes as
frozen that are still theirs to change.
```

- [ ] **Step 3: Check the parity guard**

The new prose must not accidentally contain a `redliner <group> <command>` string with no mapped tool.

Run: `cd go && go test ./internal/mcpserver/ -run TestEverySkillCommandHasAnMCPTool -v`
Expected: PASS.

- [ ] **Step 4: Run the whole suite and commit**

Run: `cd go && go test ./...`
Expected: all `ok`.

```bash
git add skills/intake/SKILL.md
git commit -m "Ask serial authors which chapters have shipped"
```

---

### Task 15: `redliner state published` — move the boundary as chapters ship

**Files:**
- Modify: `go/internal/cli/state.go` (`RunState`'s switch and usage, ~line 25; add `cmdStatePublished` beside `cmdStateStage`, ~line 215)
- Modify: `go/internal/mcpserver/server.go`, `go/internal/mcpserver/descriptions.go`, `go/internal/mcpserver/frontdoor_parity_test.go`
- Test: `go/internal/cli/state_test.go`

**Interfaces:**
- Consumes: `State.PublishedThrough` from Task 3; `schemas.SectionFiles` (existing).
- Produces: `redliner state published <manuscript_dir> <section_stem>` and the MCP tool `state_published`.

Intake (Task 14) asks once. This is what keeps the answer true: a serial ships a chapter a week, and a stale boundary is worse than none — it renders scenes as frozen that are still the author's to change.

Validate the stem against the manuscript's actual sections rather than accepting any string. A typo that silently sets `published_through` to a section that does not exist draws no line at all, and that reads as the feature being broken rather than the input being wrong.

- [ ] **Step 1: Write the failing test**

Append to `go/internal/cli/state_test.go`:

```go
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
```

Add `"strings"`, `"bytes"`, and the `schemas` import to that test file if not already present.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd go && go test ./internal/cli/ -run TestStatePublished -v`
Expected: FAIL — `Unknown command 'published'`.

- [ ] **Step 3: Implement the command**

In `go/internal/cli/state.go`, add to the usage string:

```
  redliner state published <manuscript_dir> <section_stem|none>
```

add to `RunState`'s switch:

```go
	case "published":
		if len(args) < 3 {
			fmt.Fprintln(stdout, stateUsage)
			return 1
		}
		return cmdStatePublished(args[1], args[2], stdout)
```

Match the surrounding cases' exact argument-indexing convention — read `state phase`'s case and mirror it rather than assuming this indexing is right.

Then add beside `cmdStateStage`:

```go
// cmdStatePublished records which installments have shipped. Serial
// fiction has a constraint a novel does not -- once a chapter goes out
// the author does not revise it -- so a scene above this line cannot be
// moved or cut, which is the one fact the rendered outline most needs to
// show.
//
// Validated against the manuscript's real sections rather than accepting
// any string: a typo sets a boundary matching no section, which draws no
// line at all, and that reads as the feature being broken rather than the
// input being wrong.
func cmdStatePublished(manuscriptDir, stem string, stdout io.Writer) int {
	state, ok := requireState(manuscriptDir, stdout)
	if !ok {
		return 1
	}

	// "none" is how an author says nothing has shipped yet -- a serial
	// being drafted before launch, and the correct state for any novel.
	if stem == "none" {
		state.PublishedThrough = ""
		if _, err := schemas.SaveState(manuscriptDir, state); err != nil {
			fmt.Fprintf(stdout, "Error writing state: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, "Published boundary cleared -- nothing is marked as shipped.")
		return 0
	}

	paths, err := schemas.SectionFiles(manuscriptDir)
	if err != nil {
		return reportSectionError(err, stdout)
	}
	var stems []string
	found := false
	for _, path := range paths {
		s := stemOfPath(path)
		stems = append(stems, s)
		if s == stem {
			found = true
		}
	}
	if !found {
		fmt.Fprintf(stdout, "No section %s in %s. Sections are: %s\n",
			pyReprStr(stem), manuscriptDir, strings.Join(stems, ", "))
		return 1
	}

	state.PublishedThrough = stem
	if _, err := schemas.SaveState(manuscriptDir, state); err != nil {
		fmt.Fprintf(stdout, "Error writing state: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Published through %s. Everything up to and including it renders as shipped in Outline.md.\n", stem)
	return 0
}
```

Add `"strings"` to `state.go`'s imports if it is not already there.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd go && go test ./internal/cli/ -run TestStatePublished -v`
Expected: PASS, all three tests.

- [ ] **Step 5: Add the MCP tool**

In `descriptions.go`:

```go
const descStatePublished = `Record which installment the serial has shipped through, or "none" to clear it. Everything up to and including that section renders as published in Outline.md, where it reads as no longer movable or cuttable.`
```

In `server.go`, add the input shape beside the others:

```go
type statePublishedInput struct {
	ManuscriptDir string `json:"manuscript_dir"`
	Section       string `json:"section"`
}
```

register it:

```go
	mcp.AddTool(srv, &mcp.Tool{Name: "state_published", Description: descStatePublished}, s.statePublished)
```

and add the handler, mirroring the existing `stateStage` handler's shape — read it and match it rather than inventing a second convention:

```go
func (s *redlinerServer) statePublished(_ context.Context, _ *mcp.CallToolRequest, in statePublishedInput) (*mcp.CallToolResult, any, error) {
	var out bytes.Buffer
	code := cli.RunState([]string{"published", in.ManuscriptDir, in.Section}, &out)
	if code != 0 {
		return nil, errorResult("%s", strings.TrimSpace(out.String())), nil
	}
	return nil, map[string]any{"output": strings.TrimSpace(out.String())}, nil
}
```

In `frontdoor_parity_test.go`, add to `commandToTool`:

```go
	"state published": "state_published",
```

- [ ] **Step 6: Point intake and the run skill at the command**

In `skills/intake/SKILL.md`, in the section added by Task 14, name the command explicitly so the answer is actually recorded rather than only discussed:

> Record it with `redliner state published <dir> <section_stem>` (or the
> `state_published` tool), and `... none` if nothing has shipped yet.

In `skills/run/SKILL.md`'s `/redliner:run outline` section, add one line to the reporting step:

> If the author mentions that more chapters have gone out since last
> time, update the boundary with `redliner state published <dir>
> <section_stem>` before rendering — a stale boundary shows scenes as
> frozen that are still theirs to change.

- [ ] **Step 7: Run the whole suite and commit**

Run: `cd go && go test ./...`
Expected: all `ok`, including `TestEverySkillCommandHasAnMCPTool` — Step 6 wrote `redliner state published` into two skill files, and Step 5 is what keeps that legal.

```bash
git add go/internal/cli/state.go go/internal/cli/state_test.go go/internal/mcpserver/ \
        skills/intake/SKILL.md skills/run/SKILL.md
git commit -m "Add redliner state published so the boundary can move as chapters ship"
```

---

### Task 16: End-to-end verification on the sample manuscript


**Files:**
- No source changes expected. If this task finds a defect, fix it here with a test that would have caught it.

**Interfaces:**
- Consumes: everything.
- Produces: evidence the layer works on a real manuscript, not only in unit tests.

This exists because every prior task verified one seam. Nothing has yet run the whole layer end to end, and this project's habit is to verify by running rather than by reading the code back.

- [ ] **Step 1: Build the binary**

```bash
cd go && go build -o /tmp/redliner ./cmd/redliner
```
Expected: no output, binary written.

- [ ] **Step 2: Work on a copy, never the checked-in sample**

```bash
rm -rf /tmp/outline-check && cp -R sample_manuscript /tmp/outline-check
```

- [ ] **Step 3: Confirm staleness reports both sections**

```bash
REDLINER_DOMAINS_DIR="$PWD/domains" /tmp/redliner outline stale /tmp/outline-check
```
Expected: JSON with `needs_recording` listing `section_01` and `section_02`, a 64-character hash for each in `current_hashes`, and empty `orphaned_sections`.

- [ ] **Step 4: Hand-write two outline sections**

Recording is an agent's job and this is a mechanical check, so write the files by hand using the hashes from Step 3. Substitute the real hashes:

```bash
mkdir -p /tmp/outline-check/.redliner/outline/sections
cat > /tmp/outline-check/.redliner/outline/sections/section_01.json <<'JSON'
{
  "section": "section_01",
  "section_sha256": "PASTE_SECTION_01_HASH",
  "scenes": [
    {
      "order": 1,
      "pov": "placeholder",
      "anchor": "first few words",
      "goal": "A stated intention.",
      "conflict": "What opposed it.",
      "outcome": "What changed."
    }
  ]
}
JSON
```

Do the same for `section_02`. Then paste the real hashes in:

```bash
cd /tmp/outline-check
python3 - <<'PY'
import json, pathlib, subprocess, os
stale = json.loads(subprocess.check_output(
    ["/tmp/redliner", "outline", "stale", "."],
    env={**os.environ, "REDLINER_DOMAINS_DIR": os.environ["RL_DOMAINS"]}))
for stem, h in stale["current_hashes"].items():
    p = pathlib.Path(".redliner/outline/sections") / f"{stem}.json"
    if p.exists():
        d = json.loads(p.read_text())
        d["section_sha256"] = h
        p.write_text(json.dumps(d, indent=2) + "\n")
        print("fixed", p)
PY
```
Run it with `RL_DOMAINS="$OLDPWD/domains"` set.

- [ ] **Step 5: Verify staleness is now empty**

```bash
REDLINER_DOMAINS_DIR="$PWD/domains" /tmp/redliner outline stale /tmp/outline-check
```
Expected: `needs_recording` is `[]`. This is the SHA-skip actually working — the claim the whole cost argument rests on.

- [ ] **Step 6: Validate, join, render**

```bash
cd /Volumes/T7/code/ideas/redliner
REDLINER_DOMAINS_DIR="$PWD/domains" /tmp/redliner validate /tmp/outline-check
REDLINER_DOMAINS_DIR="$PWD/domains" /tmp/redliner outline join /tmp/outline-check
REDLINER_DOMAINS_DIR="$PWD/domains" /tmp/redliner outline render /tmp/outline-check
```
Expected: validate prints `OK` lines including both `sections/section_0N.json`; join reports 2 sections and 2 scenes; render prints an absolute path ending `/tmp/outline-check/Outline.md`.

- [ ] **Step 7: Read the rendered outline**

```bash
cat /tmp/outline-check/Outline.md
```
Expected: a `# Outline` heading, a count line, `## section_01` and `## section_02`, and each scene's Goal/Conflict/Outcome. The sample manuscript's domain is `fiction`, so there must be **no** `Leaves open:` line and **no** published boundary.

Judge it as an author would: can you tell from this alone which scene could be cut? If not, the renderer is the thing to fix, not the reader.

- [ ] **Step 8: Confirm the boundary renders for a serial**

```bash
python3 - <<'PY'
import json, pathlib
p = pathlib.Path("/tmp/outline-check/.redliner/state.json")
s = json.loads(p.read_text())
s["domain"] = "serial-fiction"
p.write_text(json.dumps(s, indent=2) + "\n")
PY
# The boundary goes through the real command (Task 15).
REDLINER_DOMAINS_DIR="$PWD/domains" /tmp/redliner state published /tmp/outline-check section_01
REDLINER_DOMAINS_DIR="$PWD/domains" /tmp/redliner outline render /tmp/outline-check
cat /tmp/outline-check/Outline.md
```
Expected: the published line appears **after** `## section_01`'s scenes and **before** `## section_02`.

Note: validate will now fail these files, because serial-fiction configures `leaves_open` and the hand-written files lack it. That is the validator doing its job — confirm the failure names `leaves_open`, then move on.

- [ ] **Step 9: Confirm version archiving and the listing**

```bash
REDLINER_DOMAINS_DIR="$PWD/domains" /tmp/redliner outline versions /tmp/outline-check
```
Expected: at this point, "No outline versions archived yet" — nothing has called `ArchiveOutlineVersion`, because no run-skill flow has executed. This confirms the archive is driven by the skill flow, not as a side effect of `join`.

If you want to exercise the archive directly, the unit tests in Task 8 already do. Do not add a CLI command to trigger it manually — the skill flow is its only caller by design.

- [ ] **Step 10: Confirm `Outline.md` is not discovered as a section**

```bash
REDLINER_DOMAINS_DIR="$PWD/domains" /tmp/redliner state diff /tmp/outline-check
```
Expected: the verdict mentions only `section_01`/`section_02`. `Outline.md` must appear nowhere — it is not manuscript text.

- [ ] **Step 11: Clean up and record the result**

```bash
rm -rf /tmp/outline-check /tmp/redliner
cd go && go test ./...
```
Expected: all four packages `ok`.

If any step above revealed a defect, fix it **and add the unit test that would have caught it** before committing. A live check that finds a bug and is not turned into a test leaves the bug free to come back.

- [ ] **Step 12: Commit any fixes**

```bash
git add -A
git commit -m "Fix defects found verifying the outline layer end to end"
```

(If nothing needed fixing, skip this commit and say so — an empty commit records nothing useful.)
