"""Synthesizes the structured findings from both editing layers into one
human-readable editorial letter.

This agent never sees raw manuscript text — only the structured
DevelopmentalEditReport and LineEditReport findings already sitting in
session state. That's the payoff of tagging severity/category on every
finding: this step is synthesis over structured data, not another pass of
"read everything and summarize," so it stays cheap and consistent.
"""

from google.adk.agents import LlmAgent

from ..config import MODEL_NAME
from ..schemas import EditorialLetter

INSTRUCTION = """\
You are compiling a final editorial letter for a novelist, based on
structured findings already produced by a developmental editor and a line
editor. Do not invent new findings — synthesize the ones given.

Developmental findings:
{developmental_report}

Line-level findings, by chapter:
{line_reports}

Write:
- summary: 2-4 sentences on the manuscript's overall state
- top_priorities: the handful of findings (pulling from both layers) the
  writer should tackle first, ordered by severity and how much fixing one
  would help the others — critical/major issues that touch multiple
  chapters usually outrank a moderate line-level nitpick
- developmental_notes: a prose synthesis of the developmental findings,
  organized so it reads like an editor's letter, not a bullet dump
- line_notes: a prose synthesis of the line-level findings, organized by
  chapter

Keep the tone direct and specific, the way a good editor's letter is —
not diplomatic hedging, not generic praise.
"""

aggregator = LlmAgent(
    name="editorial_letter_aggregator",
    model=MODEL_NAME,
    description="Synthesizes developmental and line-editing findings into one editorial letter.",
    instruction=INSTRUCTION,
    output_schema=EditorialLetter,
    output_key="editorial_letter",
)
