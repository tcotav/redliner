"""Assembles the edaitor pipeline: developmental edit -> line edit (per
chapter) -> aggregate into one editorial letter.

`root_agent` is the name ADK's tooling (adk web / adk run) looks for by
convention. See run_pipeline.py for why this project runs via a plain
script instead of those CLIs: this pipeline acts on manuscript files
already on disk rather than a back-and-forth chat, so a scripted Runner
invocation is a better fit than the conversational adk web UI.
"""

from google.adk.agents import SequentialAgent

from .sub_agents.aggregator import aggregator
from .sub_agents.chapter_loop import ChapterLineEditLoop
from .sub_agents.developmental import developmental_editor

root_agent = SequentialAgent(
    name="edaitor_pipeline",
    description="Runs a manuscript through developmental and line editing, then produces an editorial letter.",
    sub_agents=[
        developmental_editor,
        ChapterLineEditLoop(),
        aggregator,
    ],
)
