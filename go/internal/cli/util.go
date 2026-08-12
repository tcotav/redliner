// Package cli is the Go port of bin/redliner_{state,canon,domain}.py and
// bin/validate_findings.py, consolidated into subcommands of one binary
// per TODO.md's "v1 plan" decision (redliner state status <dir> instead
// of redliner_state.py status <dir>) rather than four argv[0]-dispatched
// script names. Built on top of internal/schemas (Phase 2). See
// go/harness/README.md for how this is verified against the real Python
// CLI's captured output.
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// printJSON writes v as indented JSON followed by a newline, matching
// every Python command's `print(json.dumps(x, indent=2))`. Field/key
// order is whatever Go produces (struct field order, or alphabetical for
// maps) -- not a byte-match of Python's dict insertion order, which
// TODO.md's JSON-key-order hazard already establishes isn't a real
// requirement (nothing but redliner itself reads this JSON).
func printJSON(stdout io.Writer, v interface{}) int {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Fprintf(stdout, "Error encoding JSON: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, string(raw))
	return 0
}

// checkFile prints the "OK   <path>" / "FAIL <path>" + indented error
// lines validate_findings.py prints per file, and reports whether the
// file passed. This exact two-line-per-error shape is a real
// compatibility surface (skill prose and this project's own
// live-verification habit pattern-match on it) -- see
// go/harness/README.md.
func checkFile(stdout io.Writer, path string, errors []string) bool {
	if len(errors) > 0 {
		fmt.Fprintf(stdout, "FAIL %s\n", path)
		for _, e := range errors {
			fmt.Fprintf(stdout, "  - %s\n", e)
		}
		return false
	}
	fmt.Fprintf(stdout, "OK   %s\n", path)
	return true
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// loadJSON reads and parses a JSON file generically. Unlike Python's
// bare json.loads (which crashes the whole validate run on malformed
// JSON), a parse failure here degrades to a synthetic object carrying
// the error, which the validators then reject via their normal
// "not a JSON object" / missing-field checks -- strictly more robust
// than the Python original, not a compatibility gap in the tested paths
// (all real fixtures have well-formed JSON).
func loadJSON(path string) interface{} {
	raw, err := os.ReadFile(path)
	if err != nil {
		return map[string]interface{}{"__read_error__": err.Error()}
	}
	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		return map[string]interface{}{"__parse_error__": err.Error()}
	}
	return v
}

func reportField(report interface{}, key string) []interface{} {
	m, ok := report.(map[string]interface{})
	if !ok {
		return nil
	}
	list, _ := m[key].([]interface{})
	return list
}

// orEmptyStrings makes sure a nil slice marshals as JSON `[]`, not
// `null` -- see project_state.go's identical orEmpty for why this
// matters for the harness's JSON comparison.
func orEmptyStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// pyReprStr / pyListRepr approximate Python's repr() for the small set
// of shapes these commands' error/summary messages actually use --
// mirrors schemas' unexported pyRepr/pyList (not reused directly since
// those are internal to the schemas package and this is a narrow,
// separate need).
func pyReprStr(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "\\'") + "'"
}

func pyListRepr(items []string) string {
	sorted := append([]string(nil), items...)
	// Callers pass already-sorted distinct_values; sort again defensively
	// so this helper is correct standalone too.
	sort.Strings(sorted)
	quoted := make([]string, len(sorted))
	for i, s := range sorted {
		quoted[i] = pyReprStr(s)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}
