package schemas

import (
	"path/filepath"
	"testing"
)

// TestValidators_HappyFixture_AllClean checks the validators against the
// same real files capture_baseline.py's golden/happy/07_validate_findings
// step ran through Python's validate_findings.py and got "OK" for every
// one of -- see harness/golden/happy/07_validate_findings.json. If any
// of these report errors, the Go validators are stricter (or looser)
// than the Python originals on data the Python side accepts cleanly.
func TestValidators_HappyFixture_AllClean(t *testing.T) {
	fiction, err := LoadDomain(domainsDir(t), "fiction")
	if err != nil {
		t.Fatalf("LoadDomain(fiction): %v", err)
	}
	continuity := fiction.Continuity()
	entityTypes := continuity.StringSet("entity_types")
	sources := continuity.StringSet("sources")
	categories := continuity.StringSet("categories")
	devCategories := fiction.StringSet("developmental_categories")
	lineCategories := fiction.StringSet("line_categories")

	redliner := filepath.Join(fixturesDir(t), "happy", ".redliner")

	for _, obs := range []string{"section_01", "section_02"} {
		report := loadJSONFile(t, filepath.Join(redliner, "canon", "observations", obs+".json"))
		if errs := ValidateObservations(report, entityTypes, sources); len(errs) != 0 {
			t.Errorf("ValidateObservations(%s): expected no errors, got %v", obs, errs)
		}
	}

	continuityReport := loadJSONFile(t, filepath.Join(redliner, "canon", "continuity.json"))
	if errs := ValidateContinuityReport(continuityReport, categories); len(errs) != 0 {
		t.Errorf("ValidateContinuityReport: expected no errors, got %v", errs)
	}

	devReport := loadJSONFile(t, filepath.Join(redliner, "findings", "developmental.json"))
	if errs := ValidateDevelopmentalReport(devReport, devCategories); len(errs) != 0 {
		t.Errorf("ValidateDevelopmentalReport: expected no errors, got %v", errs)
	}

	letter := loadJSONFile(t, filepath.Join(redliner, "findings", "editorial_letter.json"))
	if errs := ValidateEditorialLetter(letter); len(errs) != 0 {
		t.Errorf("ValidateEditorialLetter: expected no errors, got %v", errs)
	}

	lineReport := loadJSONFile(t, filepath.Join(redliner, "findings", "line_section_01.json"))
	if errs := ValidateLineReport(lineReport, lineCategories); len(errs) != 0 {
		t.Errorf("ValidateLineReport: expected no errors, got %v", errs)
	}
}

// TestValidators_RejectMalformedInput exercises the failure paths at
// least structurally (no golden Python data for these yet -- see
// go/harness/README.md's note that malformed-input fixtures, and exact
// error-text parity for them, are follow-up work, not covered by Phase 2).
func TestValidators_RejectMalformedInput(t *testing.T) {
	if errs := ValidateObservations(map[string]interface{}{}, map[string]bool{}, map[string]bool{}); len(errs) == 0 {
		t.Error("empty observations report should fail validation")
	}

	badFact := map[string]interface{}{
		"section":        "section_01",
		"section_sha256": "abc123",
		"facts": []interface{}{
			map[string]interface{}{
				"id":          "not-a-valid-id",
				"entity":      "Mira",
				"entity_type": "character",
				"attribute":   "eye_color",
				"value":       "blue",
				"excerpt":     "her blue eyes",
				"location":    "para 3",
				"source":      "narration",
				"confidence":  "explicit",
				"opinion":     "this seems wrong", // not in FACT_REQUIRED_KEYS -- must be rejected
			},
		},
	}
	errs := ValidateObservations(badFact, map[string]bool{"character": true}, map[string]bool{"narration": true})
	if len(errs) == 0 {
		t.Error("a fact with a bad id and an extra 'opinion' key should fail validation")
	}

	if errs := ValidateEditorialLetter(map[string]interface{}{"summary": ""}); len(errs) == 0 {
		t.Error("an editorial letter missing required fields should fail validation")
	}
}
