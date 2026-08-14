package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tcotav/redliner/go/internal/schemas"
)

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
type Decision struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
	SetBy  string `json:"set_by"`
	At     string `json:"at"`
}

type decisionsFile struct {
	Decisions []Decision `json:"decisions"`
}

const decisionsUsage = `Usage:
  redliner decisions list  <manuscript_dir>
  redliner decisions apply <manuscript_dir>   # re-apply author decisions over findings`

func decisionsPath(manuscriptDir string) string {
	return filepath.Join(schemas.StateDir(manuscriptDir), "decisions.json")
}

func loadDecisions(manuscriptDir string) ([]Decision, error) {
	raw, err := os.ReadFile(decisionsPath(manuscriptDir))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var f decisionsFile
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, err
	}
	return f.Decisions, nil
}

// RunDecisions implements `redliner decisions <subcommand> <dir>`.
func RunDecisions(args []string, stdout io.Writer) int {
	if len(args) < 2 {
		fmt.Fprintln(stdout, decisionsUsage)
		return 1
	}
	command, manuscriptDir := args[0], args[1]
	if info, err := os.Stat(manuscriptDir); err != nil || !info.IsDir() {
		fmt.Fprintf(stdout, "No such directory: %s\n", manuscriptDir)
		return 1
	}

	decisions, err := loadDecisions(manuscriptDir)
	if err != nil {
		fmt.Fprintf(stdout, "Error reading decisions: %v\n", err)
		return 1
	}

	switch command {
	case "list":
		return printJSON(stdout, decisionsFile{Decisions: decisions})
	case "apply":
		return cmdDecisionsApply(manuscriptDir, decisions, stdout)
	default:
		fmt.Fprintf(stdout, "Unknown command %s\n", pyReprStr(command))
		fmt.Fprintln(stdout, decisionsUsage)
		return 1
	}
}

// cmdDecisionsApply re-applies every recorded decision over the findings
// files, reporting what it had to restore and what it could not place.
func cmdDecisionsApply(manuscriptDir string, decisions []Decision, stdout io.Writer) int {
	if len(decisions) == 0 {
		fmt.Fprintln(stdout, "No recorded decisions; nothing to apply.")
		return 0
	}
	byID := map[string]Decision{}
	for _, d := range decisions {
		byID[d.ID] = d
	}

	findingsDir := filepath.Join(schemas.StateDir(manuscriptDir), "findings")
	paths, _ := filepath.Glob(filepath.Join(findingsDir, "*.json"))
	sort.Strings(paths)

	var restored, alreadyOK []string
	seen := map[string]bool{}

	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var doc map[string]interface{}
		if err := json.Unmarshal(raw, &doc); err != nil {
			fmt.Fprintf(stdout, "Skipping unparseable %s: %v\n", filepath.Base(path), err)
			continue
		}
		findings, ok := doc["findings"].([]interface{})
		if !ok {
			continue
		}
		changed := false
		for _, item := range findings {
			finding, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			id, _ := finding["id"].(string)
			decision, wanted := byID[id]
			if !wanted {
				continue
			}
			seen[id] = true
			if current, _ := finding["status"].(string); current == decision.Status {
				alreadyOK = append(alreadyOK, id)
				continue
			}
			finding["status"] = decision.Status
			res := map[string]interface{}{"set_by": decision.SetBy, "at": decision.At}
			if decision.Reason != "" {
				res["reason"] = decision.Reason
			}
			finding["resolution"] = res
			restored = append(restored, id)
			changed = true
		}
		if changed {
			out, err := json.MarshalIndent(doc, "", "  ")
			if err != nil {
				fmt.Fprintf(stdout, "Error encoding %s: %v\n", filepath.Base(path), err)
				return 1
			}
			if err := os.WriteFile(path, append(out, '\n'), 0o644); err != nil {
				fmt.Fprintf(stdout, "Error writing %s: %v\n", filepath.Base(path), err)
				return 1
			}
		}
	}

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
}
