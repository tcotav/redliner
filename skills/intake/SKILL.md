---
name: intake
description: Interviews the author about a manuscript's intent, context, and deliberate choices (fields depend on the manuscript's domain), then writes the brief that redliner's editing passes read. Use before the first redliner run on a manuscript, or when the author wants to revise the brief.
---

# redliner:intake

Produces `<manuscript_dir>/.redliner/brief.md` — the context every editing
pass reads before forming an opinion.

This exists because **an editor who doesn't know what you were trying to
do will report your choices as your mistakes.** Sentence fragments,
present tense, a deliberately unreliable narrator, a dialect voice, a
slow literary open — all read as defects without intent. Genre also
changes severity outright: a 40-word sentence is a problem in a thriller
and unremarkable in literary fiction.

This interview is **domain-driven, not fiction-specific**: what to ask,
and what draft stages mean for severity, come from the manuscript's
active domain config, not from this file. Listing available domains and
showing one domain's full config are how you read that config — use
whichever concrete tool this session offers for those two operations
(bare `redliner domain list`/`redliner domain show <name>` commands on
the CLI variant, or the matching MCP tool on the Cowork/MCP variant); don't
hardcode fiction's fields here even as a fallback.

## Steps

1. **Determine the domain.** Check the manuscript's current state.
   - If state already exists, use its `domain` field — you're revising an
     existing brief for an already-chosen domain, don't re-ask.
   - If no state exists, list the available domains. If there's exactly
     one, use it without asking. If more than one, show the author each
     domain's `display_name`/`description` via AskUserQuestion and let
     them pick. Never guess.

2. **Set up state.** Initialize state for this manuscript in the chosen
   domain — if state already exists, that's fine, it'll say so and
   you're revising an existing brief.

3. **Load the domain config.** Look up the full config for the chosen
   domain and read its `brief_fields` and `draft_stages` — these drive
   everything below. Don't ask about anything not listed in
   `brief_fields`, and use `draft_stages`' `name` values verbatim as the
   draft-stage options.

4. **Read a sample first, then interview.** Read the first section (and
   skim one from the middle) *before* asking anything. Ask questions
   informed by what's actually on the page — "you're in present tense
   throughout, is that fixed?" beats "what tense is it?".

   **But only infer the fields the text can actually settle.** Split
   `brief_fields` in two before you start:

   - **Observable** — POV, tense, length, structure, cadence evidence.
     The text really does answer these. Infer them, and skip the
     question.
   - **Intent** — genre/subgenre, audience, comps, hook expectation, and
     draft stage. These are the author's claim about *which conventions
     apply*, not facts on the page. **Always ask these, however obvious
     the manuscript makes them seem.**

   The failure mode is specific, and it was observed in real use: an
   interviewer read a genre confidently off the page, wrote it into the
   brief without asking, and the author's actual answer named a different
   genre with different conventions. Genre is the worst case because it
   *gates severity* — the same passage is a defect under one genre's
   conventions and correct craft under another's — so a wrong guess
   mis-calibrates every later pass silently, with nothing downstream able
   to catch it. **A confident wrong genre is worse than an unasked
   question**, because the author never sees the assumption to correct
   it.

   **Anything you infer, show back for confirmation** rather than writing
   it into the brief unannounced, and mark it in the brief as inferred
   ("read from the manuscript, not supplied by the author") so a later
   reader knows which fields carry the author's authority and which carry
   yours.

5. **Interview.** Use the AskUserQuestion tool where the options are
   genuinely enumerable (draft stage, and any `brief_fields` entry with an
   obvious small set of answers), and plain conversation where they
   aren't (open-ended fields, what they're worried about). Cover:

   - **Every field in the domain's `brief_fields`** — ask using that
     field's `prompt` text.
   - **"None" is a real answer to some fields — take it and move on.**
     Comps especially: an author writing toward something original may
     have no comps by choice, and pressing for them implies the work
     should resemble something. Don't infer comps from the text to fill
     the gap. Record the refusal *as an instruction* — "none supplied,
     deliberately; do not infer comps and calibrate against them" — so a
     later pass doesn't quietly substitute its own. Comps exist for
     severity calibration (a 40-word sentence is a defect in a thriller
     and unremarkable in literary fantasy), so when they're absent, the
     genre/subgenre field carries that load alone. That is a reason to
     ask genre *carefully*, not a reason to push on comps.
   - **Draft stage** — present the domain's `draft_stages` names as the
     options. This *gates severity*, see below. Stage vocabulary and its
     severity implication are domain data, not something to hardcode
     here — a design doc's stages aren't a novel's.
   - **Deliberate choices** — the critical field, and it isn't fiction-
     specific: a design doc might deliberately leave a section
     unresolved, use fragments in an executive summary, or assume
     context this draft doesn't restate. Ask directly: "what would look
     like a mistake but isn't?"
   - **Known problem areas** — what the author already knows is broken;
     no value in reporting it back as news
   - **Off-limits** — anything they don't want touched
   - **What they want from this pass** — specific worries beat "make it good"

6. **Write the brief** to `<manuscript_dir>/.redliner/brief.md` using
   `reference/brief_template.md` in this skill directory as the structure
   — it's written generically, looping over whatever `brief_fields` and
   `draft_stages` the domain supplied. Write what the author actually
   said, in their framing — don't editorialize their intent into
   something tidier.

7. **Confirm.** Show the brief and ask whether it reflects their intent.
   Fix what's wrong. Then tell them the next step is `/redliner:run assess`.

## Draft stage gates severity

Copy the matching `draft_stages` entry's `implication` into the brief
verbatim (see the template) — every domain defines its own table via
`domain.json`'s `draft_stages`, because what a stage implies for severity
is domain calibration, not something this skill should know on its own.
The reason a stage-gate exists at all is universal, though: reporting
`minor` nits on an early draft is noise that buries the structural notes
that actually matter at that stage, whatever the domain.
