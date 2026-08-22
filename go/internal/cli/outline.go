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

const outlineUsage = `Usage:
  redliner outline stale    <manuscript_dir>   # which sections need re-recording
  redliner outline join     <manuscript_dir>   # rebuild outline.json from every section file
  redliner outline render   <manuscript_dir>   # write the author-readable Outline.md
  redliner outline versions <manuscript_dir>   # list archived outline versions`

// RunOutline resolves its own domainsDir via schemas.FindDomainsDir() --
// correct for the CLI, where os.Executable() really is this binary.
// Reused by internal/mcpserver's outline_* tools, which do NOT go
// through this entry point for that reason (a second FindDomainsDir()
// call there would search from whatever binary the calling
// test/process happens to be, not from the domainsDir the MCP server
// already resolved once) -- see RunOutlineWithDomainsDir below, which
// takes domainsDir as a parameter instead of rediscovering it. Same
// split as RunValidate/ValidateManuscript in validate.go.
func RunOutline(args []string, stdout, stderr io.Writer) int {
	domainsDir, err := schemas.FindDomainsDir()
	if err != nil {
		fmt.Fprintf(stdout, "Domain config error: %v\n", err)
		return 1
	}
	return RunOutlineWithDomainsDir(args, domainsDir, stdout, stderr)
}

// RunOutlineWithDomainsDir is RunOutline's dispatch with domainsDir
// taken as a parameter instead of resolved internally -- the piece
// internal/mcpserver's outline_* tools actually call, passing the same
// domainsDir every other tool in that server uses.
func RunOutlineWithDomainsDir(args []string, domainsDir string, stdout, stderr io.Writer) int {
	if len(args) < 2 {
		fmt.Fprintln(stdout, outlineUsage)
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
		return cmdOutlineStale(manuscriptDir, stdout)
	case "join":
		return cmdOutlineJoin(manuscriptDir, stdout)
	case "render":
		return cmdOutlineRender(manuscriptDir, domainsDir, stdout)
	case "versions":
		return cmdOutlineVersions(manuscriptDir, stdout)
	default:
		fmt.Fprintf(stdout, "Unknown command %s\n%s\n", pyReprStr(command), outlineUsage)
		return 1
	}
}

// OutlineDir and OutlineSectionsDir are exported for reuse by
// internal/mcpserver, same as ObservationsDir/CanonDir in canon.go.
func OutlineDir(manuscriptDir string) string {
	return filepath.Join(schemas.StateDir(manuscriptDir), "outline")
}

func OutlineSectionsDir(manuscriptDir string) string {
	return filepath.Join(OutlineDir(manuscriptDir), "sections")
}

// loadOutlineSections reads every *.json under outline/sections/, keyed
// by section stem. Mirrors canon.go's loadObservations.
func loadOutlineSections(manuscriptDir string) (map[string]map[string]interface{}, error) {
	dir := OutlineSectionsDir(manuscriptDir)
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

// OutlineStaleResult is the JSON shape the run skill reads to decide
// which sections to Task the outliner agent on. Field names deliberately
// differ from canon's ("recording", not "extraction") so a skill author
// reading either output knows which layer produced it.
type OutlineStaleResult struct {
	NeedsRecording        []string          `json:"needs_recording"`
	NeverRecorded         []string          `json:"never_recorded"`
	ChangedSinceRecording []string          `json:"changed_since_recording"`
	CurrentHashes         map[string]string `json:"current_hashes"`
	OrphanedSections      []string          `json:"orphaned_sections"`
}

// ComputeOutlineStale is the whole reason this layer can be re-run after
// every chapter: it costs one agent call per section whose text actually
// moved, and nothing for the rest.
//
// Deliberately a parallel implementation of canon.go's ComputeStale
// rather than a shared generic over both directories. The two read
// different trees and are already diverging (this layer archives a
// version per run; continuity has no equivalent), and one abstraction
// serving two things that merely look alike is how the next change to
// either becomes a change to both.
func ComputeOutlineStale(manuscriptDir string) (OutlineStaleResult, error) {
	recorded, err := loadOutlineSections(manuscriptDir)
	if err != nil {
		return OutlineStaleResult{}, err
	}

	sections, err := schemas.SectionFiles(manuscriptDir)
	if err != nil {
		return OutlineStaleResult{}, err
	}

	var missing, stale []string
	currentHashes := map[string]string{}
	sectionStems := map[string]bool{}

	for _, path := range sections {
		stem := stemOfPath(path)
		sectionStems[stem] = true
		fp, err := schemas.FingerprintSection(path)
		if err != nil {
			return OutlineStaleResult{}, err
		}
		existing, ok := recorded[stem]
		if !ok {
			missing = append(missing, stem)
			currentHashes[stem] = fp.SHA256
			continue
		}
		recordedHash, _ := existing["section_sha256"].(string)
		if recordedHash != fp.SHA256 {
			stale = append(stale, stem)
			currentHashes[stem] = fp.SHA256
		}
	}

	needs := append(append([]string{}, missing...), stale...)
	sort.Strings(needs)

	var orphaned []string
	for stem := range recorded {
		if !sectionStems[stem] {
			orphaned = append(orphaned, stem)
		}
	}
	sort.Strings(orphaned)

	return OutlineStaleResult{
		NeedsRecording:        orEmptyStrings(needs),
		NeverRecorded:         orEmptyStrings(missing),
		ChangedSinceRecording: orEmptyStrings(stale),
		CurrentHashes:         currentHashes,
		OrphanedSections:      orEmptyStrings(orphaned),
	}, nil
}

func cmdOutlineStale(manuscriptDir string, stdout io.Writer) int {
	result, err := ComputeOutlineStale(manuscriptDir)
	if err != nil {
		if _, ok := err.(*schemas.SectionCollisionError); ok {
			return reportSectionError(err, stdout)
		}
		fmt.Fprintf(stdout, "Error reading outline sections: %v\n", err)
		return 1
	}
	return printJSON(stdout, result)
}

func OutlinePath(manuscriptDir string) string {
	return filepath.Join(OutlineDir(manuscriptDir), "outline.json")
}

var ErrNoOutlineSections = errors.New("no outline sections recorded")

// ComputeOutlineJoin rebuilds the whole outline from every current
// per-section file, not only the ones re-recorded this run -- same
// contract as canon reconcile. Deterministic: no model call, no prose
// read. That is what makes the render below free enough to run after
// every chapter.
func ComputeOutlineJoin(manuscriptDir string) (map[string]interface{}, error) {
	recorded, err := loadOutlineSections(manuscriptDir)
	if err != nil {
		return nil, err
	}
	if len(recorded) == 0 {
		return nil, ErrNoOutlineSections
	}

	stems := make([]string, 0, len(recorded))
	for stem := range recorded {
		stems = append(stems, stem)
	}
	// Sorted by stem, which is manuscript order -- never map iteration
	// order, which would make the join non-deterministic and every
	// version archive spuriously different from the last.
	sort.Strings(stems)

	sections := make([]interface{}, 0, len(stems))
	sceneCount := 0
	for _, stem := range stems {
		section := recorded[stem]
		if scenes, ok := section["scenes"].([]interface{}); ok {
			sceneCount += len(scenes)
		}
		sections = append(sections, section)
	}

	joined := map[string]interface{}{
		"sections":    sections,
		"scene_count": sceneCount,
	}

	// published_through travels with the joined document so the renderer
	// (and anything reading outline.json later) doesn't need state too.
	if state, err := schemas.LoadState(manuscriptDir); err == nil && state != nil && state.PublishedThrough != "" {
		joined["published_through"] = state.PublishedThrough
	}

	return joined, nil
}

func cmdOutlineJoin(manuscriptDir string, stdout io.Writer) int {
	joined, err := ComputeOutlineJoin(manuscriptDir)
	if err != nil {
		if err == ErrNoOutlineSections {
			fmt.Fprintf(stdout, "No outline sections in %s. Run `redliner outline stale` and record them first.\n", OutlineSectionsDir(manuscriptDir))
			return 1
		}
		fmt.Fprintf(stdout, "Error reading outline sections: %v\n", err)
		return 1
	}

	if err := os.MkdirAll(OutlineDir(manuscriptDir), 0o755); err != nil {
		fmt.Fprintf(stdout, "Error creating %s: %v\n", OutlineDir(manuscriptDir), err)
		return 1
	}
	raw, err := json.MarshalIndent(joined, "", "  ")
	if err != nil {
		fmt.Fprintf(stdout, "Error encoding outline: %v\n", err)
		return 1
	}
	if err := os.WriteFile(OutlinePath(manuscriptDir), append(raw, '\n'), 0o644); err != nil {
		fmt.Fprintf(stdout, "Error writing %s: %v\n", OutlinePath(manuscriptDir), err)
		return 1
	}

	fmt.Fprintf(stdout, "Joined %d section(s), %v scene(s) → %s\n",
		len(joined["sections"].([]interface{})), joined["scene_count"], OutlinePath(manuscriptDir))
	return 0
}

// RenderedOutlinePath is deliberately in the manuscript directory, not
// under .redliner/. Same rule the editorial letters follow: hidden
// storage for machine state, visible files for anything a human reads.
// `.redliner` is a dotfile directory Finder hides by default, and an
// author who cannot find the outline got nothing for the run.
//
// Safe to sit beside the chapters: schemas.SectionFiles globs
// `section_*` with a .txt/.md extension, so nothing named this way is
// discovered as manuscript text.
func RenderedOutlinePath(manuscriptDir string) string {
	return filepath.Join(manuscriptDir, "Outline.md")
}

// titleCaseField turns a config field name into a display label
// ("leaves_open" -> "Leaves open"). Deliberately minimal: the field
// names are authored in domain.json by whoever designs the domain, so
// they are already readable words.
func titleCaseField(name string) string {
	words := strings.Split(name, "_")
	if len(words) == 0 || words[0] == "" {
		return name
	}
	words[0] = strings.ToUpper(words[0][:1]) + words[0][1:]
	return strings.Join(words, " ")
}

// RenderOutline builds the author-facing Markdown. Pure: takes the
// joined document and the domain's field lists, returns a string. No
// model call and no file I/O -- keeping this deterministic is what makes
// the per-run cost proportional to what the author changed rather than
// fixed, which is the whole argument for re-running after every chapter.
func RenderOutline(joined map[string]interface{}, rowFields, sectionFields []string) string {
	var b strings.Builder
	b.WriteString("# Outline\n\n")

	sections, _ := joined["sections"].([]interface{})
	fmt.Fprintf(&b, "%v scene(s) across %d section(s).\n", joined["scene_count"], len(sections))

	publishedThrough, _ := joined["published_through"].(string)

	for _, sectionRaw := range sections {
		section, ok := sectionRaw.(map[string]interface{})
		if !ok {
			continue
		}
		stem, _ := section["section"].(string)
		fmt.Fprintf(&b, "\n## %s\n", stem)

		for _, field := range sectionFields {
			if value, ok := section[field].(string); ok && value != "" {
				fmt.Fprintf(&b, "\n%s: %s\n", titleCaseField(field), value)
			}
		}

		scenes, _ := section["scenes"].([]interface{})
		if len(scenes) == 0 {
			b.WriteString("\n*No scenes recorded.*\n")
		}
		for _, sceneRaw := range scenes {
			scene, ok := sceneRaw.(map[string]interface{})
			if !ok {
				continue
			}
			order, _ := scene["order"].(float64)
			pov, _ := scene["pov"].(string)
			anchor, _ := scene["anchor"].(string)
			fmt.Fprintf(&b, "\n%d. **%s** — \"%s\"\n", int(order), pov, anchor)
			for _, field := range rowFields {
				value, _ := scene[field].(string)
				fmt.Fprintf(&b, "   - %s: %s\n", titleCaseField(field), value)
			}
		}

		// The line goes *after* the last published section. A scene above
		// it cannot be moved or cut at all, which is the one fact this
		// whole view exists to serve.
		if publishedThrough != "" && stem == publishedThrough {
			b.WriteString("\n---\n\n")
			b.WriteString("*Everything above this line is published. Those scenes can't be moved or cut.*\n\n")
			b.WriteString("---\n")
		}
	}

	return b.String()
}

func cmdOutlineRender(manuscriptDir, domainsDir string, stdout io.Writer) int {
	joined, err := ComputeOutlineJoin(manuscriptDir)
	if err != nil {
		if err == ErrNoOutlineSections {
			fmt.Fprintf(stdout, "No outline sections in %s. Run `redliner outline stale` and record them first.\n", OutlineSectionsDir(manuscriptDir))
			return 1
		}
		fmt.Fprintf(stdout, "Error reading outline sections: %v\n", err)
		return 1
	}

	rowFields, sectionFields, err := outlineFieldsFor(manuscriptDir, domainsDir)
	if err != nil {
		fmt.Fprintf(stdout, "Domain config error: %v\n", err)
		return 1
	}

	path := RenderedOutlinePath(manuscriptDir)
	if err := os.WriteFile(path, []byte(RenderOutline(joined, rowFields, sectionFields)), 0o644); err != nil {
		fmt.Fprintf(stdout, "Error writing %s: %v\n", path, err)
		return 1
	}
	// Print the absolute path: telling an author only that "the outline is
	// written" is how a run ends with them unable to find its one
	// deliverable.
	abs, absErr := filepath.Abs(path)
	if absErr != nil {
		abs = path
	}
	fmt.Fprintf(stdout, "Wrote %s\n", abs)
	return 0
}

// outlineFieldsFor resolves the manuscript's domain and returns its
// configured outline fields. A domain with no outline block yields empty
// lists rather than an error -- the caller has already decided the
// layer applies.
//
// Takes domainsDir as a parameter rather than resolving it via
// schemas.FindDomainsDir() -- see RunOutline's doc comment. A call to
// FindDomainsDir() here would search from whatever binary is currently
// running (the MCP server's process, in that front door) instead of
// reusing the domainsDir that front door already resolved once.
func outlineFieldsFor(manuscriptDir, domainsDir string) ([]string, []string, error) {
	state, _ := schemas.LoadState(manuscriptDir)
	name := schemas.DefaultDomain
	if state != nil {
		name = state.DomainName()
	}
	domain, err := schemas.LoadDomain(domainsDir, name)
	if err != nil {
		return nil, nil, err
	}
	return domain.OutlineRowFields(), domain.OutlineSectionFields(), nil
}

func OutlineVersionsDir(manuscriptDir string) string {
	return filepath.Join(OutlineDir(manuscriptDir), "versions")
}

// OutlineVersionMeta is one archived version's small sidecar. Timestamps
// are RFC3339, same as state's, and are informational only -- version
// ordering comes from the counter, never from mtime.
type OutlineVersionMeta struct {
	Version         int      `json:"version"`
	ArchivedAt      string   `json:"archived_at"`
	ChangedSections []string `json:"changed_sections"`
	SceneCount      int      `json:"scene_count"`
}

// ArchiveOutlineVersion writes a new version when the joined outline
// differs from the newest archived one, and does nothing otherwise.
//
// This cadence is the point. Keyed to the developmental round the way
// continuity is, the layer would produce no history at all for its
// primary workflow -- the author's loop is write a chapter, outline,
// write the next, outline, a loop that need never run `assess`. Every
// one of those runs would overwrite outline.json with nothing kept.
//
// Both the JSON and the rendered Markdown are archived. The Markdown is
// what makes a version readable by a person; without it, "see version 4"
// means hand-reading JSON inside a hidden directory. It costs a file
// copy because the render is deterministic.
//
// Resolves its own domainsDir via schemas.FindDomainsDir() -- correct
// here because, unlike cmdOutlineRender, this function has no MCP-facing
// caller yet (only RunOutline's CLI path and tests call it), so there is
// no already-resolved domainsDir to reuse. If that changes, thread
// domainsDir through the same way RunOutlineWithDomainsDir does.
func ArchiveOutlineVersion(manuscriptDir string, changedSections []string) (string, bool, error) {
	joined, err := ComputeOutlineJoin(manuscriptDir)
	if err != nil {
		return "", false, err
	}
	raw, err := json.MarshalIndent(joined, "", "  ")
	if err != nil {
		return "", false, err
	}
	raw = append(raw, '\n')

	state, err := schemas.LoadState(manuscriptDir)
	if err != nil {
		return "", false, err
	}
	if state == nil {
		return "", false, fmt.Errorf("no state in %s", manuscriptDir)
	}

	// Compare against the newest archived version rather than against
	// outline.json, which the caller may already have rewritten this run.
	if state.OutlineVersion > 0 {
		previous := filepath.Join(OutlineVersionsDir(manuscriptDir), fmt.Sprintf("v%d", state.OutlineVersion), "outline.json")
		if existing, err := os.ReadFile(previous); err == nil && string(existing) == string(raw) {
			return "", false, nil
		}
	}

	next := state.OutlineVersion + 1
	dest := filepath.Join(OutlineVersionsDir(manuscriptDir), fmt.Sprintf("v%d", next))
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return "", false, err
	}
	if err := os.WriteFile(filepath.Join(dest, "outline.json"), raw, 0o644); err != nil {
		return "", false, err
	}

	domainsDir, err := schemas.FindDomainsDir()
	if err != nil {
		return "", false, err
	}
	rowFields, sectionFields, err := outlineFieldsFor(manuscriptDir, domainsDir)
	if err != nil {
		return "", false, err
	}
	rendered := RenderOutline(joined, rowFields, sectionFields)
	if err := os.WriteFile(filepath.Join(dest, "Outline.md"), []byte(rendered), 0o644); err != nil {
		return "", false, err
	}

	sceneCount := 0
	if n, ok := joined["scene_count"].(int); ok {
		sceneCount = n
	}
	meta := OutlineVersionMeta{
		Version:         next,
		ArchivedAt:      schemas.NowISO(),
		ChangedSections: orEmptyStrings(changedSections),
		SceneCount:      sceneCount,
	}
	metaRaw, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return "", false, err
	}
	if err := os.WriteFile(filepath.Join(dest, "meta.json"), append(metaRaw, '\n'), 0o644); err != nil {
		return "", false, err
	}

	state.OutlineVersion = next
	if _, err := schemas.SaveState(manuscriptDir, state); err != nil {
		return "", false, err
	}
	return dest, true, nil
}

func cmdOutlineVersions(manuscriptDir string, stdout io.Writer) int {
	entries, err := os.ReadDir(OutlineVersionsDir(manuscriptDir))
	if err != nil || len(entries) == 0 {
		fmt.Fprintln(stdout, "No outline versions archived yet.")
		return 0
	}

	type row struct {
		meta OutlineVersionMeta
		path string
	}
	var rows []row
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(OutlineVersionsDir(manuscriptDir), e.Name())
		metaRaw, err := os.ReadFile(filepath.Join(dir, "meta.json"))
		if err != nil {
			continue
		}
		var meta OutlineVersionMeta
		if err := json.Unmarshal(metaRaw, &meta); err != nil {
			continue
		}
		rows = append(rows, row{meta: meta, path: dir})
	}
	if len(rows) == 0 {
		fmt.Fprintln(stdout, "No outline versions archived yet.")
		return 0
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].meta.Version < rows[j].meta.Version })

	fmt.Fprintf(stdout, "Archived outline versions (%d):\n", len(rows))
	for _, r := range rows {
		changed := "no sections re-recorded"
		if len(r.meta.ChangedSections) > 0 {
			changed = "changed: " + strings.Join(r.meta.ChangedSections, ", ")
		}
		fmt.Fprintf(stdout, "  v%-4d %s  %d scene(s), %s\n", r.meta.Version, r.meta.ArchivedAt, r.meta.SceneCount, changed)
		// Print the readable path, not just the version: reading a version
		// means opening this file.
		fmt.Fprintf(stdout, "         %s\n", filepath.Join(r.path, "Outline.md"))
	}
	return 0
}
