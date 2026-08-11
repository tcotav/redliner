package schemas

import (
	"fmt"
	"regexp"
)

// Severities and Statuses are universal across domains -- every
// finding/contradiction carries one of each regardless of vocabulary.
// Mirrors findings_schema.py's SEVERITIES/STATUSES exactly.
var Severities = map[string]bool{"minor": true, "moderate": true, "major": true, "critical": true}

var Statuses = map[string]bool{
	"open":      true, // raised, not yet addressed
	"addressed": true, // author revised; verified by a re-check pass
	"claimed":   true, // author says it's addressed; not yet verified
	"stale":     true, // manuscript changed so much this finding no longer applies as written
	"wontfix":   true, // author considered it and declined; don't re-raise
}

// DeferredCategory is a protocol-level marker (developmental observed
// something prose-level and is handing it to the line pass), not domain
// content -- fixed across domains, same as findings_schema.py.
const DeferredCategory = "deferred_to_line"

var devIDPattern = regexp.MustCompile(`^dev-\d{3}$`)
var lineIDPattern = regexp.MustCompile(`^line-[a-z0-9_]+-\d{3}$`)

func checkFinding(findingRaw interface{}, categories map[string]bool, index int, idPattern *regexp.Regexp, seenIDs map[string]bool) []string {
	prefix := fmt.Sprintf("findings[%d]", index)
	finding, ok := asObject(findingRaw)
	if !ok {
		return []string{prefix + ": not an object"}
	}

	var errors []string

	id := asString(finding["id"])
	if id == "" || !idPattern.MatchString(id) {
		errors = append(errors, fmt.Sprintf("%s: id %s does not match %s", prefix, pyRepr(finding["id"]), idPattern.String()))
	} else if seenIDs[id] {
		errors = append(errors, fmt.Sprintf("%s: duplicate id %s", prefix, pyRepr(id)))
	} else {
		seenIDs[id] = true
	}

	status := asString(finding["status"])
	if !Statuses[status] {
		errors = append(errors, fmt.Sprintf("%s: status %s not in %s", prefix, pyRepr(finding["status"]), pySetRepr(Statuses)))
	}

	category := asString(finding["category"])
	if !categories[category] {
		errors = append(errors, fmt.Sprintf("%s: category %s not in %s", prefix, pyRepr(finding["category"]), pySetRepr(categories)))
	}

	severity := asString(finding["severity"])
	if !Severities[severity] {
		errors = append(errors, fmt.Sprintf("%s: severity %s not in %s", prefix, pyRepr(finding["severity"]), pySetRepr(Severities)))
	}

	if isBlank(finding["location"]) {
		errors = append(errors, fmt.Sprintf("%s: missing/empty 'location'", prefix))
	}
	if isBlank(finding["note"]) {
		errors = append(errors, fmt.Sprintf("%s: missing/empty 'note'", prefix))
	}

	return errors
}

// ValidateDevelopmentalReport mirrors findings_schema.py's
// validate_developmental_report. `categories` is the domain's
// developmental-phase category set -- DeferredCategory is allowed
// automatically, not part of what the caller passes.
func ValidateDevelopmentalReport(reportRaw interface{}, categories map[string]bool) []string {
	report, ok := asObject(reportRaw)
	if !ok {
		return []string{"report is not a JSON object"}
	}

	var errors []string

	if isBlank(report["scope"]) {
		errors = append(errors, "missing/empty 'scope'")
	}

	round, isNumber := report["round"].(float64) // JSON numbers decode as float64
	if !isNumber || round < 1 {
		errors = append(errors, "'round' must be an integer >= 1")
	}

	// A developmental pass runs unattended -- ambiguity the brief didn't
	// cover gets recorded here instead of guessed at silently. Each
	// entry is a gap to fix in the brief, not a question to answer live.
	assumptionsRaw, ok := report["assumptions"].([]interface{})
	if !ok {
		errors = append(errors, "'assumptions' must be a list (empty if the brief covered everything)")
	} else {
		for i, raw := range assumptionsRaw {
			item, ok := asObject(raw)
			if !ok {
				errors = append(errors, fmt.Sprintf("assumptions[%d]: not an object", i))
				continue
			}
			for _, key := range []string{"assumption", "because", "affects"} {
				if _, present := item[key]; !present {
					errors = append(errors, fmt.Sprintf("assumptions[%d]: missing %s", i, pyRepr(key)))
				}
			}
			if affects, present := item["affects"]; present {
				if _, ok := affects.([]interface{}); !ok {
					errors = append(errors, fmt.Sprintf("assumptions[%d]: 'affects' must be a list of finding ids", i))
				}
			}
		}
	}

	findingsRaw, ok := report["findings"].([]interface{})
	if !ok {
		return append(errors, "'findings' is not a list")
	}

	allowed := map[string]bool{DeferredCategory: true}
	for k := range categories {
		allowed[k] = true
	}

	seenIDs := map[string]bool{}
	for i, f := range findingsRaw {
		errors = append(errors, checkFinding(f, allowed, i, devIDPattern, seenIDs)...)
	}
	return errors
}

// ValidateLineReport mirrors validate_line_report. `categories` is the
// domain's line-phase category set.
func ValidateLineReport(reportRaw interface{}, categories map[string]bool) []string {
	report, ok := asObject(reportRaw)
	if !ok {
		return []string{"report is not a JSON object"}
	}

	var errors []string
	if isBlank(report["section"]) {
		errors = append(errors, "missing/empty 'section'")
	}

	findingsRaw, ok := report["findings"].([]interface{})
	if !ok {
		return append(errors, "'findings' is not a list")
	}

	seenIDs := map[string]bool{}
	for i, f := range findingsRaw {
		errors = append(errors, checkFinding(f, categories, i, lineIDPattern, seenIDs)...)
	}
	return errors
}

// ValidateEditorialLetter mirrors validate_editorial_letter.
func ValidateEditorialLetter(letterRaw interface{}) []string {
	letter, ok := asObject(letterRaw)
	if !ok {
		return []string{"letter is not a JSON object"}
	}

	var errors []string
	for _, key := range []string{"summary", "developmental_notes", "line_notes"} {
		if isBlank(letter[key]) {
			errors = append(errors, fmt.Sprintf("missing/empty %s", pyRepr(key)))
		}
	}

	topPriorities, ok := letter["top_priorities"].([]interface{})
	if !ok || len(topPriorities) == 0 {
		errors = append(errors, "'top_priorities' must be a non-empty list")
	}

	return errors
}
