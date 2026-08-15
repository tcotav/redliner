package cli

import "testing"

// Entity normalization is unchanged: still lowercase + drop one leading
// article, still the only thing standing between "tide clock" and "the
// tide clock". It is deliberately no more than that -- containment
// matching was measured on real prose and rejected (it fuses `X` with
// `X's <relative>`), and cross-entity joining is an agent's job now.
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

// These replace the token-merging tests removed on 2026-08-14 alongside
// the merging itself. What they pin now is the narrowed contract: group
// by exact attribute, nothing else, so a collision is only ever the same
// attribute on the same entity carrying different values.
//
// The old tests covered pairwise-not-transitive merging, containment
// supersession, and protect-exact. All three described machinery that
// only existed because merging existed. Keeping them would have pinned
// behaviour the measurements deleted.

func TestLinkByAttribute_GroupsByExactAttributeOnly(t *testing.T) {
	// The pair the merging was originally added for: two attribute names
	// sharing the token "duration". They must now stay apart -- that join
	// is the agent's, and the measured cost of doing it by token was 87%
	// artifacts on real prose.
	facts := map[string]*collisionFact{
		"f1": {Attribute: "duration_not_working"},
		"f2": {Attribute: "stopped_duration"},
	}
	groups := linkByAttribute([]string{"f1", "f2"}, facts)
	if len(groups) != 2 {
		t.Fatalf("want two separate groups, got %v", groups)
	}
	for _, g := range groups {
		if len(g) != 1 {
			t.Fatalf("attributes sharing a token must not be merged: %v", groups)
		}
	}
}

func TestLinkByAttribute_KeepsTheCaseItIsGoodAt(t *testing.T) {
	// Same entity, same attribute, two values. This is what the
	// deterministic pass keeps: the shape that caught two real
	// re-description issues on a live manuscript which the agent join
	// walked past.
	facts := map[string]*collisionFact{
		"f1": {Attribute: "age_at_death", Value: "eighty-one"},
		"f2": {Attribute: "age_at_death", Value: "two months short of seventy-seven"},
		"f3": {Attribute: "place_of_death", Value: "hospice"},
	}
	groups := linkByAttribute([]string{"f1", "f2", "f3"}, facts)
	if len(groups) != 2 {
		t.Fatalf("want age_at_death and place_of_death as two groups, got %v", groups)
	}

	var found bool
	for _, g := range groups {
		if len(g) == 2 && g[0] == "f1" && g[1] == "f2" {
			found = true
		}
	}
	if !found {
		t.Fatalf("the two age_at_death facts must group together: %v", groups)
	}
}

func TestLinkByAttribute_AttributeMatchIsCaseAndSpaceInsensitive(t *testing.T) {
	facts := map[string]*collisionFact{
		"f1": {Attribute: "Eye_Color", Value: "green"},
		"f2": {Attribute: "  eye_color ", Value: "blue"},
	}
	groups := linkByAttribute([]string{"f1", "f2"}, facts)
	if len(groups) != 1 || len(groups[0]) != 2 {
		t.Fatalf("want one group of two, got %v", groups)
	}
}

// First-appearance order within a group is load-bearing: collisions.json
// reports the first fact's entity and attribute verbatim, so a reordering
// silently changes the output's surface text.
func TestLinkByAttribute_PreservesFirstAppearanceOrderWithinAGroup(t *testing.T) {
	facts := map[string]*collisionFact{
		"f1": {Attribute: "action", Value: "stole the ledger"},
		"f2": {Attribute: "origin", Value: "Selkirk"},
		"f3": {Attribute: "action", Value: "hid in a fish stall"},
	}
	groups := linkByAttribute([]string{"f1", "f2", "f3"}, facts)
	for _, g := range groups {
		if len(g) == 2 && (g[0] != "f1" || g[1] != "f3") {
			t.Fatalf("action group lost input order: %v", g)
		}
	}
}
