package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tcotav/redliner/go/internal/buildinfo"
)

// Through v0.7.0 the binary could not report its own version: `redliner
// --version` printed "Unknown subcommand '--version'", and the only
// signal was a sidecar file the installer wrote about its own intent.
// These pin the subcommand and both flag spellings.
func TestVersionSubcommand(t *testing.T) {
	for _, arg := range []string{"version", "--version", "-v"} {
		t.Run(arg, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := Dispatch([]string{arg}, &stdout, &stderr); code != 0 {
				t.Fatalf("%s exited %d, want 0 (stdout=%q)", arg, code, stdout.String())
			}
			got := strings.TrimSpace(stdout.String())
			if got != buildinfo.Version {
				t.Errorf("%s printed %q, want %q", arg, got, buildinfo.Version)
			}
		})
	}
}

// An unstamped build must say "dev" rather than claim a release number
// it can't support. `go test` never passes -ldflags, so this asserts the
// default the package ships with.
func TestVersionDefaultsToDev(t *testing.T) {
	if buildinfo.Version != "dev" {
		t.Errorf("buildinfo.Version = %q in an unstamped build, want %q", buildinfo.Version, "dev")
	}
}

// The usage text is what a user sees after a typo, so it has to list the
// subcommand that answers "what version is this?".
func TestUsageMentionsVersion(t *testing.T) {
	if !strings.Contains(usage, "redliner version") {
		t.Errorf("usage does not mention `redliner version`:\n%s", usage)
	}
}
