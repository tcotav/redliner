package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/tcotav/redliner/go/internal/schemas"
)

const validateUsage = `Usage:
  redliner validate <manuscript_dir>`

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

func loadSectionText(manuscriptDir, stem string) (string, bool) {
	for _, ext := range schemas.SectionExtensions {
		path := filepath.Join(manuscriptDir, stem+ext)
		if raw, err := os.ReadFile(path); err == nil {
			return string(raw), true
		}
	}
	return "", false
}

// verifyExcerpts mirrors validate_findings.py's _verify_excerpts: each
// item's excerpt (if present) must be a genuine, normalized substring of
// the section it claims to quote.
func verifyExcerpts(items []interface{}, sectionText, label string) []string {
	var errors []string
	normalizedSection := normalizeExcerpt(sectionText)
	for i, raw := range items {
		item, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		excerpt, _ := item["excerpt"].(string)
		if excerpt == "" {
			continue
		}
		if !strings.Contains(normalizedSection, normalizeExcerpt(excerpt)) {
			id, ok := item["id"].(string)
			if !ok || id == "" {
				id = fmt.Sprintf("index %d", i)
			}
			errors = append(errors, fmt.Sprintf("%s[%s]: excerpt not found verbatim in section text: %s", label, id, pyReprStr(excerpt)))
		}
	}
	return errors
}

// RunValidate mirrors validate_findings.py's main(): validates
// everything under a manuscript's .redliner/ (canon observations,
// continuity, developmental/line findings, editorial letter) against
// its domain's schema, including excerpt-verbatim checks.
//
// Resolves its own domainsDir via schemas.FindDomainsDir() -- correct
// for the CLI, where os.Executable() really is this binary. Reused by
// internal/mcpserver's validate_findings tool, which does NOT go through
// this entry point for that reason (a second FindDomainsDir() call
// there would search from whatever binary the calling test/process
// happens to be, not from the domainsDir the MCP server already
// resolved once) -- see ValidateManuscript below, which takes
// domainsDir as a parameter instead of rediscovering it.
func RunValidate(args []string, stdout io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stdout, validateUsage)
		return 1
	}
	domainsDir, err := schemas.FindDomainsDir()
	if err != nil {
		fmt.Fprintf(stdout, "Domain config error: %v\n", err)
		return 1
	}
	return ValidateManuscript(args[0], domainsDir, stdout)
}

// ValidateManuscript is RunValidate's logic with domainsDir taken as a
// parameter instead of resolved internally -- the piece
// internal/mcpserver's validate_findings tool actually calls, passing
// the same domainsDir every other tool in that server uses.
func ValidateManuscript(manuscriptDir, domainsDir string, stdout io.Writer) int {
	redlinerPath := schemas.StateDir(manuscriptDir)
	if info, err := os.Stat(redlinerPath); err != nil || !info.IsDir() {
		fmt.Fprintf(stdout, "No .redliner/ under %s -- pass a manuscript directory, not a findings/canon path.\n", manuscriptDir)
		return 1
	}

	state, _ := schemas.LoadState(manuscriptDir)
	domainName := schemas.DefaultDomain
	if state != nil {
		domainName = state.DomainName()
	}
	domain, err := schemas.LoadDomain(domainsDir, domainName)
	if err != nil {
		fmt.Fprintf(stdout, "Domain config error: %v\n", err)
		return 1
	}

	ok := validateCanon(stdout, manuscriptDir, redlinerPath, domain)

	findingsPath := filepath.Join(redlinerPath, "findings")
	if info, err := os.Stat(findingsPath); err != nil || !info.IsDir() {
		fmt.Fprintf(stdout, "No findings/ yet under %s\n", redlinerPath)
		if ok {
			return 0
		}
		return 1
	}

	devFile := filepath.Join(findingsPath, "developmental.json")
	devExists := fileExists(devFile)
	if devExists {
		report := loadJSON(devFile)
		errs := schemas.ValidateDevelopmentalReport(report, domain.StringSet("developmental_categories"))
		// Developmental findings don't carry excerpts -- manuscript-scope,
		// not tied to one quotable location.
		ok = checkFile(stdout, devFile, errs) && ok
	}

	lineFiles, _ := filepath.Glob(filepath.Join(findingsPath, "line_*.json"))
	sort.Strings(lineFiles)
	for _, lineFile := range lineFiles {
		report := loadJSON(lineFile)
		errs := schemas.ValidateLineReport(report, domain.StringSet("line_categories"))
		if m := lineFilePattern.FindStringSubmatch(filepath.Base(lineFile)); m != nil {
			if sectionText, found := loadSectionText(manuscriptDir, m[1]); found {
				errs = append(errs, verifyExcerpts(reportField(report, "findings"), sectionText, filepath.Base(lineFile))...)
			}
		}
		ok = checkFile(stdout, lineFile, errs) && ok
	}

	letterFile := filepath.Join(findingsPath, "editorial_letter.json")
	letterExists := fileExists(letterFile)
	if letterExists {
		letter := loadJSON(letterFile)
		errs := schemas.ValidateEditorialLetter(letter)
		ok = checkFile(stdout, letterFile, errs) && ok
	}

	if !devExists && len(lineFiles) == 0 && !letterExists {
		fmt.Fprintf(stdout, "Nothing to validate yet in %s\n", findingsPath)
	}

	if ok {
		return 0
	}
	return 1
}

// validateCanon mirrors validate_findings.py's _validate_canon: the
// continuity layer's files, if any exist yet.
func validateCanon(stdout io.Writer, manuscriptDir, redlinerPath string, domain schemas.Domain) bool {
	canonPath := filepath.Join(redlinerPath, "canon")
	if info, err := os.Stat(canonPath); err != nil || !info.IsDir() {
		return true
	}

	continuity := domain.Continuity()
	entityTypes := continuity.StringSet("entity_types")
	sources := continuity.StringSet("sources")
	categories := continuity.StringSet("categories")

	ok := true

	obsFiles, _ := filepath.Glob(filepath.Join(canonPath, "observations", "*.json"))
	sort.Strings(obsFiles)
	for _, obsFile := range obsFiles {
		report := loadJSON(obsFile)
		errs := schemas.ValidateObservations(report, entityTypes, sources)
		stem := stemOfPath(obsFile)
		if sectionText, found := loadSectionText(manuscriptDir, stem); found {
			errs = append(errs, verifyExcerpts(reportField(report, "facts"), sectionText, filepath.Base(obsFile))...)
		}
		ok = checkFile(stdout, obsFile, errs) && ok
	}

	continuityFile := filepath.Join(canonPath, "continuity.json")
	if fileExists(continuityFile) {
		report := loadJSON(continuityFile)
		errs := schemas.ValidateContinuityReport(report, categories)
		ok = checkFile(stdout, continuityFile, errs) && ok
	}

	return ok
}
