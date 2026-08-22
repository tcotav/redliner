package mcpserver

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Every `redliner <group> <command>` the skill files tell an agent to run.
// The skills directory is shared between both plugin variants by symlink
// (cowork/skills -> ../skills), so a command with no MCP tool behind it
// leaves a Cowork session unable to finish a run -- while the CLI variant
// works fine and nothing fails loudly anywhere.
//
// That is not hypothetical. Between v0.4.0 and v0.5.0, five commands
// shipped with no tool (decisions apply, rounds archive, rounds list,
// state stage, state pass) and the Cowork plugin could not complete a
// pass for two releases. It went unnoticed because both front doors are
// tested thoroughly and *separately*, and nothing tested the claim the
// README makes about them: that identical skill prose drives either one.
//
// This is that test. It is deliberately dumb -- it reads the skill files
// as text, because that is the artifact the agent actually follows.
var skillCommandPattern = regexp.MustCompile(`redliner ([a-z]+) ([a-z]+)`)

// Prose in the skills happens to match the pattern ("redliner is for",
// "redliner has been"). Listed explicitly rather than filtered by
// heuristic, so a genuinely new command can never be silently mistaken
// for a stray English phrase.
var notCommands = map[string]bool{
	"directory you": true, "does and": true, "has been": true,
	"is for": true, "advises the": true, "suggests it": true,
	"editing pass": true, "run on": true, "to work": true,
}

// commandToTool maps a CLI subcommand to the MCP tool that must exist for
// the same instruction to be followable on the Cowork front door. A
// command whose group alone is the tool (e.g. `state status` ->
// `state_status`) follows the group_command convention; anything that
// diverges is listed here.
var commandToTool = map[string]string{
	"canon reconcile":  "canon_reconcile",
	"canon stale":      "canon_stale",
	"canon bundle":     "canon_bundle",
	"canon merge":      "canon_merge",
	"state status":     "state_status",
	"state diff":       "state_diff",
	"state snapshot":   "state_snapshot",
	"state phase":      "state_phase",
	"state init":       "state_init",
	"state stage":      "state_stage",
	"state pass":       "state_pass",
	"rounds archive":   "rounds_archive",
	"rounds list":      "rounds_list",
	"decisions apply":  "decisions_apply",
	"decisions list":   "decisions_apply", // read-only view of the same file
	"domain list":      "domain_list",
	"domain show":      "domain_show",
	"outline stale":    "outline_stale",
	"outline join":     "outline_join",
	"outline render":   "outline_render",
	"outline archive":  "outline_archive",
	"outline versions": "outline_versions",
}

func skillFiles(t *testing.T) []string {
	t.Helper()
	root := repoRootForParity(t)
	var out []string
	err := filepath.Walk(filepath.Join(root, "skills"), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".md") {
			out = append(out, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking skills/: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("found no skill files -- this guard would pass vacuously")
	}
	return out
}

func repoRootForParity(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(filepath.Join(dir, "skills")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	t.Fatal("could not locate repo root from the test's working directory")
	return ""
}

func TestEverySkillCommandHasAnMCPTool(t *testing.T) {
	commands := map[string][]string{} // command -> skill files mentioning it
	for _, path := range skillFiles(t) {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range skillCommandPattern.FindAllStringSubmatch(string(body), -1) {
			cmd := m[1] + " " + m[2]
			if notCommands[cmd] {
				continue
			}
			commands[cmd] = append(commands[cmd], filepath.Base(filepath.Dir(path)))
		}
	}
	if len(commands) == 0 {
		t.Fatal("matched no commands -- the pattern or the skill files changed shape")
	}

	srv := NewServer(filepath.Join(repoRootForParity(t), "domains"))
	tools := map[string]bool{}
	for _, tl := range listToolsForParity(t, srv) {
		tools[tl.Name] = true
	}

	var names []string
	for cmd := range commands {
		names = append(names, cmd)
	}
	sort.Strings(names)

	for _, cmd := range names {
		want, mapped := commandToTool[cmd]
		if !mapped {
			t.Errorf("skill files invoke %q (in %v) but no MCP tool is mapped for it.\n"+
				"Either add the tool and map it in commandToTool, or add it to notCommands if it is prose.",
				"redliner "+cmd, commands[cmd])
			continue
		}
		if !tools[want] {
			t.Errorf("skill files invoke %q (in %v) but the MCP server exposes no %q tool -- "+
				"the Cowork variant cannot follow that instruction.",
				"redliner "+cmd, commands[cmd], want)
		}
	}
}

func listToolsForParity(t *testing.T, srv *mcp.Server) []*mcp.Tool {
	t.Helper()
	ctx := context.Background()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := srv.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()

	client := mcp.NewClient(&mcp.Implementation{Name: "parity-test"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientSession.Close()

	res, err := clientSession.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	return res.Tools
}
