package schemas

import (
	"fmt"
	"sort"
	"strings"
)

// pyRepr approximates Python's repr() for the value types that actually
// show up in these validators' error messages (strings from parsed
// JSON, or nil for an absent key) -- not a general-purpose repr. Used so
// error text reads the same shape as the Python original
// (`category 'foo' not in [...]` rather than a Go %v dump), since the
// harness's own comparison rule treats human-facing CLI/validator output
// as a real compatibility surface, not an implementation detail.
func pyRepr(v interface{}) string {
	switch val := v.(type) {
	case nil:
		return "None"
	case string:
		return "'" + strings.ReplaceAll(val, "'", "\\'") + "'"
	default:
		return fmt.Sprintf("%v", val)
	}
}

// pyList approximates Python's repr() of a list of strings, e.g.
// `sorted(missing)` interpolated into an f-string as "['a', 'b']" --
// the shape every "missing keys"/"not in" error in this package uses.
// Sorts a copy; never mutates the caller's slice.
func pyList(items []string) string {
	sorted := append([]string(nil), items...)
	sort.Strings(sorted)
	quoted := make([]string, len(sorted))
	for i, s := range sorted {
		quoted[i] = pyRepr(s)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

func pySetRepr(set map[string]bool) string {
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	return pyList(keys)
}

func asObject(v interface{}) (map[string]interface{}, bool) {
	m, ok := v.(map[string]interface{})
	return m, ok
}

func asString(v interface{}) string {
	s, _ := v.(string)
	return s
}

// isBlank mirrors Python truthiness as used by `not report.get("scope")`
// / `not str(x or "").strip()` throughout validate_findings.py,
// canon_schema.py, and findings_schema.go: false (missing), None, "",
// whitespace-only strings, 0, false, and empty lists/objects are all
// blank in Python -- not just the string cases. A present value of a
// type that can't occur from well-formed JSON input (this project's
// files are all strings/numbers/bools/lists/objects, JSON has nothing
// else) falls through to "not blank", same as Python's default
// truthiness for any other non-empty value.
func isBlank(v interface{}) bool {
	switch val := v.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(val) == ""
	case float64:
		return val == 0
	case bool:
		return !val
	case []interface{}:
		return len(val) == 0
	case map[string]interface{}:
		return len(val) == 0
	default:
		return false
	}
}
