---
name: intake
description: Interviews the author about a manuscript's intent, genre, and deliberate craft choices, then writes the manuscript brief that edaitor's editing passes read. Use before the first edaitor run on a manuscript, or when the author wants to revise the brief.
---

# edaitor:intake

Produces `<manuscript_dir>/.edaitor/brief.md` — the context every editing
pass reads before forming an opinion.

This exists because **an editor who doesn't know what you were trying to
do will report your choices as your mistakes.** Sentence fragments,
present tense, a deliberately unreliable narrator, a dialect voice, a
slow literary open — all read as defects without intent. Genre also
changes severity outright: a 40-word sentence is a problem in a thriller
and unremarkable in literary fiction.

## Steps

1. **Set up state.** Determine the manuscript directory (argument, or ask
   — defaults to the current directory). Run
   `edaitor_state.py init <manuscript_dir>` (no `python3` prefix; the
   plugin's `bin/` is on PATH while it's enabled) — if state already
   exists, that's fine, it'll say so and you're revising an existing brief.

2. **Read a sample first, then interview.** Read the first chapter (and
   skim one from the middle) *before* asking anything. Ask questions
   informed by what's actually on the page — "you're in present tense
   throughout, is that fixed?" beats "what tense is it?". Never ask the
   author something the manuscript already answers.

3. **Interview.** Use the AskUserQuestion tool where the options are
   genuinely enumerable (draft stage, POV, how blunt they want feedback),
   and plain conversation where they aren't (what the book is about, what
   they're worried about). Cover:

   - **What the book is** — logline, genre, subgenre, target audience
   - **Comps** — a couple of published books it sits beside; this
     communicates register faster than adjectives do
   - **Draft stage** — exploratory / first complete draft / revised /
     near-submission. This *gates severity*, see below.
   - **Deliberate choices** — the critical field. Fragments, tense,
     unreliable narration, dialect, head-hopping-as-style, non-linear
     structure, an intentionally slow open. Ask directly: "what would look
     like a mistake but isn't?"
   - **Known problem areas** — what the author already knows is broken;
     no value in reporting it back as news
   - **Off-limits** — anything they don't want touched
   - **What they want from this pass** — specific worries beat "make it good"

4. **Write the brief** to `<manuscript_dir>/.edaitor/brief.md` using
   `reference/brief_template.md` in this skill directory as the structure.
   Write what the author actually said, in their framing — don't
   editorialize their intent into something tidier.

5. **Confirm.** Show the brief and ask whether it reflects their intent.
   Fix what's wrong. Then tell them the next step is `/edaitor:run assess`.

## Draft stage gates severity

Put this explicitly in the brief, because it changes what the passes do:

| Stage | Editing implication |
|---|---|
| Exploratory / partial | Structure only. No line-level findings at all — the prose isn't real yet. |
| First complete draft | Developmental focus. Line findings only if `major`+ or a manuscript-wide pattern. |
| Revised | Both layers in full, normal severity calibration. |
| Near-submission | Both layers, and `minor` findings become worth reporting. |

Reporting `minor` word-choice nits on an exploratory draft is noise that
buries the structural notes that actually matter at that stage.
