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

type groupKey struct{ entity, attribute string }

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
	grouped := map[groupKey][]string{}
	var groupOrder []groupKey
	seenGroup := map[groupKey]bool{}

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

			key := groupKey{
				entity:    strings.ToLower(strings.TrimSpace(rec.Entity)),
				attribute: strings.ToLower(strings.TrimSpace(rec.Attribute)),
			}
			if !seenGroup[key] {
				seenGroup[key] = true
				groupOrder = append(groupOrder, key)
			}
			grouped[key] = append(grouped[key], id)
		}
	}

	// Python: `for (entity, attribute), fact_ids in sorted(grouped.items())`
	sortedGroups := append([]groupKey(nil), groupOrder...)
	sort.Slice(sortedGroups, func(i, j int) bool {
		if sortedGroups[i].entity != sortedGroups[j].entity {
			return sortedGroups[i].entity < sortedGroups[j].entity
		}
		return sortedGroups[i].attribute < sortedGroups[j].attribute
	})

	var collisions []Collision
	for _, key := range sortedGroups {
		factIDs := grouped[key]

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
