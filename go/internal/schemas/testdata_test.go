package schemas

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// repoRoot resolves paths relative to this test file rather than
// hardcoding an absolute path or relying on `go test`'s cwd (which is
// always the package directory, but the repo layout above it is what
// actually matters here: go/internal/schemas -> go/internal -> go ->
// repo root).
func repoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not determine test file location")
	}
	// go/internal/schemas/testdata_test.go -> go/internal/schemas -> go/internal -> go -> repo root
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
}

func fixturesDir(t *testing.T) string {
	return filepath.Join(repoRoot(t), "go", "harness", "fixtures")
}

func goldenDir(t *testing.T) string {
	return filepath.Join(repoRoot(t), "go", "harness", "golden")
}

func domainsDir(t *testing.T) string {
	return filepath.Join(repoRoot(t), "domains")
}

// loadGoldenJSON reads a capture_baseline.py step file and returns its
// stdout_json field, decoded generically -- this is deliberately reading
// the *actual* captured Python baseline rather than a value transcribed
// by hand, so a copy/paste transcription error can't silently make a
// test pass against the wrong number.
func loadGoldenJSON(t *testing.T, fixture, step string) map[string]interface{} {
	t.Helper()
	path := filepath.Join(goldenDir(t), fixture, step+".json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading golden %s: %v", path, err)
	}
	var entry struct {
		StdoutJSON map[string]interface{} `json:"stdout_json"`
	}
	if err := json.Unmarshal(raw, &entry); err != nil {
		t.Fatalf("parsing golden %s: %v", path, err)
	}
	if entry.StdoutJSON == nil {
		t.Fatalf("golden %s has no stdout_json", path)
	}
	return entry.StdoutJSON
}

func loadJSONFile(t *testing.T, path string) interface{} {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}
	return v
}
