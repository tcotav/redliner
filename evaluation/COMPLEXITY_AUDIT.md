# Outside complexity audit — a re-runnable prompt

A standing prompt for handing this repo to a model that did not build it,
to ask whether it is over-complicated for what it is meant to do.

It lives here, versioned next to the thing it evaluates, for the same
reason the fixtures under `go/harness/fixtures/bellwether/` do: an
evaluation you can re-run and compare against is worth more than one
good conversation. Run it on a new model, after a big change, or when
the repo starts feeling heavy.

## How to run it

1. Start a **fresh session** — no context from the work being audited.
   The auditor's value is that it did not make these decisions.
2. Paste the prompt below verbatim.
3. Record the run in the log at the bottom: date, model, repo version,
   and the one-thing-if-you-could-change-only-one answer.
4. Cross-check the findings with whoever knows the reasoning before
   acting on any of them.

**Evaluation only.** The prompt says so, but enforce it on your side too
— run it read-only. The output is an argument to weigh, not a work order.

## Keeping the instrument honest

The same rule the manuscript fixtures carry: **do not tune this prompt
until it gives answers you like.** If a run produces a finding you think
is wrong, the finding is the thing to argue with — not the prompt that
produced it. A prompt edited toward a preferred conclusion stops
measuring anything.

Two parts *are* meant to be updated, and both should be dated when they
change:

- **"Press hardest here"** — version-specific. These are the places the
  people who built it expect to defend weakest, and they move as the
  code moves. Stale entries make the audit answer last quarter's
  question.
- **"Already decided"** — grows as questions get settled. Its purpose is
  to stop each run rediscovering the same debate, not to fence off
  criticism: it asks the auditor to argue against the stated reason,
  which it is free to do.

Everything else — the goal statement, the novelist question, the two
opposed rules, the output shape — should stay fixed, so two runs a year
apart are comparable.

### How to word a "press hardest" entry

**State the charge, not the defense.** Every entry above does this, and
it is the thing to preserve — not brevity, which varies. "~1,500 lines of
a dead implementation kept alive only to regenerate golden files" is a
charge. "Is that discipline proportionate, or is it ceremony that slowed
delivery?" is a charge in the form of a question. Both leave the auditor
somewhere to stand.

The failure looks helpful: naming the thing, then explaining why it was
built that way, what it would cost to change, and how TODO.md resolved
it. That entry reads as more informative and measures less.

Two reasons.

**A lead does not merely point an auditor somewhere; it largely decides
what comes back.** The 2026-08-15 run's findings traced, without
exception, to a press-hardest entry or a TODO.md open question — and the
two entries that drew no finding were the only areas that went
unexamined. Whatever is listed here is close to a decision about what the
next run will report, so the list should be short and its entries should
open a question rather than answer one.

**Whoever made the decision will word the lead in the terms they defended
it in** — usually without noticing, because those are the terms they have.
An auditor who meets that reasoning here has met it twice: once in
TODO.md and once in the instrument meant to interrogate TODO.md. That is
not tuning toward a preferred conclusion, which the rule above forbids
outright, but it points the same way. The auditor is already told to
distrust this repo's justifications; don't hand it one more, in the
prompt.

Write entries as long as they need to be. Just check, before adding one,
whether you have written the case against the thing or the case for it —
and if the reasoning feels necessary to include, that is the signal it
belongs in TODO.md, where the auditor will find it and is instructed to
doubt it.

## The prompt

```
You are evaluating a codebase. Evaluation and suggestions only — do not
change any file, do not commit, do not "fix" anything you find. I will
take your suggestions and cross-check them separately.

Repo: /Volumes/T7/code/ideas/redliner (currently v0.6.0)

## What the tool is for

redliner is a Claude Code plugin that gives a fiction writer a second set
of editorial eyes: it reads a manuscript, asks questions about
inconsistencies, and suggests changes but never performs them. Fiction is
the primary use case. That sentence is the yardstick — complexity that
serves it is earned, complexity that doesn't isn't. Judge it as a tool
for a novelist, not as general-purpose software.

## Your primary question

A novelist installs this today. What is the shortest path to their first
useful output, and how many concepts must they hold in their head to get
there?

Answer it by actually trying, not only by reading. There is a working
binary at bin/redliner and a sample manuscript at sample_manuscript/.
Attempt to get a first useful result the way a novelist would, and report
exactly where you got confused, stuck, or had to read source to proceed.

## Also answer

1. What is over-built — complexity a smaller design would not need?
2. What is conspicuously MISSING given the goal? (Don't only look for
   excess; a prompt about over-complication finds only one kind of error.)
3. For every finding, label it: does this cost the USER (a novelist
   using it) or the MAINTAINER (one developer)? These are different
   problems with different urgency.

## Two rules that pull against each other. Follow both.

- **Before recommending any removal, find why it exists.** TODO.md and
  the git log record the reasoning for nearly every decision, often
  including a measurement. Say what would break if it were removed. Much
  of this codebase's complexity is scar tissue from real observed
  failures, not speculative engineering, and you cannot tell which is
  which by looking at the code alone.

- **Distrust the repo's own justifications.** This codebase argues for
  itself well, at length, and persuasively. Treat its stated reasons as
  claims to check, not as conclusions. A confident paragraph explaining
  why something is necessary is exactly where an unnecessary thing would
  hide.

## Reading order (TODO.md is ~2,500 lines — do not start there)

1. README.md — what it claims to do
2. skills/run/SKILL.md — the orchestration, ~618 lines, this drives
   everything
3. One agent file, e.g. agents/fiction-continuity-joiner.md
4. domains/fiction/domain.json
5. `git log --oneline -40` and then full messages for anything that looks
   relevant — the commit messages are unusually detailed and are a faster
   route to recent reasoning than TODO.md
6. TODO.md sections by name, only as needed

## Press hardest here

These are the places the maintainers expect a weak defense. Form your own
view; don't assume they're right that these are the weak spots.

- On 2026-08-14/15 two new subcommands (canon bundle, canon merge), four
  agent files, and an id-range merge convention were built AROUND a
  component whose own measurement suggests it may contribute nothing.
  See TODO.md, "Is deterministic collision-finding the right
  architecture?" Was building the hybrid the right call, or should the
  deterministic collision pass simply have been deleted?
- cont-5NN id offsetting exists to work around a `^cont-\d{3}$` regex.
- go/harness/python-baseline/ is ~1,500 lines of a dead implementation
  kept alive only to regenerate golden files. It had to be edited to
  make a Go-only change.
- Six agent roles × three domains = 18 agent files. Every prompt
  improvement is a 3× edit.
- Four pre-registered measurement exercises were run in two days
  (go/harness/fixtures/bellwether/). Is that discipline proportionate,
  or is it ceremony that slowed delivery?
- The author-facing surface: nine command forms, findings with five
  statuses, contradictions carrying kind + category + severity, draft
  stages gating passes.
- (added 2026-08-15) `canon reconcile --snapshot-after` moves the
  snapshot into the middle of the continuity flow. Right trade, right
  shape?

## Already decided — argue against the reasons, don't rediscover them

- Multi-domain (fiction, serial-fiction, design-doc) stays. Serial
  fiction exists because generic developmental editing produced
  irrelevant advice on an unfinished serial; design-doc because the
  author reads badly-constructed design docs at work.
- The Cowork/MCP plugin variant stays. The audience is fiction writers
  generally, not this one developer.
- TODO.md's length is a navigability problem, not a delete-it problem.
  The reasoning is the asset. Suggest how to make it navigable.

## Output

A ranked list, most important first — not a survey. For each item:
- The finding, in one sentence
- What it costs to keep / what breaks if removed
- USER or MAINTAINER
- Your confidence, and what evidence would change it

Then:
- If you could change only ONE thing, what?
- What did you not read, and where are you uncertain?
```

## Reading the results

Two failure modes to watch for, both of which mean the run told you
little:

- **It agrees everything is justified.** The "distrust the
  justifications" instruction didn't take — the repo talked it into
  agreeing. Push back once before treating the run as a clean bill.
- **It recommends cutting things whose reasons it never looked up.**
  The "find why it exists" instruction didn't take. Findings without a
  what-would-break claim are opinions, not arguments.

- **It hands your own uncertainty back as a finding.** Every ranked item
  traces to a "press hardest" lead or to something TODO.md already
  records as open, and the ranked answer to a direct either/or is a
  restatement of the open question. This reads as a productive run — the
  findings are all true — but it adds no information you didn't supply.
  The tell is a finding that cites TODO.md as its evidence. Check how
  many press-hardest leads drew *no* finding, too: a lead that goes
  unanswered is data about the auditor, not a clean bill for that code.

A good run disagrees with something specific and says what evidence
would change its mind.

**Verify the load-bearing facts before acting.** A run can restate the
repo accurately and still be wrong about the code — the 2026-08-15 run
got two mechanisms wrong at stated high confidence, both erring toward
alarm. Every finding names a file; open it.

## Run log

| Date | Model | Repo version | One thing they'd change | Notes |
| --- | --- | --- | --- | --- |
| 2026-08-15 | Claude Sonnet 5 | v0.5.0 (c7c4ed9) | Make `reconcile` take its snapshot baseline explicitly instead of reading ambient state — closes the silent-failure ordering invariant in `skills/run/SKILL.md` before building more on continuity. | Ranked 6 findings + 2 missing items. Top finding: the 2026-08-14/15 hybrid collision architecture sits on a deterministic pass TODO.md's own measurement says now yields zero new findings — reproduced independently on the bundled sample manuscript (1 real contradiction, 1 false positive, adjudicator correctly dropped the latter). Did not run a live `assess`/`line` pass through real subagents. Full report: see session transcript / requested artifact. **Cross-check 2026-08-15:** two ranked findings rest on false supporting facts, both stated at high confidence and both erring toward alarm — #4's "hard 100-id cap" is really ~499 ids (`canon.go` loops to `next < 1000` from 501), and #6's "no test reads any SKILL.md" is false (`frontdoor_parity_test.go` walks every skill file; the true claim is that no test checks *ordering*). The "reproduced independently" claim is reading a committed fixture from a 2026-08-09 run, not a fresh run. Findings #1–#5 restate the press-hardest leads; two leads (measurement ceremony, author-facing surface) drew no ranked finding at all.
