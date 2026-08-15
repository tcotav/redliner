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
  redliner canon reconcile <manuscript_dir>   # build canon + find collisions
  redliner canon bundle    <manuscript_dir>   # every fact, one compact line each, for an agent to join
  redliner canon merge     <manuscript_dir>   # fold the joiner's findings into continuity.json`

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
	case "bundle":
		return cmdCanonBundle(manuscriptDir, stdout)
	case "merge":
		return cmdCanonMerge(manuscriptDir, stdout)
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

// --- bundle ---

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
func BundleFacts(manuscriptDir string) ([]string, error) {
	observations, err := loadObservations(manuscriptDir)
	if err != nil {
		return nil, err
	}
	if len(observations) == 0 {
		return nil, ErrNoObservations
	}

	var stems []string
	for stem := range observations {
		stems = append(stems, stem)
	}
	sort.Strings(stems)

	var lines []string
	for _, stem := range stems {
		for _, raw := range reportField(observations[stem], "facts") {
			fact, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			id, _ := fact["id"].(string)
			if id == "" {
				continue
			}
			lines = append(lines, fmt.Sprintf("%s | %s | %s | %v",
				bundleFactID(stem, id),
				asStr(fact["entity"]),
				asStr(fact["attribute"]),
				fact["value"],
			))
		}
	}
	return lines, nil
}

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

func cmdCanonBundle(manuscriptDir string, stdout io.Writer) int {
	lines, err := BundleFacts(manuscriptDir)
	if err != nil {
		if errors.Is(err, ErrNoObservations) {
			fmt.Fprintf(stdout, "No observations in %s. Run extraction first.\n", ObservationsDir(manuscriptDir))
			return 1
		}
		if _, ok := err.(*schemas.SectionCollisionError); ok {
			return reportSectionError(err, stdout)
		}
		fmt.Fprintf(stdout, "Error reading observations: %v\n", err)
		return 1
	}
	for _, line := range lines {
		fmt.Fprintln(stdout, line)
	}
	return 0
}

// --- merge ---

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
func MergeJoined(manuscriptDir string) (int, int, error) {
	dir := CanonDir(manuscriptDir)
	joinedPath := filepath.Join(dir, joinedFileName)
	raw, err := os.ReadFile(joinedPath)
	if err != nil {
		return 0, 0, err
	}
	var joined map[string]interface{}
	if err := json.Unmarshal(raw, &joined); err != nil {
		return 0, 0, fmt.Errorf("%s: %w", joinedFileName, err)
	}

	continuityPath := filepath.Join(dir, "continuity.json")
	existing := map[string]interface{}{"contradictions": []interface{}{}}
	if b, err := os.ReadFile(continuityPath); err == nil {
		if err := json.Unmarshal(b, &existing); err != nil {
			return 0, 0, fmt.Errorf("continuity.json: %w", err)
		}
	}

	out, _ := existing["contradictions"].([]interface{})
	seen := map[string]bool{}
	usedIDs := map[string]bool{}
	for _, item := range out {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		seen[factIDKey(m)] = true
		usedIDs[asStr(m["id"])] = true
	}

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

	existing["contradictions"] = out
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return added, skipped, err
	}
	return added, skipped, writeJSONFile(continuityPath, existing)
}

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

func cmdCanonMerge(manuscriptDir string, stdout io.Writer) int {
	added, skipped, err := MergeJoined(manuscriptDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(stdout, "No %s in %s -- run the continuity joiner first.\n", joinedFileName, CanonDir(manuscriptDir))
			return 1
		}
		fmt.Fprintf(stdout, "Merge failed: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Merged into continuity.json: %d added, %d already present.\n", added, skipped)
	return 0
}
