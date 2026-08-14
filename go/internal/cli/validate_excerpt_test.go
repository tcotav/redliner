package cli

import (
	"strings"
	"testing"
)

// The section these tests quote against. Two sentences deliberately far
// apart, because the whole point of the list form is citing a pattern
// that spans them.
const excerptSectionText = `She was scared, and the room was cold.

A paragraph of other business entirely, which nobody needs to read.

She was scared again, and the room was colder.
`

func items(excerpts ...interface{}) []interface{} {
	out := make([]interface{}, 0, len(excerpts))
	for i, e := range excerpts {
		item := map[string]interface{}{"id": string(rune('a' + i))}
		if e != nil {
			item["excerpt"] = e
		}
		out = append(out, item)
	}
	return out
}

func TestVerifyExcerpts_ListForm(t *testing.T) {
	realA := "She was scared, and the room was cold."
	realB := "She was scared again, and the room was colder."

	for _, tc := range []struct {
		name       string
		excerpt    interface{}
		allowMulti bool
		wantErr    string // substring; "" means expect no errors
	}{
		// The bug this whole change exists for: a line finding about the
		// relationship between two separated spans now has a way to cite
		// both, and both get checked.
		{"two real spans accepted", []interface{}{realA, realB}, true, ""},
		{"one fabricated span in a list is caught", []interface{}{realA, "She was never scared at all."}, true, "excerpt[1] not found verbatim"},
		{"all fabricated caught", []interface{}{"nope", "also nope"}, true, "excerpt[0] not found verbatim"},

		// A one-element list means exactly what the string form means.
		{"single-element list accepted", []interface{}{realA}, true, ""},
		{"single-element list still verified", []interface{}{"invented"}, true, "excerpt[0] not found verbatim"},

		// A claim to cite, citing nothing.
		{"empty list rejected", []interface{}{}, true, "empty list"},
		{"blank element rejected", []interface{}{realA, "   "}, true, "excerpt[1] is empty"},
		{"non-string element rejected", []interface{}{realA, 42.0}, true, "excerpt[1] is not a string"},

		// The string form is unchanged, in both modes.
		{"string accepted", realA, true, ""},
		{"string verified", "invented", true, "not found verbatim"},
		{"string accepted for facts", realA, false, ""},
		{"empty string is no excerpt", "", false, ""},

		// The regression that made this more than an expressiveness gap:
		// a list used to type-assert to "" and skip validation entirely.
		{"list rejected where facts are validated", []interface{}{realA}, false, "must be a single string"},
		{"fabricated list rejected for facts", []interface{}{"invented"}, false, "must be a single string"},
		{"wrong type rejected for facts", 42.0, false, "must be a string"},
		{"wrong type rejected for findings", 42.0, true, "must be a string or a list of strings"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			errs := verifyExcerpts(items(tc.excerpt), excerptSectionText, "line_section_01.json", tc.allowMulti)
			joined := strings.Join(errs, "\n")
			if tc.wantErr == "" {
				if len(errs) != 0 {
					t.Fatalf("want no errors, got: %s", joined)
				}
				return
			}
			if !strings.Contains(joined, tc.wantErr) {
				t.Fatalf("want an error containing %q, got: %s", tc.wantErr, joined)
			}
		})
	}
}

// A missing excerpt stays optional -- developmental findings and some
// facts legitimately carry none, and requiring one here would reject
// them.
func TestVerifyExcerpts_AbsentIsFine(t *testing.T) {
	for _, allowMulti := range []bool{true, false} {
		if errs := verifyExcerpts(items(nil), excerptSectionText, "f.json", allowMulti); len(errs) != 0 {
			t.Errorf("allowMulti=%v: want no errors for an absent excerpt, got: %v", allowMulti, errs)
		}
	}
}

// Normalization (whitespace collapsing, markdown emphasis stripping)
// applies to every element of a list, not just the string form.
func TestVerifyExcerpts_ListNormalized(t *testing.T) {
	wrapped := []interface{}{
		"She was scared,\n   and the room was cold.",
		"She **was scared again**, and the room was colder.",
	}
	if errs := verifyExcerpts(items(wrapped), excerptSectionText, "line_section_01.json", true); len(errs) != 0 {
		t.Fatalf("want normalization applied per element, got: %v", errs)
	}
}
