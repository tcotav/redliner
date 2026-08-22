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
	//
	// "rating" and "note2" are included even though they are not judgment
	// words: the schema is a closed whitelist, not a blacklist of these
	// four names. Without a non-judgment key in this list, the test would
	// pass identically against a hardcoded blacklist of exactly these
	// words instead of the whitelist the implementation actually uses.
	for _, key := range []string{"note", "severity", "concern", "suggestion", "rating", "note2"} {
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
