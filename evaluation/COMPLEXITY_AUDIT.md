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

## The prompt

```
You are evaluating a codebase. Evaluation and suggestions only — do not
change any file, do not commit, do not "fix" anything you find. I will
take your suggestions and cross-check them separately.

Repo: /Volumes/T7/code/ideas/redliner (currently v0.5.0)

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

A good run disagrees with something specific and says what evidence
would change its mind.

## Run log

| Date | Model | Repo version | One thing they'd change | Notes |
| --- | --- | --- | --- | --- |
| | | | | |
