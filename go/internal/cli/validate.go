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
func verifyExcerpts(items []interface{}, sectionText, label string, allowMulti bool) []string {
	var errors []string
	normalizedSection := normalizeExcerpt(sectionText)
	for i, raw := range items {
		item, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		excerptRaw, present := item["excerpt"]
		if !present || excerptRaw == nil {
			continue
		}

		id, ok := item["id"].(string)
		if !ok || id == "" {
			id = fmt.Sprintf("index %d", i)
		}
		fail := func(format string, args ...interface{}) {
			errors = append(errors, fmt.Sprintf("%s[%s]: %s", label, id, fmt.Sprintf(format, args...)))
		}
		verify := func(excerpt, where string) {
			if !strings.Contains(normalizedSection, normalizeExcerpt(excerpt)) {
				fail("excerpt%s not found verbatim in section text: %s", where, pyReprStr(excerpt))
			}
		}

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
				errs = append(errs, verifyExcerpts(reportField(report, "findings"), sectionText, filepath.Base(lineFile), true)...)
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
			errs = append(errs, verifyExcerpts(reportField(report, "facts"), sectionText, filepath.Base(obsFile), false)...)
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
