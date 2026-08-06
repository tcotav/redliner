"""Custom agent that drives the line editor across chapters.

ADK ships a `LoopAgent` primitive, but it's built for "repeat these agents
until a stop condition fires" (e.g. iterative refinement of one piece of
text). What we need here is different: run the *same* agent once per item
in a list we already know the length of, accumulating one report per item.
That's simple, deterministic control flow — a plain Python loop — so per
ADK's own guidance, a custom agent (subclassing BaseAgent) is the right
tool rather than bending LoopAgent's exit_loop/max_iterations mechanism to
fit.

NOTE: this is a first-pass implementation against the documented BaseAgent
pattern. BaseAgent subclasses are pydantic models, which is why sub-agents
get declared as typed class attributes rather than set ad hoc in __init__.
Verify field-declaration details against the `google-adk` version actually
installed (`pip show google-adk`) the first time this runs — ADK is a young,
fast-moving library and exact constructor requirements may have shifted.
"""

from collections.abc import AsyncGenerator

from google.adk.agents import BaseAgent, LlmAgent
from google.adk.agents.invocation_context import InvocationContext
from google.adk.events import Event

from .line_editor import line_editor


class ChapterLineEditLoop(BaseAgent):
    """Runs `line_editor` once per chapter in session.state['chapters'],
    collecting results into session.state['line_reports'].
    """

    line_editor: LlmAgent

    def __init__(
        self, name: str = "chapter_line_edit_loop", editor: LlmAgent = line_editor
    ):
        super().__init__(name=name, line_editor=editor, sub_agents=[editor])

    async def _run_async_impl(
        self, ctx: InvocationContext
    ) -> AsyncGenerator[Event, None]:
        chapters = ctx.session.state.get("chapters", [])
        reports = []

        for chapter in chapters:
            ctx.session.state["current_chapter_name"] = chapter["name"]
            ctx.session.state["current_chapter_text"] = chapter["text"]

            async for event in self.line_editor.run_async(ctx):
                yield event

            # line_editor wrote its structured report to this state key via
            # output_key (see line_editor.py). Pull it off before the next
            # iteration overwrites it.
            report = ctx.session.state.get("current_line_report")
            if report is not None:
                reports.append(report)

        ctx.session.state["line_reports"] = reports
