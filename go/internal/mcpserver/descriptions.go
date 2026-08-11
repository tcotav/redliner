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
CLI) and returns their contents. Mirrors ` + "`redliner_canon.py reconcile\n<manuscript_dir>`" + `.`

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
