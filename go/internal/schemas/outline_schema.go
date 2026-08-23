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
		if !isNumber || order != float64(i+1) {
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
