package mcpserver

// Tool descriptions, copied verbatim from cowork/mcp_server.py's
// docstrings -- byte-for-byte, not paraphrased. TODO.md's "Known porting
// hazards" section calls this out explicitly: the Cowork spike's
// load-bearing result was Claude picking all three correct tools
// *unprompted*, off these exact descriptions, and a terser Go-native
// rewrite would degrade tool selection in a way nothing else in this
// port would catch. Each one still says "Mirrors `redliner_state.py
// ...`" even though this Go binary's actual CLI syntax is now `redliner
// state ...` (see TODO.md's "v1 plan") -- that stale-looking reference
// is intentional, not a bug: it's part of the frozen text, and the
// underlying behavior it describes is unchanged either way.

const descStateInit = `Initialize redliner state for a manuscript directory, in the given
domain (defaults to "fiction"). Fails if state already exists, or if
the domain name doesn't match a real domain config. Mirrors
` + "`redliner_state.py init <manuscript_dir> [domain]`" + `.`

const descStateStatus = `Report a manuscript's current redliner state (domain, phase, round,
section fingerprints) as JSON. Mirrors ` + "`redliner_state.py status\n<manuscript_dir>`" + `.`

const descStateDiff = `Compare the manuscript's text on disk against the last assessed
snapshot; returns a verdict (unchanged/targeted/restructured) plus
which sections changed. Mirrors ` + "`redliner_state.py diff\n<manuscript_dir>`" + `.`

const descStateSnapshot = `Record the manuscript's current text as the assessed baseline, so a
later state_diff can tell what changed. Mirrors ` + "`redliner_state.py\nsnapshot <manuscript_dir>`" + `.`

const descStatePhase = `Move a manuscript to a new phase (intake/developmental/line/
complete). Entering the domain's round-tracked phase from elsewhere
increments the round counter automatically. Mirrors ` + "`redliner_state.py\nphase <manuscript_dir> <phase>`" + `.`

const descCanonStale = `Report which sections need (re-)extraction for the continuity layer
-- never extracted, or changed since their facts were extracted --
along with each such section's current hash and any orphaned
observation files. Mirrors ` + "`redliner_canon.py stale <manuscript_dir>`" + `.`

const descCanonReconcile = `Rebuild the merged canon and find continuity collisions from every
current observations file. Writes canon.json and collisions.json to
the manuscript's .redliner/canon/ directory (same side effect as the
CLI) and returns their contents. Mirrors ` + "`redliner_canon.py reconcile\n<manuscript_dir>`" + `.

Pass snapshot_after: true when this call is part of an assess or recheck
flow. It records the current text as the assessed baseline in the same
call, which is what keeps likely_unpropagated_revision working: that flag
is computed by diffing against the baseline currently in state, and a
separate snapshot overwrites exactly that baseline. Doing them as two
tool calls means one order works and the other silently disables the
flag. Omit it for a standalone continuity run, which records no baseline.`

const descDomainList = `List every domain config available (name, display name,
description). Mirrors ` + "`redliner_domain.py list`" + `.`

const descDomainShow = `Show the full config for one named domain -- categories, continuity
vocabulary, brief fields, draft stages. Mirrors ` + "`redliner_domain.py\nshow <name>`" + `.`

const descValidateFindings = `Validate everything currently under a manuscript's .redliner/
directory (canon observations, continuity, developmental/line
findings, editorial letter) against its domain's schema, including
excerpt-verbatim checks against the actual section text. Mirrors
` + "`validate_findings.py <manuscript_dir>`" + ` -- same pass/fail logic and
per-file detail, captured from its stdout rather than re-derived.`

// descContext has no mcp_server.py counterpart: `context` is a Go-only
// composite added to cut coordinator round trips, not a ported operation.
// Written here rather than extracted from a Python docstring, and
// deliberately says what it replaces so a caller reaches for it first.
const descContext = `Orientation for a manuscript in a single call: current state (phase,
round, domain), the active domain's full config (categories, brief
fields, draft stages), the section list, the diff verdict against the
last snapshot, continuity staleness, and which .redliner files exist.

Prefer this over calling state_status, domain_show, state_diff and
canon_stale separately -- it returns all of them at once, and each
avoided call is a full round trip.`

// --- Go-only tools ---
//
// These have no mcp_server.py counterpart: they expose CLI subcommands
// added after the Python implementation stopped being the reference. They
// exist because the Cowork variant shares skills/ with the CLI variant by
// symlink, so a command the skill invokes with no tool behind it leaves
// Cowork unable to finish a run -- which is exactly what happened between
// v0.4.0 and v0.5.0. See server_test.go's front-door parity guard.

const descDecisionsApply = `Re-apply the author's recorded resolutions to the findings files, restoring any a pass overwrote. Returns counts and the ids of decisions whose finding no longer exists.`

const descRoundsArchive = `Archive a completed pass's findings under .redliner/rounds/ so the next round has a "before" to diff against. Pass kind: developmental, line, continuity, or outline.`

const descRoundsList = `List the archived rounds under .redliner/rounds/.`

const descStateStage = `Record the manuscript's draft stage, which gates how severely findings are reported and whether line editing runs at all.`

const descStatePass = `Record that a pass of the given kind (developmental, line, continuity) completed, so status can report what has actually been run rather than only the current phase.`

const descStatePublished = `Record which installment the serial has shipped through, or "none" to clear it. Everything up to and including that section renders as published in Outline.md, where it reads as no longer movable or cuttable.`

const descCanonBundle = `Return every extracted fact as one compact line, "id | entity | attribute | value", for the continuity joiner to read in a single call. Deliberately omits excerpts and metadata: measured at 86 bytes per fact against 267 for the full JSON, with no loss of join accuracy.`

const descCanonMerge = `Fold the continuity joiner's findings (canon/joined.json) into canon/continuity.json, renumbering the joiner's ids into the cont-5NN range. Deduplicates on the set of facts cited, so re-running after a re-join adds only what is new.`

const descOutlineStale = `Which sections need (re-)recording for the outline, plus their current SHA-256 hashes and any orphaned outline files whose section no longer exists. Cheap and deterministic — call it before recording, and skip any section it does not list.`

const descOutlineJoin = `Rebuild .redliner/outline/outline.json from every current per-section outline file, in manuscript order. Deterministic; no model call.`

const descOutlineRender = `Write the author-readable Outline.md into the manuscript directory (not .redliner/), rendering every recorded scene and the published boundary. Deterministic; no model call.`

const descOutlineArchive = `Save the current joined outline as a new numbered version under .redliner/outline/versions/, so a later question like "what did this look like two chapters ago" has an answer. Pass changed_sections as the stems re-recorded this run, or omit it -- an empty list is valid and still checked, since a run that only deleted an orphaned section still changed the joined outline. A no-op, reported as such rather than as an error, when the joined outline is unchanged since the last archived version: call it after every render without worrying about creating noise.`

const descOutlineVersions = `List the archived outline versions under .redliner/outline/versions/, newest counter last, with the date, what changed, and the path to each version's readable Outline.md.`
