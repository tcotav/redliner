package cli

import "testing"

// These cover the collision-grouping normalization added 2026-08-12. The
// bug it fixes was found by a real end-to-end run, not by a fixture: two
// independent per-section extractions named one tide clock "tide clock"
// and "the tide clock", with attributes "duration_not_working" and
// "stopped_duration", so an exact (entity, attribute) match never
// collided them and a blatant eleven-vs-fifteen-years contradiction was
// reported as a clean manuscript. See TODO.md.
//
// The Python oracle in go/harness/python-baseline carries the identical
// logic; the golden harness diffs the two. Change one, change both.

func TestNormEntity_DropsLeadingArticle(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"tide clock", "tide clock"},
		{"the tide clock", "tide clock"},
		{"The Tide Clock", "tide clock"},
		{"  a loupe  ", "loupe"},
		{"an instrument", "instrument"},
		// Only a leading *article word* goes -- not any leading "a".
		{"anvil", "anvil"},
		{"theory", "theory"},
	} {
		if got := normEntity(tc.in); got != tc.want {
			t.Errorf("normEntity(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestAttrTokens_DropsStopwordsAndSeparators(t *testing.T) {
	got := attrTokens("duration_not_working")
	if !got["duration"] || !got["working"] {
		t.Errorf("expected duration+working, got %v", got)
	}
	if got["not"] {
		t.Errorf("stopword 'not' should be dropped, got %v", got)
	}
	// The pair that motivated all this must share a token.
	if !tokensIntersect(attrTokens("duration_not_working"), attrTokens("stopped_duration")) {
		t.Error("duration_not_working and stopped_duration must intersect")
	}
	// Unrelated attributes must not.
	if tokensIntersect(attrTokens("owns"), attrTokens("contains")) {
		t.Error("owns and contains must not intersect")
	}
}

func TestLinkByAttribute_IsPairwiseNotTransitive(t *testing.T) {
	// A shares "duration" with B; B shares "day" with C; A and C share
	// nothing. A transitive closure would fuse all three into one
	// malformed collision -- observed for real before this was fixed.
	facts := map[string]*collisionFact{
		"f1": {Attribute: "stopped_duration"},
		"f2": {Attribute: "duration_on_day"},
		"f3": {Attribute: "understood_on_day"},
	}
	for _, g := range linkByAttribute([]string{"f1", "f2", "f3"}, facts) {
		if len(g) == 3 {
			t.Fatalf("f1+f2+f3 were fused transitively: %v", g)
		}
	}
}

func TestLinkByAttribute_SupersedesContainedGroups(t *testing.T) {
	// Two facts whose attributes overlap should be reported once as a
	// merged pair, not additionally as their two singleton halves.
	//
	// Note the halves here are singletons -- neither is a collision on
	// its own, so neither is protected. That distinction is the subject
	// of the sibling test below.
	facts := map[string]*collisionFact{
		"f1": {Attribute: "duration_not_working"},
		"f2": {Attribute: "stopped_duration"},
	}
	groups := linkByAttribute([]string{"f1", "f2"}, facts)
	if len(groups) != 1 || len(groups[0]) != 2 {
		t.Fatalf("want one merged pair, got %v", groups)
	}
}

func TestLinkByAttribute_ProtectsExactGroupThatIsItselfACollision(t *testing.T) {
	// The bellwether shape: one attribute carries a genuine two-value
	// collision, and a neighbouring attribute shares a token with it.
	// Containment used to let the merged superset supersede the clean
	// pair, so `age_at_death: [eighty-one, seventy-seven]` was only ever
	// reported alongside `hospice` -- signal hidden behind noise, which
	// is a recall bug rather than a cosmetic one. The clean pair must
	// survive as a group of its own.
	facts := map[string]*collisionFact{
		"f1": {Attribute: "age_at_death", Value: "eighty-one"},
		"f2": {Attribute: "age_at_death", Value: "two months short of seventy-seven"},
		"f3": {Attribute: "place_of_death", Value: "hospice"},
	}
	groups := linkByAttribute([]string{"f1", "f2", "f3"}, facts)

	var foundClean bool
	for _, g := range groups {
		if len(g) == 2 && ((g[0] == "f1" && g[1] == "f2") || (g[0] == "f2" && g[1] == "f1")) {
			foundClean = true
		}
	}
	if !foundClean {
		t.Fatalf("clean age_at_death collision was superseded by the merged group: %v", groups)
	}

	// The merge itself is still reported -- this protects a group, it
	// doesn't suppress the union.
	var foundMerged bool
	for _, g := range groups {
		if len(g) == 3 {
			foundMerged = true
		}
	}
	if !foundMerged {
		t.Errorf("the merged group should still be reported too, got %v", groups)
	}
}

func TestLinkByAttribute_SingleValuedExactGroupStaysUnprotected(t *testing.T) {
	// Guard against over-reach: an exact-attribute group with only one
	// distinct value is not a collision, so it must still be superseded
	// by the merge. Otherwise every half comes back and the containment
	// rule stops doing anything at all.
	facts := map[string]*collisionFact{
		"f1": {Attribute: "age_at_death", Value: "eighty-one"},
		"f2": {Attribute: "age_at_death", Value: "eighty-one"},
		"f3": {Attribute: "place_of_death", Value: "hospice"},
	}
	for _, g := range linkByAttribute([]string{"f1", "f2", "f3"}, facts) {
		if len(g) == 2 {
			t.Fatalf("single-valued half should have been superseded, got %v", g)
		}
	}
}
