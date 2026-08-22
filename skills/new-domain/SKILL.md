---
name: new-domain
description: Walks someone through designing a new redliner domain (a kind of document to edit, beyond fiction) — category vocabulary for both editing phases, the continuity layer's entity types/sources/categories, brief fields, and draft stages — then generates that domain's domain.json and its agent files. Use when the author wants redliner to work on something that isn't fiction (a design doc, product proposal, or any other long-form document) and no suitable domain exists yet.
---

# redliner:new-domain

Produces `domains/<name>/domain.json` and its agent files
(`agents/<name>-developmental-editor.md`, `<name>-line-editor.md`,
`<name>-editorial-aggregator.md`, `<name>-continuity-extractor.md`,
`<name>-continuity-adjudicator.md`, `<name>-continuity-joiner.md`, and
`<name>-outliner.md` when this domain configures an outline layer),
following the same shape as `domains/fiction/domain.json` and
`agents/fiction-*.md`.

Read `domains/fiction/domain.json` and skim `agents/fiction-*.md` before
starting — they're the worked example throughout this skill, not
optional background.

## Why this generates static files instead of a generic prompt

A domain is a **template that generates concrete files**, regenerated
when the domain changes, hand-editable after. The alternative — generic
agent prompts that read category vocabulary out of `domain.json` at
runtime — was considered and rejected: it would hollow out real prompt
craft (see `fiction-developmental-editor.md`'s handling of
`deferred_to_line`) into generic text, and it would concentrate every
future bug in this skill's generation step instead of in a reviewable,
diffable agent file. Don't reintroduce runtime injection to "simplify"
this.

## Step 1: Name and frame the domain

Interview conversationally, not as a rigid form:

- **`name`** — kebab-case, short (`design-doc`, not
  `design-document-review-domain`). List the existing domains first and
  refuse a name that collides.
- **`display_name`** — human-readable (`"Design Doc / Product Proposal"`).
- **`description`** — one sentence: what kind of document, what makes it
  a document at all (fiction: "characters, plot, and a fictional world";
  a design doc: "a stated problem, a proposed approach, and consequences
  for people other than the author").

`round_tracked_phase` stays `"developmental"` and `unit_name` stays
`"section"` — **do not make these domain-configurable.** Phase names are
fixed project-wide (see `TODO.md`); `unit_name` is currently descriptive
only — no code reads it yet, the `section_<NNN>` naming convention is
still hardcoded in `go/internal/schemas/project_state.go` (the file *extension*
is flexible — `.txt` or `.md` — but the `section_` stem prefix isn't) —
so setting `unit_name` to anything but `"section"` would promise
something the plumbing doesn't deliver yet. Don't ask the author about
either.

## Step 2: Design the two category vocabularies

This is the hardest interview in this skill, because a bad category list
silently degrades every finding downstream forever, and it won't look
broken — it'll just quietly produce vague or overlapping findings. Hold
to these rules for **both** `developmental_categories` and
`line_categories`:

- **4–7 categories per phase.** Fewer than 4 usually means two real
  categories got merged; more than 7 usually means severity is leaking
  into category (see below), or the list is really two domains.
- **Each category must be something a reviewer could plausibly disagree
  about being present.** "Is this a `pacing` problem?" is a real
  question with a real wrong answer. A category nobody could contest
  ("has_text", "is_readable") isn't doing work.
- **No category that's really a severity in disguise.** Don't allow both
  a `minor_issue` category and a `severity` field — that's the same axis
  twice. If two proposed categories differ only in how bad the problem
  is rather than in *what kind* of problem it is, merge them and let
  `severity` carry the difference.
- **Developmental categories are whole-document; line categories are
  local to one section.** Fiction's test: `plot` needs the whole
  manuscript to judge; `word_choice` needs one sentence. Apply the same
  split here — if a candidate developmental category can be judged from
  one section alone, it's probably a line category instead.

Draft the list yourself from the domain description, explain your
reasoning against each rule above, and confirm with the author rather
than asking them to invent categories from nothing — they know the
document, you know what makes a category well-formed.

## Step 3: Design the continuity layer

Three lists, and they interlock — work through them in this order,
using `domains/fiction/domain.json`'s `continuity` block as the pattern:

1. **`entity_types`** — the kinds of things this document makes checkable
   assertions about. Fiction: `character`, `place`, `object`,
   `organization`, `event`, `world_rule`. Ask: what nouns in this
   document get a specific, restatable fact attached to them?
2. **`sources`** — *where* in the document an assertion can come from,
   distinguished by how much authority/reliability it carries. Fiction
   distinguishes `narration` (authorial, trustworthy) from `dialogue`/
   `character_thought` (a character can be wrong or lying). Ask: does
   this document have a section that's a deliberate simplification of
   another (an executive summary vs. a body, an abstract vs. detail) —
   that's the same shape as a character's imprecise framing, and it's
   why `sources` exists rather than treating every assertion as equally
   authoritative.
3. **`categories`** — what *kind* of contradiction a collision between
   two facts represents. Fiction: `character_attribute`, `timeline`,
   `geography`, `world_rule`, `naming`, `relationship`, `object`. These
   follow fairly mechanically once `entity_types` is set — usually close
   to one category per entity type plus `timeline` and `naming`, which
   apply across all of them.

Sanity-check the whole design against a concrete case before moving on:
restate "the summary says X, a detail section says Y" using this
domain's actual vocabulary. If it doesn't fit cleanly, revise the lists,
don't contort the example.

## Step 4: Design brief fields and draft stages

- **`brief_fields`** — what does `/redliner:intake` need to ask before any
  pass can judge this document fairly? Each entry needs `name` (snake_
  case), `label` (for the brief template), and `prompt` (the actual
  question). Fiction has 9 (logline, release format, genre, audience,
  comps, length, POV, tense, structure) — there's no fixed count here,
  just "what does an editor need to know that isn't on the page."
  Two recurring patterns worth reusing rather than reinventing:
  - **A closure/format field** (fiction's "release format": standalone
    vs. series vs. serialized) — any domain where a unit's ending gets
    judged for completeness needs to know whether incompleteness is
    intentional at that scale. This exists because it was missed once:
    the developmental editor flagged a serialized manuscript's
    intentionally-open chapter endings as structural gaps, because
    nothing had told it chapters weren't meant to resolve individually.
  - **An optional required-structure field** (design-doc's "required
    structure": paste your org's template if you have one) — paired
    with a `structure_compliance`-style developmental category that
    only fires when the field is non-empty. Don't have the category
    invent an expected structure when the author didn't give one.
- **`draft_stages`** — an ordered list of `{name, implication}`, where
  `implication` states what each stage means for finding severity.
  Fiction's four stages run exploratory → first-complete → revised →
  near-submission, tightening severity at each step. Design this
  domain's own stages; don't just relabel fiction's.

## Step 5: Write and validate `domain.json`

Before writing the file, ask whether this domain wants an outline
layer: a pass that records each unit's scenes as short structured rows
(fiction: goal/conflict/outcome) so an author can scan a whole
manuscript's shape without rereading it. Not every domain needs one —
skip it for a domain with no scene-like internal structure (design-doc
has none). If the author wants one, design `outline.row_fields`: each
entry is a recordable **fact** about a unit, never a rating — "what was
the driving intention" is a row field, "how well does this scene work"
is not, for the same reason the outliner's own prompt forbids opinions.
Three to four fields is the working range; more than that stops being
scannable. Optionally also design `outline.section_fields` for a fact
recorded once per unit rather than per scene (serial-fiction's
`leaves_open`) — most domains won't need one.

Write `domains/<name>/domain.json` matching
`domains/fiction/domain.json`'s exact key structure. Then look up the
full config for the new domain by name (whichever concrete tool this
session offers for that — a bare `redliner domain show <name>` command
on the CLI variant, or the matching MCP tool on the Cowork/MCP
variant).

This calls the same loader every pass uses — if it errors, the domain
config is malformed (missing keys, empty lists); fix it and re-check
until it comes back clean. Don't hand-verify the shape yourself and skip
this — the loader's validation is the actual contract, not a formality.

## Step 6: Generate the agent files

For each of the six core roles, use the matching template in
`reference/templates/<role>.md` in this skill directory. Each template
marks blocks `<!-- FIXED -->` (copy verbatim, substituting only
`{{...}}` placeholders that come straight from `domain.json` — no
creative judgment involved) and `<!-- AUTHORED -->` (needs real writing
for this domain).

Then generate a conditional seventh: **only** when this domain's
`domain.json` has an `outline` block (Step 3 or Step 5 will have asked
whether this domain wants one), generate `agents/<name>-outliner.md`
from `reference/templates/outliner.md`, following the same FIXED/
AUTHORED convention. A domain with no `outline` block gets no outliner
file — don't generate one "just in case."

For AUTHORED blocks: read the matching `agents/fiction-*.md` file first
as the quality bar, then write fresh prose for this domain — role
framing, scope boundaries, and a real worked example grounded in this
domain's own vocabulary. **A thin reskin (find/replace "manuscript" →
"document") is not acceptable** — if an AUTHORED block reads like it
could apply to any domain, it hasn't actually been written for this one.
The `description:` frontmatter line matters more than it looks: it's
what Claude Code's Task routing reads to pick the right agent, so it
needs to name this domain's actual scope and explicitly rule out the
sibling role, the way fiction's does.

Write each finished file to `agents/<name>-<role>.md`. **Both the
filename and the frontmatter `name:` field must read `<name>-<role>`,
exactly matching each other.** Strip **every** HTML comment before
writing — the leading template-header block *and* every inline
`<!-- FIXED -->` / `<!-- AUTHORED: ... -->` marker scattered through the
body. All of it is instructions for you, not part of the shipped prompt;
a leftover marker won't break registration or validation (Step 7's
checks wouldn't catch it) but it's generator scaffolding leaking into
what the model reads at inference time, and it should never ship.

## Step 7: Verify — don't just read the files back

This is not optional. A skill that generates five files and declares
success without checking whether they *work* is exactly the failure mode
that produced a real, previously-shipped bug in this project (renaming
`agents/developmental-editor.md` alone, without touching its frontmatter
`name:`, silently kept every reference resolving to the *old* unprefixed
agent id — the file looked renamed and wasn't).

1. **Frontmatter/filename consistency.** For each generated file —
   including `<name>-outliner.md` when this domain has an `outline`
   block — read its frontmatter `name:` and confirm it equals
   `<domain>-<role>` matching the filename stem exactly. Any mismatch:
   stop, fix, recheck.
2. **Domain config still valid.** Look up the new domain's full config
   again and confirm it still comes back clean.
3. **Live registration check.** For each generated agent — again
   including the outliner when this domain has one — use the Task tool
   to invoke `redliner:<name>-<role>` with a trivial prompt: reply with
   the single word `PONG` and nothing else. Confirm each one responds —
   an "Agent type not found" error means step 6's naming didn't actually
   take, whatever the file contents look like. Do this for every
   generated agent before reporting success on any.

Only after all three checks pass, tell the author the domain is ready.

## Step 8: Hand off

Summarize what was created: the category lists, the continuity design,
and one line per agent file on its role framing. Point the author at
`agents/<name>-*.md` and `domains/<name>/domain.json` directly — these
are plain files, hand-editable any time without re-running this skill.
Tell them the next step is `/redliner:intake` on a real document in this
domain, which is the real end-to-end test no synthetic check here can
substitute for.
