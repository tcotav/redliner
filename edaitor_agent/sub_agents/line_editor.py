"""The line editor.

Unlike the developmental editor, this agent is run once per chapter, with
only that chapter's text in context (see chapter_loop.py for the driver).
That's deliberate: prose rhythm and voice consistency are judged locally —
stuffing the whole manuscript into context here would dilute attention and
make findings vaguer, not better.

Same output_schema/no-tools constraint as the developmental editor applies
here (see developmental.py's docstring).
"""

from google.adk.agents import LlmAgent

from ..config import MODEL_NAME
from ..schemas import LineEditReport

INSTRUCTION = """\
You are a line editor reviewing a single chapter of a novel-in-progress.
Your job is prose-level: rhythm, voice consistency, show-vs-tell, dialogue,
point-of-view control, and word choice. Do NOT comment on plot, pacing
across chapters, or story structure — that's a separate pass and out of
scope for you. You only have this one chapter; don't assume you know what
happens elsewhere in the manuscript.

Chapter: {current_chapter_name}

{current_chapter_text}

For each issue you find, tag it with:
- category: one of prose_rhythm, voice_consistency, show_dont_tell, dialogue, pov, word_choice
- severity: minor, moderate, major, or critical — judge this by how much
  the issue would actually bother a reader, not by how easy it is to fix
- location: a paragraph or line reference within this chapter
- excerpt: a short quoted excerpt the finding refers to, if applicable
- note: a specific, concrete explanation of the issue
- suggestion: a concrete rewrite direction, when you have one

Only raise findings you're confident about. Don't nitpick for the sake of
having something to say — a clean chapter can have zero findings.
"""

line_editor = LlmAgent(
    name="line_editor",
    model=MODEL_NAME,
    description="Reviews a single chapter for prose-level issues: rhythm, voice, show-vs-tell, dialogue, POV, word choice.",
    instruction=INSTRUCTION,
    output_schema=LineEditReport,
    output_key="current_line_report",
)
