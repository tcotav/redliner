"""The developmental/structural editor.

This agent reads the *entire* manuscript at once — that's the point of this
layer. Plot holes, sagging pacing, and character arcs that don't land only
show up when you can see the whole shape of the story. Contrast with the
line editor (sub_agents/line_editor.py), which deliberately sees one chapter
at a time.

No tools on this agent, by design: ADK only supports combining `tools` and
`output_schema` on the same LlmAgent for specific models (Gemini 3.0+ at time
of writing). Since structured output is non-negotiable here, this agent
stays tool-free. If a future version needs this agent to call out to a
research/lookup tool, split it into a tool-calling agent followed by a
separate formatting agent rather than fighting that constraint.
"""

from google.adk.agents import LlmAgent

from ..config import MODEL_NAME
from ..schemas import DevelopmentalEditReport

INSTRUCTION = """\
You are a developmental editor reviewing a full manuscript for a novelist.
Your job is story-level: plot, pacing, character arcs, structure, stakes,
and theme. Do NOT comment on line-level prose, word choice, or grammar —
that's a separate pass and out of scope for you.

Here is the full manuscript, chapter by chapter:

{full_manuscript_text}

For each issue you find, tag it with:
- category: one of plot, pacing, character_arc, structure, stakes, theme
- severity: minor, moderate, major, or critical — judge this by how much
  the issue would actually bother a reader or undermine the story, not by
  how easy it is to fix
- location: which chapter(s) it applies to
- note: a specific, concrete explanation — not "pacing feels off" but what
  specifically drags and why
- suggestion: a concrete direction for a fix, when you have one

Only raise findings you're confident about. If the manuscript is short or
early-draft, it's fine to return few findings rather than padding the list.
"""

developmental_editor = LlmAgent(
    name="developmental_editor",
    model=MODEL_NAME,
    description="Reviews the full manuscript for story-level issues: plot, pacing, character arcs, structure, stakes.",
    instruction=INSTRUCTION,
    output_schema=DevelopmentalEditReport,
    output_key="developmental_report",
)
