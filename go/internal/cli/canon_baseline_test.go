package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tcotav/redliner/go/internal/schemas"
)

// These cover the one invariant `skills/run/SKILL.md` used to enforce
// with the sentence "Don't reorder this": reconcile has to read the
// snapshot baseline before `state snapshot` overwrites it, and getting
// it backwards silently disabled `likely_unpropagated_revision` with
// nothing in the output to say so.

// contradictorySections writes a manuscript whose two sections disagree
// about one attribute, plus the observations that record the disagreement.
func contradictorySections(t *testing.T, dir string) {
	t.Helper()
	sections := map[string]string{
		"section_01.md": "Her green eyes scanning the alleys, Mira ran.\n",
		"section_02.md": "Her blue eyes caught the lamplight.\n",
	}
	for name, body := range sections {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	fact := func(id, section, value string) map[string]interface{} {
		return map[string]interface{}{
			"id": id, "entity": "Mira", "entity_type": "character",
			"attribute": "eye_color", "value": value,
			"excerpt": "her " + value + " eyes", "location": "paragraph 1",
			"source": "narration", "confidence": "explicit",
		}
	}
	writeObservations(t, dir, "section_01", []map[string]interface{}{fact("fact-section_01-001", "section_01", "green")})
	writeObservations(t, dir, "section_02", []map[string]interface{}{fact("fact-section_02-001", "section_02", "blue")})
}

// initStateWithBaseline records a snapshot, then edits one section so the
// baseline genuinely differs from the current text -- the situation
// likely_unpropagated_revision exists to detect.
func initStateWithBaseline(t *testing.T, dir string) {
	t.Helper()
	fingerprints, err := schemas.FingerprintManuscript(dir)
	if err != nil {
		t.Fatal(err)
	}
	state := schemas.NewState(dir, "fiction")
	state.SectionFingerprints = fingerprints
	if _, err := schemas.SaveState(dir, state); err != nil {
		t.Fatal(err)
	}
	// Edit section_02 after the snapshot: now one of the two sections
	// carrying the contradiction is touched and the other isn't.
	body := "Her blue eyes caught the lamplight, and she turned away slowly.\n"
	if err := os.WriteFile(filepath.Join(dir, "section_02.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestComputeReconcile_FlagsUnpropagatedRevisionAgainstAPriorBaseline(t *testing.T) {
	dir := t.TempDir()
	contradictorySections(t, dir)
	initStateWithBaseline(t, dir)

	_, collisions, diagnostics, err := ComputeReconcile(dir, BaselineFromState(dir))
	if err != nil {
		t.Fatal(err)
	}
	if len(collisions) != 1 {
		t.Fatalf("want the eye_color collision, got %d", len(collisions))
	}
	if !collisions[0].LikelyUnpropagatedRevision {
		t.Errorf("one section edited since snapshot and one not -- want the flag set, got %+v", collisions[0])
	}
	if diagnostics.RevisionDetectionIdle() {
		t.Error("a baseline that differs from the text is not idle")
	}
	if len(diagnostics.ChangedSections) != 1 || diagnostics.ChangedSections[0] != "section_02" {
		t.Errorf("changed sections = %v, want [section_02]", diagnostics.ChangedSections)
	}
}

// The regression this whole change is about: snapshot first, and the
// baseline matches the text everywhere, so nothing can ever be flagged.
// The behaviour is unchanged -- what is new is that reconcile now says so.
func TestComputeReconcile_SnapshotFirstIsIdleAndSaysSo(t *testing.T) {
	dir := t.TempDir()
	contradictorySections(t, dir)
	initStateWithBaseline(t, dir)

	// Simulate the wrong order: take the snapshot before reconciling.
	var buf bytes.Buffer
	if code := cmdStateSnapshot(dir, &buf); code != 0 {
		t.Fatalf("snapshot: %s", buf.String())
	}

	_, collisions, diagnostics, err := ComputeReconcile(dir, BaselineFromState(dir))
	if err != nil {
		t.Fatal(err)
	}
	if len(collisions) != 1 {
		t.Fatalf("the collision itself must still be found, got %d", len(collisions))
	}
	if collisions[0].LikelyUnpropagatedRevision {
		t.Error("nothing differs from the baseline, so the flag cannot be set")
	}
	if !diagnostics.RevisionDetectionIdle() {
		t.Fatal("this is exactly the case the diagnostic exists to report")
	}

	// And the CLI has to surface it, on stderr, without touching the
	// stdout the Python oracle's goldens compare.
	var stdout, stderr bytes.Buffer
	if code := RunCanon([]string{"reconcile", dir}, &stdout, &stderr); code != 0 {
		t.Fatalf("reconcile: %s", stdout.String())
	}
	if strings.Contains(stdout.String(), "unpropagated revision this run") {
		t.Error("the note belongs on stderr -- stdout is compared against the Python oracle")
	}
	if !strings.Contains(stderr.String(), "no collision could be flagged as an unpropagated revision") {
		t.Errorf("stderr should explain why nothing was flagged, got: %q", stderr.String())
	}
}

// A first-ever assess has no baseline at all. Nothing could be
// unpropagated yet, so this must stay quiet -- a note here would fire on
// every manuscript's first run and train the reader to ignore it.
func TestComputeReconcile_NoBaselineIsSilentNotIdle(t *testing.T) {
	dir := t.TempDir()
	contradictorySections(t, dir)
	state := schemas.NewState(dir, "fiction")
	if _, err := schemas.SaveState(dir, state); err != nil {
		t.Fatal(err)
	}

	_, _, diagnostics, err := ComputeReconcile(dir, BaselineFromState(dir))
	if err != nil {
		t.Fatal(err)
	}
	if diagnostics.BaselineSections != 0 {
		t.Errorf("want no baseline, got %d sections", diagnostics.BaselineSections)
	}
	if diagnostics.RevisionDetectionIdle() {
		t.Error("no baseline is not the same as an idle one")
	}

	var stdout, stderr bytes.Buffer
	if code := RunCanon([]string{"reconcile", dir}, &stdout, &stderr); code != 0 {
		t.Fatalf("reconcile: %s", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("a first-ever reconcile must not warn, got: %q", stderr.String())
	}
}

// --snapshot-after is what removes the invariant: the two steps can no
// longer be written down in the wrong order because they are one call.
func TestCanonReconcile_SnapshotAfterFlagsThenRecordsTheBaseline(t *testing.T) {
	dir := t.TempDir()
	contradictorySections(t, dir)
	initStateWithBaseline(t, dir)

	var stdout, stderr bytes.Buffer
	if code := RunCanon([]string{"reconcile", dir, "--snapshot-after"}, &stdout, &stderr); code != 0 {
		t.Fatalf("reconcile --snapshot-after: %s", stdout.String())
	}

	// The flag was computed against the OLD baseline...
	raw, err := os.ReadFile(filepath.Join(CanonDir(dir), "collisions.json"))
	if err != nil {
		t.Fatal(err)
	}
	var written CollisionsFile
	if err := json.Unmarshal(raw, &written); err != nil {
		t.Fatal(err)
	}
	if len(written.Collisions) != 1 || !written.Collisions[0].LikelyUnpropagatedRevision {
		t.Fatalf("the flag must be computed before the snapshot lands: %+v", written.Collisions)
	}
	if !strings.Contains(stdout.String(), "Snapshotted 2 sections as assessed.") {
		t.Errorf("the snapshot should have run in the same call, got: %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("nothing idle about this run, got: %q", stderr.String())
	}

	// ...and the new baseline now matches the text, so an immediate
	// re-reconcile is the idle case.
	stdout.Reset()
	stderr.Reset()
	if code := RunCanon([]string{"reconcile", dir}, &stdout, &stderr); code != 0 {
		t.Fatalf("second reconcile: %s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "unpropagated revision this run") {
		t.Errorf("the snapshot landed, so the next run is idle: %q", stderr.String())
	}
}

func TestCanonReconcile_RejectsUnknownOptions(t *testing.T) {
	dir := t.TempDir()
	contradictorySections(t, dir)
	var stdout, stderr bytes.Buffer
	if code := RunCanon([]string{"reconcile", dir, "--snapshot"}, &stdout, &stderr); code == 0 {
		t.Error("a misspelled flag must not be silently ignored -- that is how the snapshot gets skipped")
	}
	if !strings.Contains(stdout.String(), "Unknown option") {
		t.Errorf("want an explicit rejection, got: %q", stdout.String())
	}
}
