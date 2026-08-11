package schemas

import (
	"fmt"
	"regexp"
)

// Confidences and ContradictionKinds are universal across domains, same
// split as canon_schema.py: "her green eyes" is explicit; "she reached
// the top shelf easily" implies height. Implied facts shouldn't drive
// hard contradictions on their own.
var Confidences = map[string]bool{"explicit": true, "implied": true}

var ContradictionKinds = map[string]bool{
	"contradiction": true, // two assertions that cannot both be true
	"unverified":    true, // looks wrong, but needs the author -- lying
	// character, unreliable narrator, or possible in-world explanation
}

// factRequiredKeys is deliberately exhaustive with no optional
// extension point -- see canon_schema.py's FACT_OPTIONAL_KEYS comment
// for why: adding an optional key here is how this schema stops being
// judgment-free.
var factRequiredKeys = map[string]bool{
	"id": true, "entity": true, "entity_type": true, "attribute": true,
	"value": true, "excerpt": true, "location": true, "source": true,
	"confidence": true,
}

var factIDPattern = regexp.MustCompile(`^fact-[a-z0-9_]+-\d{3}$`)
var contradictionIDPattern = regexp.MustCompile(`^cont-\d{3}$`)

func checkFact(factRaw interface{}, entityTypes, sources map[string]bool, index int, seenIDs map[string]bool) []string {
	prefix := fmt.Sprintf("facts[%d]", index)
	fact, ok := asObject(factRaw)
	if !ok {
		return []string{prefix + ": not an object"}
	}

	var errors []string

	var missing []string
	for k := range factRequiredKeys {
		if _, present := fact[k]; !present {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		errors = append(errors, fmt.Sprintf("%s: missing keys %s", prefix, pyList(missing)))
	}

	// The point of the whole schema: no room for an opinion.
	var extra []string
	for k := range fact {
		if !factRequiredKeys[k] {
			extra = append(extra, k)
		}
	}
	if len(extra) > 0 {
		errors = append(errors, fmt.Sprintf("%s: unexpected keys %s — extraction records facts, not judgments", prefix, pyList(extra)))
	}

	id := asString(fact["id"])
	if id == "" || !factIDPattern.MatchString(id) {
		errors = append(errors, fmt.Sprintf("%s: id %s does not match %s", prefix, pyRepr(fact["id"]), factIDPattern.String()))
	} else if seenIDs[id] {
		errors = append(errors, fmt.Sprintf("%s: duplicate id %s", prefix, pyRepr(id)))
	} else {
		seenIDs[id] = true
	}

	entityType := asString(fact["entity_type"])
	if !entityTypes[entityType] {
		errors = append(errors, fmt.Sprintf("%s: entity_type %s not in %s", prefix, pyRepr(fact["entity_type"]), pySetRepr(entityTypes)))
	}

	source := asString(fact["source"])
	if !sources[source] {
		errors = append(errors, fmt.Sprintf("%s: source %s not in %s", prefix, pyRepr(fact["source"]), pySetRepr(sources)))
	}

	confidence := asString(fact["confidence"])
	if !Confidences[confidence] {
		errors = append(errors, fmt.Sprintf("%s: confidence %s not in %s", prefix, pyRepr(fact["confidence"]), pySetRepr(Confidences)))
	}

	for _, key := range []string{"entity", "attribute", "value", "excerpt", "location"} {
		if _, present := fact[key]; present && isBlank(fact[key]) {
			errors = append(errors, fmt.Sprintf("%s: missing/empty %s", prefix, pyRepr(key)))
		}
	}

	return errors
}

// ValidateObservations validates one section's extracted facts.
// `entityTypes`/`sources` come from the manuscript's domain config.
// Mirrors canon_schema.py's validate_observations.
func ValidateObservations(reportRaw interface{}, entityTypes, sources map[string]bool) []string {
	report, ok := asObject(reportRaw)
	if !ok {
		return []string{"observations file is not a JSON object"}
	}

	var errors []string
	if isBlank(report["section"]) {
		errors = append(errors, "missing/empty 'section'")
	}
	// Recorded so the skill can skip re-extracting unchanged sections.
	if isBlank(report["section_sha256"]) {
		errors = append(errors, "missing/empty 'section_sha256'")
	}

	factsRaw, ok := report["facts"].([]interface{})
	if !ok {
		return append(errors, "'facts' is not a list")
	}

	seenIDs := map[string]bool{}
	for i, f := range factsRaw {
		errors = append(errors, checkFact(f, entityTypes, sources, i, seenIDs)...)
	}
	return errors
}

// ValidateContinuityReport validates adjudicated contradictions.
// `categories` is the domain's continuity-category set. Mirrors
// canon_schema.py's validate_continuity_report.
func ValidateContinuityReport(reportRaw interface{}, categories map[string]bool) []string {
	report, ok := asObject(reportRaw)
	if !ok {
		return []string{"continuity report is not a JSON object"}
	}

	contradictionsRaw, ok := report["contradictions"].([]interface{})
	if !ok {
		return []string{"'contradictions' is not a list"}
	}

	var errors []string
	seenIDs := map[string]bool{}
	for i, itemRaw := range contradictionsRaw {
		prefix := fmt.Sprintf("contradictions[%d]", i)
		item, ok := asObject(itemRaw)
		if !ok {
			errors = append(errors, prefix+": not an object")
			continue
		}

		id := asString(item["id"])
		if id == "" || !contradictionIDPattern.MatchString(id) {
			errors = append(errors, fmt.Sprintf("%s: id %s does not match %s", prefix, pyRepr(item["id"]), contradictionIDPattern.String()))
		} else if seenIDs[id] {
			errors = append(errors, fmt.Sprintf("%s: duplicate id %s", prefix, pyRepr(id)))
		} else {
			seenIDs[id] = true
		}

		status := asString(item["status"])
		if !Statuses[status] {
			errors = append(errors, fmt.Sprintf("%s: status %s not in %s", prefix, pyRepr(item["status"]), pySetRepr(Statuses)))
		}

		kind := asString(item["kind"])
		if !ContradictionKinds[kind] {
			errors = append(errors, fmt.Sprintf("%s: kind %s not in %s", prefix, pyRepr(item["kind"]), pySetRepr(ContradictionKinds)))
		}

		category := asString(item["category"])
		if !categories[category] {
			errors = append(errors, fmt.Sprintf("%s: category %s not in %s", prefix, pyRepr(item["category"]), pySetRepr(categories)))
		}

		severity := asString(item["severity"])
		if !Severities[severity] {
			errors = append(errors, fmt.Sprintf("%s: severity %s not in %s", prefix, pyRepr(item["severity"]), pySetRepr(Severities)))
		}

		factIDs, ok := item["fact_ids"].([]interface{})
		if !ok || len(factIDs) < 2 {
			errors = append(errors, fmt.Sprintf("%s: 'fact_ids' must list at least the two conflicting facts", prefix))
		}

		if isBlank(item["note"]) {
			errors = append(errors, fmt.Sprintf("%s: missing/empty 'note'", prefix))
		}
	}
	return errors
}
