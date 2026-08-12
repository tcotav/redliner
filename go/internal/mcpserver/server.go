// Package mcpserver is the Go port of cowork/mcp_server.py: the same 10
// deterministic operations exposed as MCP tools instead of CLI
// subcommands, for plugin hosts (Cowork) that reject a top-level bin/
// directory on PATH. See TODO.md's "Cowork support via an MCP server
// variant" and "Port to a compiled language" sections for why this
// front door exists and how it's meant to converge with the CLI.
//
// Tool names and descriptions are carried over from mcp_server.py
// verbatim (descriptions.go) -- this is not a reimplementation with a
// similar shape, it is the same 10 operations, mostly calling straight
// into internal/schemas the same way mcp_server.py calls straight into
// schemas/*.py, bypassing internal/cli's printed-output layer entirely
// for the 8 tools that don't need it. canon_reconcile and
// validate_findings are the two exceptions, same as in Python: they
// reuse internal/cli's logic directly (ComputeReconcile/WriteCanonFiles,
// RunValidate) rather than re-deriving it a second way.
package mcpserver

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/tcotav/redliner/go/internal/cli"
	"github.com/tcotav/redliner/go/internal/schemas"
)

// NewServer builds the redliner MCP server. domainsDir must already be
// resolved (schemas.FindDomainsDir()) by the caller -- this package does
// no path resolution of its own, so it's trivial to point at a fixed
// directory in tests.
func NewServer(domainsDir string) *mcp.Server {
	s := &redlinerServer{domainsDir: domainsDir}
	srv := mcp.NewServer(&mcp.Implementation{Name: "redliner", Version: "0.1.0"}, nil)

	mcp.AddTool(srv, &mcp.Tool{Name: "state_init", Description: descStateInit}, s.stateInit)
	mcp.AddTool(srv, &mcp.Tool{Name: "state_status", Description: descStateStatus}, s.stateStatus)
	mcp.AddTool(srv, &mcp.Tool{Name: "state_diff", Description: descStateDiff}, s.stateDiff)
	mcp.AddTool(srv, &mcp.Tool{Name: "state_snapshot", Description: descStateSnapshot}, s.stateSnapshot)
	mcp.AddTool(srv, &mcp.Tool{Name: "state_phase", Description: descStatePhase}, s.statePhase)
	mcp.AddTool(srv, &mcp.Tool{Name: "canon_stale", Description: descCanonStale}, s.canonStale)
	mcp.AddTool(srv, &mcp.Tool{Name: "canon_reconcile", Description: descCanonReconcile}, s.canonReconcile)
	mcp.AddTool(srv, &mcp.Tool{Name: "domain_list", Description: descDomainList}, s.domainList)
	mcp.AddTool(srv, &mcp.Tool{Name: "domain_show", Description: descDomainShow}, s.domainShow)
	mcp.AddTool(srv, &mcp.Tool{Name: "validate_findings", Description: descValidateFindings}, s.validateFindings)

	return srv
}

type redlinerServer struct {
	domainsDir string
}

// --- input shapes -- field names match mcp_server.py's function
// parameter names 1:1, since those are what the MCP schema exposes to
// the calling model. ---

type manuscriptDirInput struct {
	ManuscriptDir string `json:"manuscript_dir"`
}

type stateInitInput struct {
	ManuscriptDir string `json:"manuscript_dir"`
	Domain        string `json:"domain,omitempty"`
}

type statePhaseInput struct {
	ManuscriptDir string `json:"manuscript_dir"`
	Phase         string `json:"phase"`
}

type domainShowInput struct {
	Name string `json:"name"`
}

type noInput struct{}

// errorResult mirrors every mcp_server.py tool's {"error": "..."}
// return shape -- a *soft* failure the tool call still succeeds at the
// protocol level for (IsError unset), same as Python returning a dict
// instead of raising. This is deliberate, not incidental: the Cowork
// spike's tool-selection behavior depended on the model being able to
// read an error message back as normal tool output, not on catching an
// MCP protocol-level error.
func errorResult(format string, args ...any) map[string]any {
	return map[string]any{"error": fmt.Sprintf(format, args...)}
}

func pyReprStr(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "\\'") + "'"
}

// --- state_* -- mirrors mcp_server.py's state_* tools, which import
// schemas.project_state/domain_loader directly rather than calling
// redliner_state.py's cmd_* functions (those only print). ---

func (s *redlinerServer) stateInit(_ context.Context, _ *mcp.CallToolRequest, in stateInitInput) (*mcp.CallToolResult, any, error) {
	domain := in.Domain
	if domain == "" {
		domain = schemas.DefaultDomain
	}
	if existing, err := schemas.LoadState(in.ManuscriptDir); err == nil && existing != nil {
		return nil, errorResult("State already exists at %s", schemas.StatePath(in.ManuscriptDir)), nil
	}
	if _, err := schemas.LoadDomain(s.domainsDir, domain); err != nil {
		return nil, errorResult("Domain config error: %v", err), nil
	}
	state := schemas.NewState(in.ManuscriptDir, domain)
	path, err := schemas.SaveState(in.ManuscriptDir, state)
	if err != nil {
		return nil, nil, err
	}
	return nil, map[string]any{
		"status": "initialized",
		"path":   path,
		"domain": domain,
		"phase":  "intake",
	}, nil
}

func (s *redlinerServer) stateStatus(_ context.Context, _ *mcp.CallToolRequest, in manuscriptDirInput) (*mcp.CallToolResult, any, error) {
	state, err := schemas.LoadState(in.ManuscriptDir)
	if err != nil {
		return nil, nil, err
	}
	if state == nil {
		return nil, errorResult("No redliner state in %s. Call state_init first.", in.ManuscriptDir), nil
	}
	return nil, state, nil
}

func (s *redlinerServer) stateDiff(_ context.Context, _ *mcp.CallToolRequest, in manuscriptDirInput) (*mcp.CallToolResult, any, error) {
	state, err := schemas.LoadState(in.ManuscriptDir)
	if err != nil {
		return nil, nil, err
	}
	if state == nil {
		return nil, errorResult("No redliner state in %s", in.ManuscriptDir), nil
	}
	diff, err := schemas.DiffManuscript(in.ManuscriptDir, state)
	if err != nil {
		if ce, ok := err.(*schemas.SectionCollisionError); ok {
			return nil, errorResult("Section file error: %s", ce.Error()), nil
		}
		return nil, nil, err
	}
	return nil, diff, nil
}

func (s *redlinerServer) stateSnapshot(_ context.Context, _ *mcp.CallToolRequest, in manuscriptDirInput) (*mcp.CallToolResult, any, error) {
	state, err := schemas.LoadState(in.ManuscriptDir)
	if err != nil {
		return nil, nil, err
	}
	if state == nil {
		return nil, errorResult("No redliner state in %s", in.ManuscriptDir), nil
	}
	fingerprints, err := schemas.FingerprintManuscript(in.ManuscriptDir)
	if err != nil {
		if ce, ok := err.(*schemas.SectionCollisionError); ok {
			return nil, errorResult("Section file error: %s", ce.Error()), nil
		}
		return nil, nil, err
	}
	state.SectionFingerprints = fingerprints
	if _, err := schemas.SaveState(in.ManuscriptDir, state); err != nil {
		return nil, nil, err
	}
	return nil, map[string]any{
		"status":        "snapshotted",
		"section_count": len(fingerprints),
	}, nil
}

func (s *redlinerServer) statePhase(_ context.Context, _ *mcp.CallToolRequest, in statePhaseInput) (*mcp.CallToolResult, any, error) {
	if !schemas.IsValidPhase(in.Phase) {
		return nil, errorResult("Unknown phase %s. Must be one of: %s", pyReprStr(in.Phase), strings.Join(schemas.Phases, ", ")), nil
	}
	state, err := schemas.LoadState(in.ManuscriptDir)
	if err != nil {
		return nil, nil, err
	}
	if state == nil {
		return nil, errorResult("No redliner state in %s", in.ManuscriptDir), nil
	}

	domain, err := schemas.LoadDomain(s.domainsDir, state.DomainName())
	if err != nil {
		return nil, errorResult("Domain config error: %v", err), nil
	}
	roundTrackedPhase := domain.RoundTrackedPhase()

	previous := state.Phase
	state.Phase = in.Phase
	if in.Phase == roundTrackedPhase && previous != roundTrackedPhase {
		state.DevelopmentalRound++
	}
	if _, err := schemas.SaveState(in.ManuscriptDir, state); err != nil {
		return nil, nil, err
	}
	return nil, map[string]any{
		"previous_phase":      previous,
		"phase":               in.Phase,
		"developmental_round": state.DevelopmentalRound,
	}, nil
}

// --- canon_* -- mirrors mcp_server.py's canon_* tools. canon_stale
// reimplements redliner_canon.py's stale computation the same way
// Python's tool does (against the shared internal/cli.ComputeStale
// rather than a second copy); canon_reconcile calls the same
// computation the CLI's `canon reconcile` uses. ---

func (s *redlinerServer) canonStale(_ context.Context, _ *mcp.CallToolRequest, in manuscriptDirInput) (*mcp.CallToolResult, any, error) {
	result, err := cli.ComputeStale(in.ManuscriptDir)
	if err != nil {
		if ce, ok := err.(*schemas.SectionCollisionError); ok {
			return nil, errorResult("Section file error: %s", ce.Error()), nil
		}
		// Matches mcp_server.py's canon_stale: load_observations errors
		// are not caught there either and propagate as a real failure,
		// not a returned {"error": ...} dict.
		return nil, nil, err
	}
	return nil, result, nil
}

func (s *redlinerServer) canonReconcile(_ context.Context, _ *mcp.CallToolRequest, in manuscriptDirInput) (*mcp.CallToolResult, any, error) {
	canon, collisions, err := cli.ComputeReconcile(in.ManuscriptDir)
	if err != nil {
		switch {
		case err == cli.ErrNoObservations:
			return nil, errorResult("No observations in %s. Run extraction first.", cli.ObservationsDir(in.ManuscriptDir)), nil
		default:
			if ce, ok := err.(*schemas.SectionCollisionError); ok {
				return nil, errorResult("Section file error: %s", ce.Error()), nil
			}
			return nil, nil, err
		}
	}
	if err := cli.WriteCanonFiles(in.ManuscriptDir, canon, collisions); err != nil {
		return nil, nil, err
	}
	return nil, map[string]any{
		"canon":      canon,
		"collisions": cli.OrEmptyCollisions(collisions),
	}, nil
}

// --- domain_* -- mirrors mcp_server.py's domain_* tools, which import
// schemas.domain_loader directly (not redliner_domain.py's cmd_*
// functions). Unlike the CLI's `domain list`, a malformed domain config
// is skipped silently here with no stderr note either -- matches
// Python's mcp_server.py exactly, which has no equivalent of the CLI's
// warning print for this tool. ---

func (s *redlinerServer) domainList(_ context.Context, _ *mcp.CallToolRequest, _ noInput) (*mcp.CallToolResult, any, error) {
	names := schemas.ListDomains(s.domainsDir)
	summaries := make([]cli.DomainSummary, 0, len(names))
	for _, name := range names {
		d, err := schemas.LoadDomain(s.domainsDir, name)
		if err != nil {
			continue
		}
		summaries = append(summaries, cli.DomainSummary{
			Name:        d.String("name"),
			DisplayName: d.String("display_name"),
			Description: d.String("description"),
		})
	}
	return nil, summaries, nil
}

func (s *redlinerServer) domainShow(_ context.Context, _ *mcp.CallToolRequest, in domainShowInput) (*mcp.CallToolResult, any, error) {
	d, err := schemas.LoadDomain(s.domainsDir, in.Name)
	if err != nil {
		return nil, errorResult("Domain config error: %v", err), nil
	}
	return nil, d, nil
}

// --- validate_findings -- mirrors mcp_server.py's validate_findings
// tool exactly: reuses internal/cli.RunValidate (the same logic the CLI's
// `redliner validate` uses), capturing its stdout instead of re-deriving
// the pass/fail logic and per-file detail a second way. ---

func (s *redlinerServer) validateFindings(_ context.Context, _ *mcp.CallToolRequest, in manuscriptDirInput) (*mcp.CallToolResult, any, error) {
	var buf bytes.Buffer
	// ValidateManuscript, not RunValidate: takes s.domainsDir directly
	// instead of re-resolving it via schemas.FindDomainsDir(), which
	// would search from this process's own binary rather than reusing
	// the directory this server was constructed with. See
	// internal/cli/validate.go's ValidateManuscript doc comment.
	exitCode := cli.ValidateManuscript(in.ManuscriptDir, s.domainsDir, &buf)
	return nil, map[string]any{
		"ok":     exitCode == 0,
		"output": buf.String(),
	}, nil
}
