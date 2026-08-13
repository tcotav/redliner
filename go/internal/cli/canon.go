package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tcotav/redliner/go/internal/schemas"
)

const canonUsage = `Usage:
  redliner canon stale     <manuscript_dir>   # which sections need re-extraction
  redliner canon reconcile <manuscript_dir>   # build canon + find collisions`

func RunCanon(args []string, stdout io.Writer) int {
	if len(args) < 2 {
		fmt.Fprintln(stdout, canonUsage)
		return 1
	}
	command, manuscriptDir := args[0], args[1]
	info, err := os.Stat(manuscriptDir)
	if err != nil || !info.IsDir() {
		fmt.Fprintf(stdout, "No such directory: %s\n", manuscriptDir)
		return 1
	}

	switch command {
	case "stale":
		return cmdCanonStale(manuscriptDir, stdout)
	case "reconcile":
		return cmdCanonReconcile(manuscriptDir, stdout)
	default:
		fmt.Fprintf(stdout, "Unknown command %s\n", pyReprStr(command))
		return 1
	}
}

// ObservationsDir and CanonDir are exported for reuse by internal/mcpserver
// (Phase 4), which -- like mcp_server.py importing redliner_canon.py's
// module-level helpers directly -- calls into this package's canon
// logic rather than re-deriving it.
func ObservationsDir(manuscriptDir string) string {
	return filepath.Join(schemas.StateDir(manuscriptDir), "canon", "observations")
}

func CanonDir(manuscriptDir string) string {
	return filepath.Join(schemas.StateDir(manuscriptDir), "canon")
}

func stemOfPath(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// loadObservations reads every *.json file in <manuscript>/.redliner/
// canon/observations/, keyed by section stem. Mirrors
// redliner_canon.py's load_observations. Callers that need a
// deterministic iteration order (cmdCanonReconcile) sort the keys
// themselves rather than relying on map order.
func loadObservations(manuscriptDir string) (map[string]map[string]interface{}, error) {
	dir := ObservationsDir(manuscriptDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]map[string]interface{}{}, nil
		}
		return nil, err
	}

	out := map[string]map[string]interface{}{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		var parsed map[string]interface{}
		if err := json.Unmarshal(raw, &parsed); err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		out[stemOfPath(e.Name())] = parsed
	}
	return out, nil
}

// StaleResult mirrors cmd_stale's returned JSON shape.
type StaleResult struct {
	NeedsExtraction        []string          `json:"needs_extraction"`
	NeverExtracted         []string          `json:"never_extracted"`
	ChangedSinceExtraction []string          `json:"changed_since_extraction"`
	CurrentHashes          map[string]string `json:"current_hashes"`
	OrphanedObservations   []string          `json:"orphaned_observations"`
}

// ComputeStale is cmd_stale's computation, pure (no I/O writer, no
// printing) so it's reusable as-is by internal/mcpserver's canon_stale
// tool -- mirrors mcp_server.py's canon_stale, which reimplements this
// same loop against redliner_canon.py's module-level helpers rather
// than calling cmd_stale (which only prints). Exported for that reuse.
//
// Returns the raw error from loadObservations/SectionFiles unwrapped:
// callers (cmdCanonStale below, and mcpserver) each decide how to
// present a *schemas.SectionCollisionError specially and other errors
// generically, matching how the two Python front doors already present
// these differently from each other.
func ComputeStale(manuscriptDir string) (StaleResult, error) {
	observations, err := loadObservations(manuscriptDir)
	if err != nil {
		return StaleResult{}, err
	}

	sections, err := schemas.SectionFiles(manuscriptDir)
	if err != nil {
		return StaleResult{}, err
	}

	var missing, stale []string
	currentHashes := map[string]string{}
	sectionStems := map[string]bool{}

	for _, path := range sections {
		stem := stemOfPath(path)
		sectionStems[stem] = true
		fp, err := schemas.FingerprintSection(path)
		if err != nil {
			return StaleResult{}, err
		}
		recorded, ok := observations[stem]
		if !ok {
			missing = append(missing, stem)
			currentHashes[stem] = fp.SHA256
			continue
		}
		recordedHash, _ := recorded["section_sha256"].(string)
		if recordedHash != fp.SHA256 {
			stale = append(stale, stem)
			currentHashes[stem] = fp.SHA256
		}
	}

	needsExtraction := append(append([]string{}, missing...), stale...)
	sort.Strings(needsExtraction)

	var orphaned []string
	for stem := range observations {
		if !sectionStems[stem] {
			orphaned = append(orphaned, stem)
		}
	}
	sort.Strings(orphaned)

	return StaleResult{
		NeedsExtraction:        orEmptyStrings(needsExtraction),
		NeverExtracted:         orEmptyStrings(missing),
		ChangedSinceExtraction: orEmptyStrings(stale),
		CurrentHashes:          currentHashes,
		OrphanedObservations:   orEmptyStrings(orphaned),
	}, nil
}

func cmdCanonStale(manuscriptDir string, stdout io.Writer) int {
	result, err := ComputeStale(manuscriptDir)
	if err != nil {
		if _, ok := err.(*schemas.SectionCollisionError); ok {
			return reportSectionError(err, stdout)
		}
		fmt.Fprintf(stdout, "Error reading observations: %v\n", err)
		return 1
	}
	return printJSON(stdout, result)
}

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

type collisionFact struct {
	ID         string
	Entity     string
	EntityType string
	Attribute  string
	Value      interface{}
	Excerpt    string
	Location   string
	Source     string
	Confidence string
	Section    string
}

// CollisionFact is one fact's contribution to a Collision, as written to
// collisions.json.
type CollisionFact struct {
	ID         string      `json:"id"`
	Section    string      `json:"section"`
	Value      interface{} `json:"value"`
	Excerpt    string      `json:"excerpt"`
	Location   string      `json:"location"`
	Source     string      `json:"source"`
	Confidence string      `json:"confidence"`
}

// Collision mirrors cmd_reconcile's per-collision dict.
type Collision struct {
	Entity                         string          `json:"entity"`
	Attribute                      string          `json:"attribute"`
	DistinctValues                 []string        `json:"distinct_values"`
	Facts                          []CollisionFact `json:"facts"`
	AllNarration                   bool            `json:"all_narration"`
	AnyImplied                     bool            `json:"any_implied"`
	SectionsEditedSinceSnapshot    []string        `json:"sections_edited_since_snapshot"`
	SectionsUntouchedSinceSnapshot []string        `json:"sections_untouched_since_snapshot"`
	LikelyUnpropagatedRevision     bool            `json:"likely_unpropagated_revision"`
}

// CollisionsFile is collisions.json's top-level shape.
type CollisionsFile struct {
	Collisions []Collision `json:"collisions"`
}

// CanonAttributeValue is one fact contributing to an entity's attribute
// in canon.json.
type CanonAttributeValue struct {
	Value      interface{} `json:"value"`
	Section    string      `json:"section"`
	FactID     string      `json:"fact_id"`
	Source     string      `json:"source"`
	Confidence string      `json:"confidence"`
}

// CanonEntity is one entity's merged facts in canon.json.
type CanonEntity struct {
	EntityType string                           `json:"entity_type"`
	Attributes map[string][]CanonAttributeValue `json:"attributes"`
}

// Canon is canon.json's top-level shape.
type Canon struct {
	Entities        map[string]*CanonEntity `json:"entities"`
	FactCount       int                     `json:"fact_count"`
	SectionsCovered []string                `json:"sections_covered"`
}

// --- entity/attribute normalization for collision grouping -------------
// Mirrors python-baseline/redliner_canon.py's _norm_entity /
// _attr_tokens / linkByAttribute exactly -- the harness diffs this
// operation against that oracle, so the two must stay byte-identical.
//
// Added 2026-08-12: exact (entity, attribute) matching silently missed a
// real contradiction because independent per-section extractions named
// the same thing differently ("tide clock" vs "the tide clock";
// "duration_not_working" vs "stopped_duration"). See TODO.md.
var attrStopwords = map[string]bool{
	"of": true, "the": true, "a": true, "an": true, "is": true, "was": true,
	"are": true, "were": true, "be": true, "been": true, "to": true,
	"in": true, "at": true, "on": true, "for": true, "not": true, "no": true,
}

// normEntity lowercases, trims, and drops one leading article.
func normEntity(value string) string {
	text := strings.ToLower(strings.TrimSpace(value))
	for _, article := range []string{"the ", "a ", "an "} {
		if strings.HasPrefix(text, article) {
			return strings.TrimSpace(text[len(article):])
		}
	}
	return text
}

// attrTokens returns an attribute name's significant tokens.
func attrTokens(value string) map[string]bool {
	out := map[string]bool{}
	for _, part := range strings.FieldsFunc(strings.ToLower(strings.TrimSpace(value)), func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'))
	}) {
		if part != "" && !attrStopwords[part] {
			out[part] = true
		}
	}
	return out
}

// groupKey identifies a group by its fact-id set, order-independently --
// Python compares frozensets, so this stands in for that.
func groupKey(factIDs []string) string {
	sorted := append([]string(nil), factIDs...)
	sort.Strings(sorted)
	return strings.Join(sorted, "\x00")
}

func tokensIntersect(a, b map[string]bool) bool {
	for k := range a {
		if b[k] {
			return true
		}
	}
	return false
}

// linkByAttribute groups one entity's facts: exact-attribute groups, plus
// pairwise unions of two groups sharing a significant attribute token.
// Deliberately NOT transitive -- chaining A~B~C fuses attributes with
// nothing in common and yields a malformed collision.
func linkByAttribute(factIDs []string, factsByID map[string]*collisionFact) [][]string {
	pos := map[string]int{}
	for i, id := range factIDs {
		pos[id] = i
	}
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

	var groups [][]string
	for _, k := range keys {
		groups = append(groups, append([]string(nil), exact[k]...))
	}
	toks := make([]map[string]bool, len(keys))
	for i, k := range keys {
		toks[i] = attrTokens(k)
	}
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if tokensIntersect(toks[i], toks[j]) {
				merged := append(append([]string(nil), exact[keys[i]]...), exact[keys[j]]...)
				sort.Slice(merged, func(a, b int) bool { return pos[merged[a]] < pos[merged[b]] })
				groups = append(groups, merged)
			}
		}
	}

	// An exact-attribute group that is *itself* a collision (two or more
	// distinct values under one attribute name) is never superseded --
	// see the containment note below.
	protected := map[string]bool{}
	for _, k := range keys {
		values := map[string]bool{}
		for _, id := range exact[k] {
			values[strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", factsByID[id].Value)))] = true
		}
		if len(values) > 1 {
			protected[groupKey(exact[k])] = true
		}
	}

	// Drop groups contained in a larger one, so a merged pair supersedes
	// its two halves rather than being reported three times.
	//
	// Except when a half is a real collision on its own. "The merged pair
	// supersedes its halves" assumes the merge is at least as informative,
	// and it isn't: the superset drags in values from the *other*
	// attribute, so a clean `age_at_death: [eighty-one, seventy-seven]`
	// gets replaced by one also carrying `hospice` and `March`. That hides
	// signal behind noise -- a recall bug, not just a cosmetic one. Found
	// 2026-08-12 by simulating an entity-matching fix over the bellwether
	// fixture; see TODO.md.
	sort.SliceStable(groups, func(a, b int) bool { return len(groups[a]) > len(groups[b]) })
	var seen []map[string]bool
	var out [][]string
	for _, g := range groups {
		set := map[string]bool{}
		for _, id := range g {
			set[id] = true
		}
		contained := false
		if !protected[groupKey(g)] {
			for _, s := range seen {
				all := true
				for id := range set {
					if !s[id] {
						all = false
						break
					}
				}
				if all {
					contained = true
					break
				}
			}
		}
		if contained {
			continue
		}
		seen = append(seen, set)
		out = append(out, g)
	}
	sort.SliceStable(out, func(a, b int) bool { return pos[out[a][0]] < pos[out[b][0]] })
	return out
}

// ErrNoObservations signals cmd_reconcile's "no observations yet" case --
// a sentinel rather than a formatted error so each caller (the CLI, the
// MCP tool) can render its own message text around
// ObservationsDir(manuscriptDir), same as Python's two front doors each
// format this independently.
var ErrNoObservations = errors.New("no observations to reconcile")

// ComputeReconcile is cmd_reconcile's computation, pure (no I/O writer,
// no file writes, no printing) -- exported so internal/mcpserver's
// canon_reconcile tool can call it directly and get structured data back
// instead of Python's approach of capturing stdout and re-reading the
// written files. The observable side effect (canon.json/collisions.json
// written to disk) still happens for both front doors, via the shared
// WriteCanonFiles below -- only the in-process data path differs from
// Python's, not the on-disk contract.
func ComputeReconcile(manuscriptDir string) (Canon, []Collision, error) {
	observations, err := loadObservations(manuscriptDir)
	if err != nil {
		return Canon{}, nil, err
	}
	if len(observations) == 0 {
		return Canon{}, nil, ErrNoObservations
	}

	// `state = load_state(manuscript_dir) or {}` -- a missing state file
	// is not an error here, just "nothing changed since snapshot".
	state, _ := schemas.LoadState(manuscriptDir)
	changedSinceSnapshot := map[string]bool{}
	if state != nil && len(state.SectionFingerprints) > 0 {
		diff, err := schemas.DiffManuscript(manuscriptDir, state)
		if err != nil {
			return Canon{}, nil, err
		}
		for _, s := range diff.Changed {
			changedSinceSnapshot[s] = true
		}
		for _, s := range diff.Added {
			changedSinceSnapshot[s] = true
		}
	}

	var sectionStems []string
	for stem := range observations {
		sectionStems = append(sectionStems, stem)
	}
	sort.Strings(sectionStems)

	factsByID := map[string]*collisionFact{}
	var factOrder []string // insertion order = sorted sections, then each report's facts array order -- matches Python's dict/list iteration order, which determines canon.json's per-attribute array order (real content, not just key order)
	byEntity := map[string][]string{}
	seenEntity := map[string]bool{}
	var entityOrder []string

	for _, stem := range sectionStems {
		for _, raw := range reportField(observations[stem], "facts") {
			fact, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			id, _ := fact["id"].(string)
			if id == "" {
				continue
			}
			rec := &collisionFact{
				ID:         id,
				Entity:     asStr(fact["entity"]),
				EntityType: asStr(fact["entity_type"]),
				Attribute:  asStr(fact["attribute"]),
				Value:      fact["value"],
				Excerpt:    asStr(fact["excerpt"]),
				Location:   asStr(fact["location"]),
				Source:     asStr(fact["source"]),
				Confidence: asStr(fact["confidence"]),
				Section:    stem,
			}
			factsByID[id] = rec
			factOrder = append(factOrder, id)

			ent := normEntity(rec.Entity)
			if !seenEntity[ent] {
				seenEntity[ent] = true
				entityOrder = append(entityOrder, ent)
			}
			byEntity[ent] = append(byEntity[ent], id)
		}
	}

	// Group by normalized entity, then link facts whose attribute names
	// share a significant token. Mirrors the Python oracle's ordering:
	// entities sorted, then each group keyed by its lowest attribute name.
	type linked struct {
		entity   string
		sortAttr string
		factIDs  []string
	}
	var groups []linked
	entityKeys := append([]string(nil), entityOrder...)
	sort.Strings(entityKeys)
	for _, ent := range entityKeys {
		for _, ids := range linkByAttribute(byEntity[ent], factsByID) {
			sortAttr := ""
			for _, id := range ids {
				a := strings.ToLower(strings.TrimSpace(factsByID[id].Attribute))
				if sortAttr == "" || a < sortAttr {
					sortAttr = a
				}
			}
			groups = append(groups, linked{entity: ent, sortAttr: sortAttr, factIDs: ids})
		}
	}
	sort.SliceStable(groups, func(i, j int) bool {
		if groups[i].entity != groups[j].entity {
			return groups[i].entity < groups[j].entity
		}
		return groups[i].sortAttr < groups[j].sortAttr
	})

	var collisions []Collision
	for _, g := range groups {
		factIDs := g.factIDs

		valueGroups := map[string][]string{}
		for _, fid := range factIDs {
			v := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", factsByID[fid].Value)))
			valueGroups[v] = append(valueGroups[v], fid)
		}
		if len(valueGroups) < 2 {
			continue
		}

		involved := make([]*collisionFact, len(factIDs))
		for i, fid := range factIDs {
			involved[i] = factsByID[fid]
		}

		sectionSet := map[string]bool{}
		for _, f := range involved {
			sectionSet[f.Section] = true
		}
		var sections []string
		for s := range sectionSet {
			sections = append(sections, s)
		}
		sort.Strings(sections)

		var edited, untouched []string
		for _, s := range sections {
			if changedSinceSnapshot[s] {
				edited = append(edited, s)
			} else {
				untouched = append(untouched, s)
			}
		}

		allNarration, anyImplied := true, false
		for _, f := range involved {
			if f.Source != "narration" {
				allNarration = false
			}
			if f.Confidence == "implied" {
				anyImplied = true
			}
		}

		var distinctValues []string
		for v := range valueGroups {
			distinctValues = append(distinctValues, v)
		}
		sort.Strings(distinctValues)

		factsOut := make([]CollisionFact, len(involved))
		for i, f := range involved {
			factsOut[i] = CollisionFact{
				ID: f.ID, Section: f.Section, Value: f.Value, Excerpt: f.Excerpt,
				Location: f.Location, Source: f.Source, Confidence: f.Confidence,
			}
		}

		collisions = append(collisions, Collision{
			Entity:                         factsByID[factIDs[0]].Entity,
			Attribute:                      factsByID[factIDs[0]].Attribute,
			DistinctValues:                 distinctValues,
			Facts:                          factsOut,
			AllNarration:                   allNarration,
			AnyImplied:                     anyImplied,
			SectionsEditedSinceSnapshot:    orEmptyStrings(edited),
			SectionsUntouchedSinceSnapshot: orEmptyStrings(untouched),
			LikelyUnpropagatedRevision:     len(edited) > 0 && len(untouched) > 0,
		})
	}

	entities := map[string]*CanonEntity{}
	for _, id := range factOrder {
		f := factsByID[id]
		ent, ok := entities[f.Entity]
		if !ok {
			ent = &CanonEntity{EntityType: f.EntityType, Attributes: map[string][]CanonAttributeValue{}}
			entities[f.Entity] = ent
		}
		ent.Attributes[f.Attribute] = append(ent.Attributes[f.Attribute], CanonAttributeValue{
			Value: f.Value, Section: f.Section, FactID: f.ID, Source: f.Source, Confidence: f.Confidence,
		})
	}

	canon := Canon{
		Entities:        entities,
		FactCount:       len(factsByID),
		SectionsCovered: sectionStems,
	}

	return canon, collisions, nil
}

// WriteCanonFiles writes canon.json and collisions.json to the
// manuscript's .redliner/canon/ directory -- the one side effect both
// the CLI and the MCP tool must produce identically, factored out so
// there's exactly one place that does it.
func WriteCanonFiles(manuscriptDir string, canon Canon, collisions []Collision) error {
	dir := CanonDir(manuscriptDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating canon dir: %w", err)
	}
	if err := writeJSONFile(filepath.Join(dir, "canon.json"), canon); err != nil {
		return fmt.Errorf("writing canon.json: %w", err)
	}
	if err := writeJSONFile(filepath.Join(dir, "collisions.json"), CollisionsFile{Collisions: OrEmptyCollisions(collisions)}); err != nil {
		return fmt.Errorf("writing collisions.json: %w", err)
	}
	return nil
}

func cmdCanonReconcile(manuscriptDir string, stdout io.Writer) int {
	canon, collisions, err := ComputeReconcile(manuscriptDir)
	if err != nil {
		switch {
		case errors.Is(err, ErrNoObservations):
			fmt.Fprintf(stdout, "No observations in %s. Run extraction first.\n", ObservationsDir(manuscriptDir))
		case isSectionCollisionError(err):
			reportSectionError(err, stdout)
		default:
			fmt.Fprintf(stdout, "Error reading observations: %v\n", err)
		}
		return 1
	}

	if err := WriteCanonFiles(manuscriptDir, canon, collisions); err != nil {
		fmt.Fprintf(stdout, "Error writing canon: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "Canon: %d entities, %d facts.\n", len(canon.Entities), canon.FactCount)
	fmt.Fprintf(stdout, "Collisions to adjudicate: %d\n", len(collisions))
	for _, c := range collisions {
		flag := ""
		if c.LikelyUnpropagatedRevision {
			flag = " (likely unpropagated revision)"
		}
		fmt.Fprintf(stdout, "  - %s.%s: %s%s\n", c.Entity, c.Attribute, pyListRepr(c.DistinctValues), flag)
	}
	return 0
}

func isSectionCollisionError(err error) bool {
	_, ok := err.(*schemas.SectionCollisionError)
	return ok
}

func asStr(v interface{}) string {
	s, _ := v.(string)
	return s
}

// OrEmptyCollisions makes sure a nil slice marshals as JSON `[]`, not
// `null` -- exported alongside the rest of canon.go's reused pieces.
func OrEmptyCollisions(c []Collision) []Collision {
	if c == nil {
		return []Collision{}
	}
	return c
}

func writeJSONFile(path string, v interface{}) error {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o644)
}
