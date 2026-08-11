package cli

import (
	"fmt"
	"io"

	"github.com/tcotav/redliner/go/internal/schemas"
)

const domainUsage = `Usage:
  redliner domain list
  redliner domain show <name>`

func RunDomain(args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stdout, domainUsage)
		return 1
	}
	domainsDir, err := schemas.FindDomainsDir()
	if err != nil {
		fmt.Fprintf(stdout, "Domain config error: %v\n", err)
		return 1
	}

	switch args[0] {
	case "list":
		return cmdDomainList(domainsDir, stdout, stderr)
	case "show":
		if len(args) < 2 {
			fmt.Fprintln(stdout, "show requires a domain name")
			return 1
		}
		return cmdDomainShow(domainsDir, args[1], stdout)
	default:
		fmt.Fprintf(stdout, "Unknown command %s\n", pyReprStr(args[0]))
		fmt.Fprintln(stdout, domainUsage)
		return 1
	}
}

// DomainSummary is one entry of `domain list`'s output -- mirrors
// redliner_domain.py's cmd_list.
type DomainSummary struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
}

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

func cmdDomainShow(domainsDir, name string, stdout io.Writer) int {
	d, err := schemas.LoadDomain(domainsDir, name)
	if err != nil {
		fmt.Fprintf(stdout, "Domain config error: %v\n", err)
		return 1
	}
	return printJSON(stdout, d)
}
