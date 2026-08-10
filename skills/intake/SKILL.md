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
active domain config, not from this file. `redliner_domain.py list` and
`redliner_domain.py show <name>` (both bare commands, no `python3` prefix
— `bin/` is on PATH while the plugin is enabled) are how you read that
config; don't hardcode fiction's fields here even as a fallback.

## Steps

1. **Determine the domain.** Run `redliner_state.py status <manuscript_dir>`.
   - If state already exists, use its `domain` field — you're revising an
     existing brief for an already-chosen domain, don't re-ask.
   - If no state exists, run `redliner_domain.py list`. If it returns
     exactly one domain, use it without asking. If more than one, show
     the author each domain's `display_name`/`description` via
     AskUserQuestion and let them pick. Never guess.

2. **Set up state.** Run
   `redliner_state.py init <manuscript_dir> <domain>` — if state already
   exists, that's fine, it'll say so and you're revising an existing brief.

3. **Load the domain config.** Run `redliner_domain.py show <domain>` and
   read its `brief_fields` and `draft_stages` — these drive everything
   below. Don't ask about anything not listed in `brief_fields`, and use
   `draft_stages`' `name` values verbatim as the draft-stage options.

4. **Read a sample first, then interview.** Read the first section (and
   skim one from the middle) *before* asking anything. Ask questions
   informed by what's actually on the page — "you're in present tense
   throughout, is that fixed?" beats "what tense is it?". Never ask the
   author something the manuscript already answers.

5. **Interview.** Use the AskUserQuestion tool where the options are
   genuinely enumerable (draft stage, and any `brief_fields` entry with an
   obvious small set of answers), and plain conversation where they
   aren't (open-ended fields, what they're worried about). Cover:

   - **Every field in the domain's `brief_fields`** — ask using that
     field's `prompt` text.
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
