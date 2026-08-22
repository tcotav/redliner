package mcpserver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/tcotav/redliner/go/internal/cli"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not determine test file location")
	}
	// go/internal/mcpserver/server_test.go -> go/internal/mcpserver -> go/internal -> go -> repo root
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
}

func copyDir(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, raw, info.Mode())
	})
	if err != nil {
		t.Fatalf("copying fixture: %v", err)
	}
}

// connect spins up a real in-process client/server pair over
// mcp.NewInMemoryTransports -- an actual MCP protocol round trip
// (tools/list, tools/call JSON-RPC), not a direct Go function call, so
// this exercises the same request path a real client (Claude via
// Cowork) does. Mirrors the pattern used in the SDK's own example tests.
func connect(t *testing.T, srv *mcp.Server) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.0.1"}, nil)
	t1, t2 := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, t1, nil); err != nil {
		t.Fatalf("server.Connect: %v", err)
	}
	session, err := client.Connect(ctx, t2, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	t.Cleanup(func() { session.Close() })
	return session
}

func callTool(t *testing.T, session *mcp.ClientSession, name string, args map[string]any) (map[string]any, bool) {
	t.Helper()
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		t.Fatalf("CallTool(%s): %v", name, err)
	}
	if res.IsError {
		var texts string
		for _, c := range res.Content {
			if tc, ok := c.(*mcp.TextContent); ok {
				texts += tc.Text
			}
		}
		t.Fatalf("CallTool(%s) returned a protocol-level tool error: %s", name, texts)
	}
	out, ok := res.StructuredContent.(map[string]any)
	return out, ok
}

// TestToolNamesAndDescriptions_MatchPython locks the 10 tool names and
// their descriptions against golden/mcp_tool_descriptions.json --
// extracted directly from cowork/mcp_server.py's real docstrings via
// Python's own `ast.get_docstring` (see that file's header comment for
// the exact extraction command), not against this package's own
// descriptions.go constants. Comparing against a second copy of the same
// constants would only prove the SDK plumbs whatever string it's given
// through unchanged -- it would say nothing about whether the
// hand-transcription into descriptions.go actually matches Python,
// which is the one thing TODO.md calls a frozen interface (it's what
// made Claude pick the right tools unprompted in the original Cowork
// spike). This also keeps working once the Python is deleted in a later
// phase, since the golden file -- not the live source -- is the
// reference from here on.
func TestToolNamesAndDescriptions_MatchPython(t *testing.T) {
	domainsDir := filepath.Join(repoRoot(t), "domains")
	session := connect(t, NewServer(domainsDir))

	res, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	goldenPath := filepath.Join(repoRoot(t), "go", "harness", "golden", "mcp_tool_descriptions.json")
	raw, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("reading %s: %v", goldenPath, err)
	}
	var want map[string]string
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatalf("parsing %s: %v", goldenPath, err)
	}
	if len(want) != 10 {
		t.Fatalf("golden file has %d descriptions, want 10 -- did extraction pick up every @mcp.tool()?", len(want))
	}

	// Go-only composites over the ported operations. They have no Python
	// counterpart by design, so they are exempt from the description
	// comparison below -- but they must be listed here explicitly, so
	// adding a tool stays a deliberate act rather than something that
	// silently slips past this guard.
	goOnly := map[string]bool{
		"context":          true,
		"decisions_apply":  true,
		"rounds_archive":   true,
		"rounds_list":      true,
		"state_stage":      true,
		"state_pass":       true,
		"canon_bundle":     true,
		"canon_merge":      true,
		"outline_stale":    true,
		"outline_join":     true,
		"outline_render":   true,
		"outline_versions": true,
	}

	// A ported tool that has grown a Go-only parameter needs somewhere to
	// document it -- the description is what the calling model reads to
	// decide how to invoke the tool, so a flag that exists but isn't
	// described is a flag no model will pass. These tools' descriptions
	// must still *begin* with Python's docstring verbatim (so drift in
	// the ported part is still caught); what follows is Go-only text.
	// Named individually, for the same reason goOnly is: growing this
	// list stays a deliberate act.
	goOnlyAddendum := map[string]bool{
		"canon_reconcile": true, // snapshot_after, added 2026-08-15
	}

	got := map[string]string{}
	for _, tool := range res.Tools {
		if goOnly[tool.Name] {
			continue
		}
		desc := tool.Description
		if goOnlyAddendum[tool.Name] {
			if prefix, ok := want[tool.Name]; ok && strings.HasPrefix(desc, prefix) {
				if strings.TrimSpace(desc[len(prefix):]) == "" {
					t.Errorf("tool %q is listed as carrying a Go-only addendum but has none", tool.Name)
				}
				desc = prefix
			}
		}
		got[tool.Name] = desc
	}

	if len(got) != len(want) {
		t.Errorf("ported tool count: got %d, want %d (all names: %v; Go-only exemptions: %v)",
			len(got), len(want), toolNames(res.Tools), goOnly)
	}
	for name, desc := range want {
		gotDesc, ok := got[name]
		if !ok {
			t.Errorf("missing tool %q", name)
			continue
		}
		if gotDesc != desc {
			t.Errorf("tool %q description mismatch (Go descriptions.go has drifted from Python's real docstring)\n got: %q\nwant: %q", name, gotDesc, desc)
		}
	}
}

func toolNames(tools []*mcp.Tool) []string {
	names := make([]string, len(tools))
	for i, tl := range tools {
		names[i] = tl.Name
	}
	return names
}

// loadGoldenStdoutJSON reads a capture_baseline.py step file's
// stdout_json field -- the real Python-captured output, same file
// go/internal/cli/golden_test.go diffs the CLI against. Used here so
// this file's expectations (fact_count, collision count, verdict, ...)
// come from Python's real output, not a number typed by hand into the
// test -- if capture_baseline.py is ever re-run and the numbers change,
// this test's expectations move with it instead of silently going
// stale.
func loadGoldenStdoutJSON(t *testing.T, fixture, step string) map[string]any {
	t.Helper()
	path := filepath.Join(repoRoot(t), "go", "harness", "golden", fixture, step+".json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading golden %s: %v", path, err)
	}
	var entry struct {
		StdoutJSON map[string]any `json:"stdout_json"`
	}
	if err := json.Unmarshal(raw, &entry); err != nil {
		t.Fatalf("parsing golden %s: %v", path, err)
	}
	if entry.StdoutJSON == nil {
		t.Fatalf("golden %s has no stdout_json", path)
	}
	return entry.StdoutJSON
}

// TestMCPTools_MatchGolden re-runs the same happy-fixture operations the
// CLI's differential test does (go/internal/cli/golden_test.go), through
// real MCP tool calls instead of cli.Dispatch, and checks each result
// against the same real Python-captured golden data -- proving the two
// front doors actually agree with Python, not just with each other or
// with a number typed into this file by hand.
func TestMCPTools_MatchGolden(t *testing.T) {
	domainsDir := filepath.Join(repoRoot(t), "domains")
	fixturesRoot := filepath.Join(repoRoot(t), "go", "harness", "fixtures")
	workRoot := filepath.Join(repoRoot(t), "go", "harness", ".work-mcp")

	manuscriptDir := filepath.Join(workRoot, "happy")
	os.RemoveAll(manuscriptDir)
	t.Cleanup(func() { os.RemoveAll(manuscriptDir) })
	copyDir(t, filepath.Join(fixturesRoot, "happy"), manuscriptDir)

	session := connect(t, NewServer(domainsDir))

	t.Run("domain_list", func(t *testing.T) {
		res, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "domain_list", Arguments: map[string]any{}})
		if err != nil || res.IsError {
			t.Fatalf("domain_list failed: err=%v isError=%v", err, res != nil && res.IsError)
		}
		raw, _ := json.Marshal(res.StructuredContent)
		var domains []map[string]any
		if err := json.Unmarshal(raw, &domains); err != nil {
			t.Fatalf("domain_list result not a JSON array: %v", err)
		}
		names := map[string]bool{}
		for _, d := range domains {
			names[d["name"].(string)] = true
		}
		for _, want := range []string{"fiction", "design-doc", "serial-fiction"} {
			if !names[want] {
				t.Errorf("domain_list missing %q: got %v", want, names)
			}
		}
	})

	t.Run("state_status", func(t *testing.T) {
		want := loadGoldenStdoutJSON(t, "happy", "03_state_status")
		out, ok := callTool(t, session, "state_status", map[string]any{"manuscript_dir": manuscriptDir})
		if !ok {
			t.Fatal("state_status did not return a structured object")
		}
		if out["domain"] != want["domain"] || out["phase"] != want["phase"] {
			t.Errorf("state_status mismatch: got %+v, want (from golden) domain=%v phase=%v", out, want["domain"], want["phase"])
		}
	})

	t.Run("state_diff_unchanged", func(t *testing.T) {
		want := loadGoldenStdoutJSON(t, "happy", "04_state_diff")
		out, ok := callTool(t, session, "state_diff", map[string]any{"manuscript_dir": manuscriptDir})
		if !ok {
			t.Fatal("state_diff did not return a structured object")
		}
		if out["verdict"] != want["verdict"] {
			t.Errorf("state_diff verdict: got %v, want %v (from golden)", out["verdict"], want["verdict"])
		}
	})

	t.Run("canon_reconcile_matches_cli", func(t *testing.T) {
		// 06_canon_reconcile's stdout is human-readable summary lines,
		// not JSON (no stdout_json) -- the real Python-verified
		// reference for canon.json/collisions.json's *content* is this
		// step's captured state_dir_snapshot (what capture_baseline.py
		// read back off disk after the real bin/redliner_canon.py ran),
		// the same field go/internal/cli/golden_test.go diffs the CLI
		// against. Comparing the MCP result against a file this same
		// call just wrote would be circular -- this compares against an
		// independent, previously-captured Python run instead.
		var entry struct {
			StateDirSnapshot map[string]json.RawMessage `json:"state_dir_snapshot"`
		}
		goldenPath := filepath.Join(repoRoot(t), "go", "harness", "golden", "happy", "06_canon_reconcile.json")
		raw, err := os.ReadFile(goldenPath)
		if err != nil {
			t.Fatalf("reading %s: %v", goldenPath, err)
		}
		if err := json.Unmarshal(raw, &entry); err != nil {
			t.Fatalf("parsing %s: %v", goldenPath, err)
		}
		var wantCanon struct {
			FactCount float64 `json:"fact_count"`
		}
		json.Unmarshal(entry.StateDirSnapshot["canon/canon.json"], &wantCanon)
		var wantCollisions struct {
			Collisions []any `json:"collisions"`
		}
		json.Unmarshal(entry.StateDirSnapshot["canon/collisions.json"], &wantCollisions)

		out, ok := callTool(t, session, "canon_reconcile", map[string]any{"manuscript_dir": manuscriptDir})
		if !ok {
			t.Fatal("canon_reconcile did not return a structured object")
		}
		canon, _ := out["canon"].(map[string]any)
		if canon == nil {
			t.Fatal("canon_reconcile result missing 'canon'")
		}
		if int(canon["fact_count"].(float64)) != int(wantCanon.FactCount) {
			t.Errorf("fact_count: got %v, want %v (from the real Python capture)", canon["fact_count"], wantCanon.FactCount)
		}
		collisions, _ := out["collisions"].([]any)
		if len(collisions) != len(wantCollisions.Collisions) {
			t.Errorf("collisions: got %d, want %d (from the real Python capture)", len(collisions), len(wantCollisions.Collisions))
		}

		// The write side effect must also match what the CLI produces --
		// read canon.json back off disk.
		canonPath := filepath.Join(manuscriptDir, ".redliner", "canon", "canon.json")
		if _, err := os.Stat(canonPath); err != nil {
			t.Errorf("canon_reconcile should have written %s: %v", canonPath, err)
		}
	})

	t.Run("validate_findings", func(t *testing.T) {
		out, ok := callTool(t, session, "validate_findings", map[string]any{"manuscript_dir": manuscriptDir})
		if !ok {
			t.Fatal("validate_findings did not return a structured object")
		}
		if out["ok"] != true {
			t.Errorf("validate_findings ok: got %v, want true\noutput: %v", out["ok"], out["output"])
		}
	})

	t.Run("canon_stale_nothing_stale", func(t *testing.T) {
		out, ok := callTool(t, session, "canon_stale", map[string]any{"manuscript_dir": manuscriptDir})
		if !ok {
			t.Fatal("canon_stale did not return a structured object")
		}
		needs, _ := out["needs_extraction"].([]any)
		if len(needs) != 0 {
			t.Errorf("needs_extraction: got %v, want none (both sections already extracted in this fixture)", needs)
		}
	})

	t.Run("domain_show_fiction", func(t *testing.T) {
		out, ok := callTool(t, session, "domain_show", map[string]any{"name": "fiction"})
		if !ok {
			t.Fatal("domain_show did not return a structured object")
		}
		if out["round_tracked_phase"] != "developmental" {
			t.Errorf("domain_show: got round_tracked_phase=%v, want developmental", out["round_tracked_phase"])
		}
	})

	t.Run("state_phase_round_trip", func(t *testing.T) {
		// developmental -> intake -> developmental, same round-increment
		// rule the CLI's golden test already verifies against Python;
		// this checks the MCP front door produces the same effect.
		out, ok := callTool(t, session, "state_phase", map[string]any{"manuscript_dir": manuscriptDir, "phase": "intake"})
		if !ok || out["phase"] != "intake" {
			t.Fatalf("state_phase -> intake: %+v (ok=%v)", out, ok)
		}
		out, ok = callTool(t, session, "state_phase", map[string]any{"manuscript_dir": manuscriptDir, "phase": "developmental"})
		if !ok {
			t.Fatal("state_phase -> developmental did not return a structured object")
		}
		if out["phase"] != "developmental" {
			t.Errorf("phase: got %v, want developmental", out["phase"])
		}
		if int(out["developmental_round"].(float64)) != 2 {
			t.Errorf("developmental_round: got %v, want 2 (was 1, re-entering developmental increments it)", out["developmental_round"])
		}
	})
}

// TestMCPTools_StateInitAndSnapshot exercises the two tools the happy
// fixture's already-initialized state can't (state_init requires no
// prior state; state_snapshot's effect is easiest to see on a fixture
// state_diff already covers separately) -- a fresh manuscript dir
// through init -> status -> snapshot.
func TestMCPTools_StateInitAndSnapshot(t *testing.T) {
	domainsDir := filepath.Join(repoRoot(t), "domains")
	fixturesRoot := filepath.Join(repoRoot(t), "go", "harness", "fixtures")
	workRoot := filepath.Join(repoRoot(t), "go", "harness", ".work-mcp")

	manuscriptDir := filepath.Join(workRoot, "crlf")
	os.RemoveAll(manuscriptDir)
	t.Cleanup(func() { os.RemoveAll(manuscriptDir) })
	copyDir(t, filepath.Join(fixturesRoot, "crlf"), manuscriptDir)

	session := connect(t, NewServer(domainsDir))

	out, ok := callTool(t, session, "state_init", map[string]any{"manuscript_dir": manuscriptDir})
	if !ok || out["status"] != "initialized" {
		t.Fatalf("state_init: %+v (ok=%v)", out, ok)
	}
	if out["domain"] != "fiction" {
		t.Errorf("state_init domain: got %v, want fiction (default)", out["domain"])
	}

	// Double-init must be a soft error, matching the CLI/Python contract.
	out, ok = callTool(t, session, "state_init", map[string]any{"manuscript_dir": manuscriptDir})
	if !ok || out["error"] == nil {
		t.Errorf("second state_init should return a soft error, got: %+v (ok=%v)", out, ok)
	}

	out, ok = callTool(t, session, "state_snapshot", map[string]any{"manuscript_dir": manuscriptDir})
	if !ok || out["status"] != "snapshotted" {
		t.Fatalf("state_snapshot: %+v (ok=%v)", out, ok)
	}
	if int(out["section_count"].(float64)) != 1 {
		t.Errorf("section_count: got %v, want 1", out["section_count"])
	}
}

// TestMCPTools_ErrorContract checks the "soft failure" contract TODO.md
// calls out: a bad domain, a double-init, a missing state -- these are
// all real usage errors, but the *tool call itself* must still succeed
// at the protocol level (IsError false), with the error message readable
// back as ordinary structured content. This is what let the model
// self-correct in the original Cowork spike instead of hitting an opaque
// protocol error.
func TestMCPTools_ErrorContract(t *testing.T) {
	domainsDir := filepath.Join(repoRoot(t), "domains")
	session := connect(t, NewServer(domainsDir))

	dir := t.TempDir()

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "state_status",
		Arguments: map[string]any{"manuscript_dir": dir},
	})
	if err != nil {
		t.Fatalf("state_status on an uninitialized dir returned a protocol error: %v", err)
	}
	if res.IsError {
		t.Fatal("state_status on an uninitialized dir set IsError -- should be a soft {'error': ...} result")
	}
	out, _ := res.StructuredContent.(map[string]any)
	if out["error"] == nil {
		t.Errorf("expected a soft error result, got: %+v", out)
	}
}

func TestOutlineToolsAreRegistered(t *testing.T) {
	srv := NewServer(filepath.Join(repoRootForParity(t), "domains"))
	tools := map[string]bool{}
	for _, tl := range listToolsForParity(t, srv) {
		tools[tl.Name] = true
	}
	for _, name := range []string{"outline_stale", "outline_join", "outline_render", "outline_versions"} {
		if !tools[name] {
			t.Errorf("MCP server exposes no %q tool -- the Cowork front door cannot follow the outline skill prose without it", name)
		}
	}
}

// TestOutlineTools_ActuallyWork calls outline_stale and outline_render
// through a real server and checks their real effects -- not just that
// the tool names exist (TestOutlineToolsAreRegistered), but that they're
// wired to the right logic. It would catch a tool registered against
// the wrong handler, a handler that always errors, or an input schema
// that doesn't match the field the CLI path expects.
//
// Deliberately constructs the server with the repo's real domains/ dir
// (not a temp dir), so outline_render's domain lookup runs the same
// path the MCP server always uses -- see runOutlineCommand's doc
// comment. Under the pre-fix code, outline_render resolved its domain
// config via schemas.FindDomainsDir() from os.Executable() instead of
// from the domainsDir passed to NewServer, so this exact call would
// have failed (or silently used the wrong domain) whenever the test
// binary wasn't sitting near a domains/ directory.
func TestOutlineTools_ActuallyWork(t *testing.T) {
	domainsDir := filepath.Join(repoRoot(t), "domains")
	session := connect(t, NewServer(domainsDir))
	ctx := context.Background()

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".redliner"), 0o755); err != nil {
		t.Fatal(err)
	}
	state := `{"manuscript_dir":"` + dir + `","domain":"fiction","phase":"developmental",` +
		`"developmental_round":1,"section_fingerprints":{},"created_at":"x"}`
	if err := os.WriteFile(filepath.Join(dir, ".redliner", "state.json"), []byte(state), 0o644); err != nil {
		t.Fatal(err)
	}
	sectionText := "section one body text\n"
	if err := os.WriteFile(filepath.Join(dir, "section_01.txt"), []byte(sectionText), 0o644); err != nil {
		t.Fatal(err)
	}

	// outline_stale should report section_01 as never recorded -- a real
	// answer computed from the fixture, not a canned success.
	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "outline_stale",
		Arguments: map[string]any{"manuscript_dir": dir},
	})
	if err != nil {
		t.Fatalf("outline_stale: protocol error: %v", err)
	}
	if res.IsError {
		t.Fatalf("outline_stale: unexpected soft error: %+v", res.StructuredContent)
	}
	stale, ok := res.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("outline_stale: StructuredContent is %T, want map[string]any", res.StructuredContent)
	}
	neverRecorded, _ := stale["never_recorded"].([]any)
	if len(neverRecorded) != 1 || neverRecorded[0] != "section_01" {
		t.Errorf("outline_stale never_recorded = %v, want [section_01]", stale["never_recorded"])
	}
	currentHashes, _ := stale["current_hashes"].(map[string]any)
	hash, _ := currentHashes["section_01"].(string)
	if hash == "" {
		t.Fatalf("outline_stale current_hashes has no section_01 entry: %+v", stale)
	}

	// Record the section directly (bypassing the outliner agent, which
	// this test isn't exercising) using the hash outline_stale reported,
	// then join and render through the server.
	if err := os.MkdirAll(cli.OutlineSectionsDir(dir), 0o755); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(sectionText))
	if hex.EncodeToString(sum[:]) != hash {
		t.Fatalf("computed hash %s != outline_stale's reported hash %s", hex.EncodeToString(sum[:]), hash)
	}
	sectionRecord, err := json.Marshal(map[string]any{
		"section":        "section_01",
		"section_sha256": hash,
		"scenes":         []any{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cli.OutlineSectionsDir(dir), "section_01.json"), sectionRecord, 0o644); err != nil {
		t.Fatal(err)
	}

	res, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "outline_join",
		Arguments: map[string]any{"manuscript_dir": dir},
	})
	if err != nil {
		t.Fatalf("outline_join: protocol error: %v", err)
	}
	if res.IsError {
		t.Fatalf("outline_join: unexpected soft error: %+v", res.StructuredContent)
	}

	// outline_render is the tool whose whole value is the file it writes.
	// This is the call that depended on domainsDir threading -- see the
	// doc comment above.
	res, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "outline_render",
		Arguments: map[string]any{"manuscript_dir": dir},
	})
	if err != nil {
		t.Fatalf("outline_render: protocol error: %v", err)
	}
	if res.IsError {
		t.Fatalf("outline_render: unexpected soft error (likely a domain-lookup regression): %+v", res.StructuredContent)
	}
	renderedPath := filepath.Join(dir, "Outline.md")
	body, err := os.ReadFile(renderedPath)
	if err != nil {
		t.Fatalf("outline_render did not write %s: %v", renderedPath, err)
	}
	if !strings.Contains(string(body), "section_01") {
		t.Errorf("Outline.md does not mention section_01:\n%s", body)
	}
}
