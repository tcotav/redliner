# redliner go/internal/cli — a walkthrough

*2026-08-15T20:59:13Z by Showboat 0.6.1*
<!-- showboat-id: 5bb94065-9607-40c0-91a7-b4358056c258 -->

-

-

```bash
sed -n "20,51p" go/internal/cli/dispatch.go
```

```output
// Dispatch is the binary's top-level subcommand router for every
// subcommand *except* "mcp", shared by cmd/redliner/main.go and this
// package's own tests. "mcp" is handled by main.go directly, not here:
// internal/mcpserver imports internal/cli (mirroring
// mcp_server.py importing straight into schemas/ and reusing
// redliner_canon.py's/validate_findings.py's functions), so this
// package cannot import internal/mcpserver back without a cycle.
func Dispatch(args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stdout, usage)
		return 1
	}
	switch args[0] {
	case "state":
		return RunState(args[1:], stdout)
	case "canon":
		return RunCanon(args[1:], stdout)
	case "domain":
		return RunDomain(args[1:], stdout, stderr)
	case "validate":
		return RunValidate(args[1:], stdout)
	case "context":
		return RunContext(args[1:], stdout)
	case "decisions":
		return RunDecisions(args[1:], stdout)
	case "rounds":
		return RunRounds(args[1:], stdout)
	default:
		fmt.Fprintf(stdout, "Unknown subcommand %s\n\n%s\n", pyReprStr(args[0]), usage)
		return 1
	}
}
```

-

```bash
sed -n "96,115p" go/internal/cli/util.go
```

```output
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
```

-

```bash
sed -n "81,92p" go/internal/cli/state.go
```

```output
func requireState(manuscriptDir string, stdout io.Writer) (*schemas.State, bool) {
	state, err := schemas.LoadState(manuscriptDir)
	if err != nil {
		fmt.Fprintf(stdout, "Error reading state: %v\n", err)
		return nil, false
	}
	if state == nil {
		fmt.Fprintf(stdout, "No redliner state in %s. Run: redliner state init %s\n", manuscriptDir, manuscriptDir)
		return nil, false
	}
	return state, true
}
```

-

```bash
sed -n "208,210p;264,281p" go/internal/cli/state.go
```

```output
// passKinds are the passes an author can actually run. Deliberately
// distinct from schemas.Phases -- see cmdStatePass for why.
var passKinds = []string{"developmental", "line", "continuity"}
func cmdStatePass(manuscriptDir, kind string, stdout io.Writer) int {
	// Deliberately NOT validated against schemas.Phases. The pass kinds
	// and the phases are different sets: `continuity` is a real pass that
	// is explicitly not phase-gated (it tracks its own per-section
	// staleness), while `intake` and `complete` are phases nobody runs a
	// pass of. Validating against Phases rejected `continuity` and
	// accepted `intake`, both wrong.
	valid := false
	for _, k := range passKinds {
		if k == kind {
			valid = true
			break
		}
	}
	if !valid {
		fmt.Fprintf(stdout, "Unknown pass %s. Must be one of: %s\n", pyReprStr(kind), strings.Join(passKinds, ", "))
		return 1
	}
```

-

```bash
sed -n "189,199p" go/internal/cli/state.go
```

```output
	roundTrackedPhase := domain.RoundTrackedPhase()

	previous := state.Phase
	state.Phase = phase
	// Entering the domain's round-tracked phase (fiction: "developmental")
	// from anywhere else starts a new round -- mirrors cmd_phase exactly,
	// including reading the phase name from domain config rather than a
	// hardcoded string.
	if phase == roundTrackedPhase && previous != roundTrackedPhase {
		state.DevelopmentalRound++
	}
```

-

```bash
sed -n "49,69p" go/internal/cli/domain.go
```

```output
func cmdDomainList(domainsDir string, stdout, stderr io.Writer) int {
	names := schemas.ListDomains(domainsDir)
	summaries := make([]DomainSummary, 0, len(names))
	for _, name := range names {
		d, err := schemas.LoadDomain(domainsDir, name)
		if err != nil {
			// Mirrors cmd_list exactly: a malformed domain config is
			// skipped with a note on *stderr*, not stdout -- stdout is
			// pure JSON here, and mixing this in would corrupt it for
			// any caller parsing `domain list`'s output.
			fmt.Fprintf(stderr, "Domain config error in %s: %v\n", pyReprStr(name), err)
			continue
		}
		summaries = append(summaries, DomainSummary{
			Name:        d.String("name"),
			DisplayName: d.String("display_name"),
			Description: d.String("description"),
		})
	}
	return printJSON(stdout, summaries)
}
```

-

```bash
sed -n "12,26p" go/internal/cli/context.go
```

```output
// `redliner context` exists to cut coordinator round trips, not to add
// capability: every field below was already obtainable, just across four
// or five separate commands. Measured on a real assess run (see TODO.md,
// "Cost/latency optimization"), the coordinator spent 18 API calls, half
// of them Bash, re-reading ~50K tokens of context each time -- including
// `domain show <name>` run twice back to back. One command that answers
// "where does this manuscript stand and what vocabulary applies" removes
// those round trips, and each removed round trip is a full model latency
// cycle as well as the tokens.
//
// Go-only by design: this is a convenience over operations the Python
// baseline already had, not a ported operation, so it is deliberately
// absent from go/harness/python-baseline and from the golden fixtures.
// The oracle's contract covers the ported surface; adding a composite
// here does not weaken it.
```

-

```bash
sed -n "66,90p" go/internal/cli/context.go
```

```output

	// Staleness needs observations; a manuscript with no continuity run
	// yet is a normal state, not an error -- report the zero value
	// rather than failing the whole call.
	stale, staleErr := ComputeStale(manuscriptDir)
	if staleErr != nil {
		stale = StaleResult{}
	}

	stateDir := schemas.StateDir(manuscriptDir)
	exists := func(rel string) bool {
		_, err := os.Stat(filepath.Join(stateDir, rel))
		return err == nil
	}
	// Line findings are written one file per section as
	// `findings/line_<section_stem>.json` (see skills/run/SKILL.md's line
	// flow), never as a single `findings/line.json`. Checking for the
	// latter reported `false` permanently -- including immediately after a
	// completed line pass -- so a coordinator orienting via this call was
	// told no line pass had ever run. Glob instead.
	anyExists := func(pattern string) bool {
		matches, err := filepath.Glob(filepath.Join(stateDir, pattern))
		return err == nil && len(matches) > 0
	}

```

-

```bash
sed -n "18,29p" go/internal/cli/validate.go
```

```output
// markdownEmphasis strips paired markdown emphasis/code delimiters
// before excerpt comparison, mirrors validate_findings.py's
// _MARKDOWN_EMPHASIS exactly (same deliberate choice to only match
// doubled/paired forms, not bare single */_).
var markdownEmphasis = regexp.MustCompile(`\*\*|__|` + "`" + `|~~`)

var lineFilePattern = regexp.MustCompile(`^line_(.+)\.json$`)

func normalizeExcerpt(text string) string {
	text = markdownEmphasis.ReplaceAllString(text, "")
	return strings.Join(strings.Fields(text), " ")
}
```

-

```bash
sed -n "41,57p" go/internal/cli/validate.go
```

```output
// verifyExcerpts checks that each item's excerpt (if present) is a
// genuine, normalized substring of the section it claims to quote.
//
// allowMulti governs whether `excerpt` may also be a *list* of strings,
// each validated as its own verbatim contiguous span. That is true for
// line findings and false for extracted facts, and the split is
// deliberate: a line finding about prose rhythm or POV is frequently
// about the relationship between two separated passages, which no single
// contiguous span can cite, while a fact asserts one thing and has one
// place it comes from. See TODO.md, "The excerpt field can't express a
// pattern across separated spans".
//
// Anything that is neither form is an error rather than a skip. It used
// to be a skip -- `excerpt, _ := item["excerpt"].(string)` on a list
// yielded "" and fell through the empty check -- so the workaround an
// agent would naturally reach for silently disabled the one guarantee
// this function exists to provide.
```

-

```bash
sed -n "84,117p" go/internal/cli/validate.go
```

```output
		switch excerpt := excerptRaw.(type) {
		case string:
			// An empty string is "no excerpt", same as omitting the key.
			if excerpt != "" {
				verify(excerpt, "")
			}
		case []interface{}:
			if !allowMulti {
				fail("excerpt must be a single string here -- a fact asserts one thing and cites one span")
				continue
			}
			if len(excerpt) == 0 {
				fail("excerpt is an empty list -- cite at least one span, or omit the field")
				continue
			}
			for j, spanRaw := range excerpt {
				span, ok := spanRaw.(string)
				if !ok {
					fail("excerpt[%d] is not a string", j)
					continue
				}
				if strings.TrimSpace(span) == "" {
					fail("excerpt[%d] is empty", j)
					continue
				}
				verify(span, fmt.Sprintf("[%d]", j))
			}
		default:
			if allowMulti {
				fail("excerpt must be a string or a list of strings")
			} else {
				fail("excerpt must be a string")
			}
		}
```

-

```bash
sed -n "15,25p" go/internal/cli/decisions.go
```

```output
// Author decisions -- resolve/wontfix -- are recorded in
// .redliner/decisions.json, a file no agent ever writes, and re-applied
// over the findings files after every pass.
//
// The problem this solves: findings files are rewritten wholesale by the
// developmental and line editors on each re-check. Agent prompts tell
// them to preserve author-set statuses, but an instruction is not a
// guarantee -- a single agent that renumbers or forgets silently
// discards a decision the author made, and nothing detects it. Keeping
// the decisions somewhere agents don't write, and re-applying them
// deterministically, makes the guarantee structural instead.
```

-

```bash
sed -n "162,181p" go/internal/cli/decisions.go
```

```output
	var missing []string
	for _, d := range decisions {
		if !seen[d.ID] {
			missing = append(missing, d.ID)
		}
	}
	sort.Strings(missing)

	fmt.Fprintf(stdout, "Decisions: %d recorded, %d already correct, %d restored.\n",
		len(decisions), len(alreadyOK), len(restored))
	if len(restored) > 0 {
		fmt.Fprintf(stdout, "Restored (a pass had overwritten these): %s\n", strings.Join(restored, ", "))
	}
	if len(missing) > 0 {
		// Not an error: a finding can legitimately vanish when a section is
		// cut or heavily rewritten. Worth saying, because the alternative is
		// a decision silently applying to nothing forever.
		fmt.Fprintf(stdout, "No longer present in any findings file: %s\n", strings.Join(missing, ", "))
	}
	return 0
```

-

```bash
sed -n "14,20p;73,86p" go/internal/cli/rounds.go
```

```output
// Completed passes are archived under .redliner/rounds/<pass>-round<N>/
// so a later round can be diffed against an earlier one.
//
// Without this the "before" simply doesn't exist: every pass rewrites
// findings in place, and `assess` explicitly clears stale findings from
// the previous round before writing new ones. Archiving at the end of a
// pass is what makes that clear step safe.
	// Which files belong to this pass. Continuity's artifacts live under
	// canon/, not findings/, so it archives a different set.
	var sources []string
	stateDir := schemas.StateDir(manuscriptDir)
	switch pass {
	case "developmental":
		sources, _ = filepath.Glob(filepath.Join(stateDir, "findings", "developmental.json"))
	case "line":
		sources, _ = filepath.Glob(filepath.Join(stateDir, "findings", "line_*.json"))
	case "continuity":
		sources, _ = filepath.Glob(filepath.Join(stateDir, "canon", "continuity.json"))
		more, _ := filepath.Glob(filepath.Join(stateDir, "canon", "collisions.json"))
		sources = append(sources, more...)
	}
```

```bash
sed -n "92,109p" go/internal/cli/rounds.go
```

```output
	dest := filepath.Join(roundsDir(manuscriptDir), fmt.Sprintf("%s-round%d", pass, state.DevelopmentalRound))
	if err := os.MkdirAll(dest, 0o755); err != nil {
		fmt.Fprintf(stdout, "Error creating %s: %v\n", dest, err)
		return 1
	}

	sort.Strings(sources)
	for _, src := range sources {
		data, err := os.ReadFile(src)
		if err != nil {
			fmt.Fprintf(stdout, "Error reading %s: %v\n", src, err)
			return 1
		}
		if err := os.WriteFile(filepath.Join(dest, filepath.Base(src)), data, 0o644); err != nil {
			fmt.Fprintf(stdout, "Error writing archive: %v\n", err)
			return 1
		}
	}
```

-

-

```bash
sed -n "283,314p" go/internal/cli/canon.go
```

```output
// linkByAttribute groups one entity's facts by exact attribute name, in
// first-appearance order. That is the whole rule.
//
// It used to do more: it also unioned any two attribute groups sharing a
// significant token, to catch `duration_not_working` against
// `stopped_duration`. That was measured and removed on 2026-08-14. The
// merging generated **87% of the collisions on a real 330-fact corpus as
// artifacts** -- pairs like `emotional_state` against `physical_state`,
// which share the token "state" and nothing else -- and drove collision
// count as facts^1.4, sending order 1,000+ non-collisions to an
// adjudicator on a full-length manuscript. This repo's own oldest
// fixture shows it in miniature: of the four collisions the sample
// manuscript produced, two were token-merge artifacts.
//
// It also did not buy the recall it was added for. The case it was meant
// to fix needs the two halves to be under the *same* entity, and the
// blind-manuscript run scored 0/4 because entity partitioning kept them
// apart regardless -- twice with the attribute matching exactly. Fuzzy
// attributes never had a chance to help.
//
// Cross-entity and cross-attribute joining is now an agent's job, which
// measured 4/4 against this same 0/4 (see TODO.md, "Is deterministic
// collision-finding the right architecture?"). What is left here is the
// case a string comparison is genuinely good at and an agent measurably
// missed: the same attribute on the same entity, recorded twice with
// different values.
//
// Removed with it: attrTokens, tokensIntersect, attrStopwords, and the
// protect-exact/containment logic, which existed only to stop a merged
// superset from swallowing a clean exact-attribute group. That fix
// (v0.3.0) is moot once nothing merges -- this is a deliberate reversal
// of it, not an oversight.
```

-

```bash
sed -n "315,332p" go/internal/cli/canon.go
```

```output
func linkByAttribute(factIDs []string, factsByID map[string]*collisionFact) [][]string {
	exact := map[string][]string{}
	var keys []string
	for _, id := range factIDs {
		k := strings.ToLower(strings.TrimSpace(factsByID[id].Attribute))
		if _, ok := exact[k]; !ok {
			keys = append(keys, k)
		}
		exact[k] = append(exact[k], id)
	}
	sort.Strings(keys)

	out := make([][]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, append([]string(nil), exact[k]...))
	}
	return out
}
```

-

```bash
sed -n "273,281p" go/internal/cli/canon.go
```

```output
func normEntity(value string) string {
	text := strings.ToLower(strings.TrimSpace(value))
	for _, article := range []string{"the ", "a ", "an "} {
		if strings.HasPrefix(text, article) {
			return strings.TrimSpace(text[len(article):])
		}
	}
	return text
}
```

-

```bash
sed -n "186,196p" go/internal/cli/canon.go
```

```output
// --- reconcile ---
//
// Deliberately the most direct possible transliteration of
// redliner_canon.py's cmd_reconcile, maps-of-maps and all -- this is the
// function TODO.md's Go-vs-Rust decision was made on (see the
// "reconcile-shape" argument in that discussion): several maps that
// reference the same underlying fact data, mutated and read together.
// Go's GC-backed maps let this port directly; forcing the exact same
// shape through Rust's borrow checker would have meant a redesign, not
// a port.

```

-

```bash
grep -n "sort\.\|insertion order\|iteration order" go/internal/cli/canon.go | sed -n "1,20p"
```

```output
69:// deterministic iteration order (cmdCanonReconcile) sort the keys
155:	sort.Strings(needsExtraction)
163:	sort.Strings(orphaned)
325:	sort.Strings(keys)
379:	sort.Strings(sectionStems)
382:	var factOrder []string // insertion order = sorted sections, then each report's facts array order -- matches Python's dict/list iteration order, which determines canon.json's per-attribute array order (real content, not just key order)
431:	sort.Strings(entityKeys)
444:	sort.SliceStable(groups, func(i, j int) bool {
477:		sort.Strings(sections)
502:		sort.Strings(distinctValues)
660:	sort.Strings(stems)
823:	sort.Strings(ids)
```

-

```bash
sed -n "455,462p" go/internal/cli/canon.go
```

```output
		valueGroups := map[string][]string{}
		for _, fid := range factIDs {
			v := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", factsByID[fid].Value)))
			valueGroups[v] = append(valueGroups[v], fid)
		}
		if len(valueGroups) < 2 {
			continue
		}
```

-

```bash
sed -n "479,486p;519,522p" go/internal/cli/canon.go
```

```output
		var edited, untouched []string
		for _, s := range sections {
			if changedSinceSnapshot[s] {
				edited = append(edited, s)
			} else {
				untouched = append(untouched, s)
			}
		}
			SectionsEditedSinceSnapshot:    orEmptyStrings(edited),
			SectionsUntouchedSinceSnapshot: orEmptyStrings(untouched),
			LikelyUnpropagatedRevision:     len(edited) > 0 && len(untouched) > 0,
		})
```

-

```bash
sed -n "625,646p" go/internal/cli/canon.go
```

```output
// BundleFacts renders every extracted fact as one line of
// `id | entity | attribute | value`, sections in order, with a compact id
// (`s{section}f{number}`) standing in for the full fact id.
//
// This is what gets handed to the continuity joiner. The format is not
// cosmetic -- it was measured. The same facts as JSON, carrying
// entity_type/source/confidence/excerpt, ran 267 bytes per fact; this
// runs 86, a 68% reduction, and cost fell 44% (more than the input share
// alone explains). Recall was unchanged at 4/4 across five runs, so the
// dropped fields were not carrying the join. See
// go/harness/fixtures/bellwether/SCALE_TEST.md.
//
// Size is the reason this exists: at 86 bytes/fact an order-2,000-fact
// novel is a ~172KB bundle and fits one call, which means the corpus does
// not have to be partitioned. That matters because every partitioning
// scheme measured so far destroys exactly the long-range cross-entity
// joins the join is for.
//
// `excerpt` is deliberately absent. Beyond size, a bundle carrying both a
// value and its original quotation lets a reader spot a mismatch between
// the two instead of doing the entity join -- which is a different, much
// easier task, and would make any measurement of the join dishonest.
```

-

```bash
sed -n "56,85p" go/internal/cli/canon_bundle_test.go
```

```output

// The excerpt is the field most likely to creep back in "for context".
// It must not: beyond costing most of the bundle's size, a bundle
// carrying both a value and its quotation lets a reader find a seeded
// contradiction by spotting the mismatch between them, which is a much
// easier task than the entity join and would invalidate every recall
// number measured against this format.
func TestBundleFacts_CarriesOnlyTheFourColumns(t *testing.T) {
	dir := t.TempDir()
	writeObservations(t, dir, "section_01", []map[string]interface{}{
		{"id": "fact-section_01-001", "entity": "Mira", "attribute": "origin", "value": "Selkirk",
			"excerpt": "a girl out of Selkirk", "location": "paragraph 1",
			"source": "narration", "confidence": "explicit", "entity_type": "character"},
	})

	lines, err := BundleFacts(dir)
	if err != nil {
		t.Fatal(err)
	}
	line := lines[0]
	if strings.Count(line, " | ") != 3 {
		t.Fatalf("want exactly four columns, got %q", line)
	}
	for _, leaked := range []string{"a girl out of Selkirk", "paragraph 1", "narration", "explicit", "character"} {
		if strings.Contains(line, leaked) {
			t.Errorf("bundle leaked %q: %s", leaked, line)
		}
	}
}

```

-

```bash
sed -n "684,703p" go/internal/cli/canon.go
```

```output
// bundleFactID compresses `fact-section_03-017` to `s3f017`. The section
// number is kept in the id rather than repeated as a column, so the agent
// can cite a section without the bundle paying for the word every line.
// Anything that doesn't match the expected shape is passed through whole
// rather than mangled -- an unfamiliar id is still a usable citation.
func bundleFactID(stem, id string) string {
	sec := strings.TrimPrefix(stem, "section_")
	sec = strings.TrimLeft(sec, "0")
	if sec == "" {
		sec = "0"
	}
	num := id
	if i := strings.LastIndex(id, "-"); i >= 0 {
		num = id[i+1:]
	}
	if sec == stem || num == id {
		return id
	}
	return "s" + sec + "f" + num
}
```

-

```bash
sed -n "726,746p" go/internal/cli/canon.go
```

```output
// JoinedFile is what the continuity joiner writes: the same contradiction
// shape continuity.json uses, in its own file so two agents never write
// one path. An agent that rewrites a file wholesale is how author
// decisions got clobbered before (see the decisions command); the fix
// there and here is the same -- each writer owns a file, and the merge is
// deterministic.
const joinedFileName = "joined.json"

// joinerIDBase offsets merged ids into cont-5NN so provenance is legible
// at a glance and the adjudicator's own cont-0NN numbering can never
// collide with the joiner's. The id pattern allows exactly three digits,
// so this leaves 499 slots on each side -- far past what either produces
// on a real manuscript (9 collisions on a 330-fact corpus).
const joinerIDBase = 500

// MergeJoined folds the joiner's findings into continuity.json, keeping
// the adjudicator's entries as they are. Matching on the fact-id set
// makes it idempotent: re-running after a re-join adds what is new
// instead of duplicating what is already there.
//
// Returns (added, skipped).
```

```bash
sed -n "779,801p" go/internal/cli/canon.go
```

```output
	added, skipped, next := 0, 0, joinerIDBase+1
	for _, item := range reportField(joined, "contradictions") {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if key := factIDKey(m); key == "" || seen[key] {
			skipped++
			continue
		}
		seen[factIDKey(m)] = true
		for next < 1000 && usedIDs[fmt.Sprintf("cont-%03d", next)] {
			next++
		}
		if next >= 1000 {
			return added, skipped, fmt.Errorf("ran out of cont-5NN ids merging %s", joinedFileName)
		}
		m["id"] = fmt.Sprintf("cont-%03d", next)
		usedIDs[asStr(m["id"])] = true
		next++
		out = append(out, m)
		added++
	}
```

-

```bash
grep -rn "cont-.d{3}" go/internal/schemas/canon_schema.go go/harness/python-baseline/schemas/canon_schema.py
```

```output
go/internal/schemas/canon_schema.go:31:var contradictionIDPattern = regexp.MustCompile(`^cont-\d{3}$`)
go/harness/python-baseline/schemas/canon_schema.py:69:CONTRADICTION_ID_PATTERN = re.compile(r"^cont-\d{3}$")
```

-

```bash
sed -n "810,825p" go/internal/cli/canon.go
```

```output
// factIDKey identifies a contradiction by the set of facts it cites,
// order-independently -- the one part of an entry that is a fact about
// the manuscript rather than a judgment about it, so it is what dedupes
// across re-runs even when the wording of the note changes.
func factIDKey(item map[string]interface{}) string {
	raw, ok := item["fact_ids"].([]interface{})
	if !ok || len(raw) == 0 {
		return ""
	}
	ids := make([]string, 0, len(raw))
	for _, v := range raw {
		ids = append(ids, asStr(v))
	}
	sort.Strings(ids)
	return strings.Join(ids, "\x00")
}
```

-

```bash
sed -n "88,129p" go/internal/cli/canon_merge_test.go
```

```output
// Re-running a join must not duplicate what is already merged -- the
// joiner renumbers from scratch each run, so identity has to come from
// the facts cited, not from the id.
func TestMergeJoined_IsIdempotent(t *testing.T) {
	dir := t.TempDir()
	writeCanonJSON(t, dir, "joined.json", map[string]interface{}{
		"contradictions": []interface{}{contradiction("cont-001", "f3", "f4")},
	})

	if _, _, err := MergeJoined(dir); err != nil {
		t.Fatal(err)
	}
	added, skipped, err := MergeJoined(dir)
	if err != nil {
		t.Fatal(err)
	}
	if added != 0 || skipped != 1 {
		t.Fatalf("second merge: added=%d skipped=%d, want 0/1", added, skipped)
	}
	if got := readContradictions(t, dir); len(got) != 1 {
		t.Fatalf("want 1 contradiction after two merges, got %d", len(got))
	}
}

// Fact order within an entry is not meaningful, so two entries citing the
// same facts in a different order are the same finding.
func TestMergeJoined_DedupesRegardlessOfFactOrder(t *testing.T) {
	dir := t.TempDir()
	writeCanonJSON(t, dir, "continuity.json", map[string]interface{}{
		"contradictions": []interface{}{contradiction("cont-001", "f2", "f1")},
	})
	writeCanonJSON(t, dir, "joined.json", map[string]interface{}{
		"contradictions": []interface{}{contradiction("cont-001", "f1", "f2")},
	})
	added, skipped, err := MergeJoined(dir)
	if err != nil {
		t.Fatal(err)
	}
	if added != 0 || skipped != 1 {
		t.Fatalf("got added=%d skipped=%d, want 0/1", added, skipped)
	}
}
```

-

```bash
cd go && go build -o /tmp/redliner-wt ./cmd/redliner && D=$(mktemp -d) && mkdir -p "$D/.redliner/canon" && python3 -c "
import json,sys
d=sys.argv[1]
items=[{\"id\":f\"cont-{i:03d}\",\"status\":\"open\",\"kind\":\"contradiction\",\"category\":\"character_attribute\",\"severity\":\"moderate\",\"fact_ids\":[f\"fact-a-{i:03d}\",f\"fact-b-{i:03d}\"],\"note\":\"n\"} for i in range(150)]
json.dump({\"contradictions\":items},open(d+\"/.redliner/canon/joined.json\",\"w\"))
" "$D" && /tmp/redliner-wt canon merge "$D" && python3 -c "
import json,sys
ids=[c[\"id\"] for c in json.load(open(sys.argv[1]+\"/.redliner/canon/continuity.json\"))[\"contradictions\"]]
print(\"merged:\", len(ids), \"contradictions\")
print(\"first:\", ids[0], \"| 100th:\", ids[99], \"| last:\", ids[-1])
print(\"ids past cont-599:\", sum(1 for i in ids if i > \"cont-599\"))
" "$D"
```

```output
Merged into continuity.json: 150 added, 0 already present.
merged: 150 contradictions
first: cont-501 | 100th: cont-600 | last: cont-650
ids past cont-599: 51
```

-

```bash
sed -n "80,88p;169,178p" go/internal/cli/golden_test.go
```

```output
// knownDivergentStdout names the one step whose Go stdout intentionally
// differs from Python's: requireState's "no state yet" message now says
// `redliner state init <dir>` (this port's actual invocation syntax),
// not `redliner_state.py init <dir>` (Python's) -- see state.go's
// requireState comment. Every other non-JSON step's stdout, regardless
// of exit code, is compared exactly; this is the only named exception.
var knownDivergentStdout = map[string]bool{
	"empty/01_state_status_no_state": true,
}
	// were captured from *this* checkout's absolute path. Running from a
	// worktree, a second clone, or CI at a different path makes every
	// path-embedding subtest fail with a diff that looks exactly like a
	// port bug but isn't one. Fail loudly and specifically instead of
	// letting that happen silently.
	probe := loadGolden(t, "crlf", "01_state_init")
	if stdout, _ := probe["stdout"].(string); !strings.Contains(stdout, workRoot) {
		t.Skipf("golden data was captured from a different checkout path than %s -- "+
			"re-run `python3 go/harness/capture_baseline.py` from this checkout first", workRoot)
	}
```

-

```bash
sed -n "35,44p" go/internal/cli/canon_norm_test.go
```

```output
// behaviour the measurements deleted.

func TestLinkByAttribute_GroupsByExactAttributeOnly(t *testing.T) {
	// The pair the merging was originally added for: two attribute names
	// sharing the token "duration". They must now stay apart -- that join
	// is the agent's, and the measured cost of doing it by token was 87%
	// artifacts on real prose.
	facts := map[string]*collisionFact{
		"f1": {Attribute: "duration_not_working"},
		"f2": {Attribute: "stopped_duration"},
```

-

```bash
cd go && go test -count=1 -v ./internal/cli/ 2>&1 | grep -E "^(--- |ok|FAIL|PASS)" | sed -E "s/ \([0-9.]+s\)//; s/\t[0-9.]+s$/\tOK/"
```

```output
--- PASS: TestBundleFacts_ShapeAndOrder
--- PASS: TestBundleFacts_CarriesOnlyTheFourColumns
--- PASS: TestBundleFactID_PassesThroughUnexpectedShapes
--- PASS: TestBundleFacts_NoObservations
--- PASS: TestMergeJoined_KeepsAdjudicatorEntriesAndOffsetsJoinerIDs
--- PASS: TestMergeJoined_IsIdempotent
--- PASS: TestMergeJoined_DedupesRegardlessOfFactOrder
--- PASS: TestMergeJoined_NoContinuityFileYet
--- PASS: TestCanonMerge_MissingJoinedFileIsAClearMessage
--- PASS: TestNormEntity_DropsLeadingArticle
--- PASS: TestLinkByAttribute_GroupsByExactAttributeOnly
--- PASS: TestLinkByAttribute_KeepsTheCaseItIsGoodAt
--- PASS: TestLinkByAttribute_AttributeMatchIsCaseAndSpaceInsensitive
--- PASS: TestLinkByAttribute_PreservesFirstAppearanceOrderWithinAGroup
--- SKIP: TestContext_AnswersEverythingTheOrientationCallsDid
--- PASS: TestContext_UsageWithoutArgs
--- PASS: TestBuildContext_DetectsPerSectionLineFindings
--- PASS: TestDecisionsApply_RestoresWhatAPassOverwrote
--- PASS: TestCLI_MatchesPythonGolden
--- PASS: TestRoundsArchive_KeepsEachPassSeparately
--- PASS: TestStateStage_ValidatesAgainstDomainVocabulary
--- PASS: TestStatePass_CountsPassesNotPhases
--- PASS: TestVerifyExcerpts_ListForm
--- PASS: TestVerifyExcerpts_AbsentIsFine
--- PASS: TestVerifyExcerpts_ListNormalized
PASS
ok  	github.com/tcotav/redliner/go/internal/cli	OK
```

-

```bash
cd go && go test -count=1 -run TestContext_Answers -v ./internal/cli/ 2>&1 | head -2 | sed -E "s#/private/var/folders/[^ ]*b001#<go-test-tmpdir>#; s#\(tried:.*#(tried: ...)#"
```

```output
=== RUN   TestContext_AnswersEverythingTheOrientationCallsDid
    context_test.go:20: state init unavailable in this environment: Domain config error: no domains/ directory found near <go-test-tmpdir> (tried: ...)
```

```bash
sed -n "16,21p" go/internal/cli/context_test.go; echo "--- the pattern that works, from golden_test.go: ---"; sed -n "153,154p" go/internal/cli/golden_test.go
```

```output
func TestContext_AnswersEverythingTheOrientationCallsDid(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer
	if code := RunState([]string{"init", dir, "fiction"}, &out); code != 0 {
		t.Skipf("state init unavailable in this environment: %s", out.String())
	}
--- the pattern that works, from golden_test.go: ---
func TestCLI_MatchesPythonGolden(t *testing.T) {
	t.Setenv("REDLINER_DOMAINS_DIR", filepath.Join(repoRoot(t), "domains"))
```

-

```bash
D=$(mktemp -d) && mkdir -p "$D/.redliner/canon" && printf "{\"manuscript_dir\":\"%s\",\"domain\":\"fiction\",\"phase\":\"developmental\",\"developmental_round\":3,\"section_fingerprints\":{},\"created_at\":\"x\"}" "$D" > "$D/.redliner/state.json" && echo "{\"contradictions\":[{\"id\":\"cont-001\",\"note\":\"FIRST PASS\"}]}" > "$D/.redliner/canon/continuity.json" && /tmp/redliner-wt rounds archive "$D" continuity | sed "s#$D#<tmp>#" && echo "{\"contradictions\":[{\"id\":\"cont-001\",\"note\":\"SECOND PASS\"}]}" > "$D/.redliner/canon/continuity.json" && /tmp/redliner-wt rounds archive "$D" continuity | sed "s#$D#<tmp>#" && echo "--- archive dirs after two continuity passes: ---" && ls "$D/.redliner/rounds/" && echo "--- which version survived: ---" && grep -o "[A-Z]* PASS" "$D/.redliner/rounds/continuity-round3/continuity.json"
```

```output
Archived 1 file(s) to <tmp>/.redliner/rounds/continuity-round3
Archived 1 file(s) to <tmp>/.redliner/rounds/continuity-round3
--- archive dirs after two continuity passes: ---
continuity-round3
--- which version survived: ---
SECOND PASS
```

-
