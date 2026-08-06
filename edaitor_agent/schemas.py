"""Structured output contracts for edaitor's editor agents.

Every editor agent returns one of the Pydantic models below via
`output_schema`, rather than free-text prose. That's a deliberate choice:
it's what lets the aggregator agent (and, later, any evaluation harness)
consume findings programmatically instead of re-parsing prose.

Severity and category are tagged at the Finding level for both layers,
per early design decision — not optional/loose fields to fill in later.
"""

from __future__ import annotations

from enum import Enum

from pydantic import BaseModel, Field


class Severity(str, Enum):
    """How much this finding matters, independent of what kind of issue it is.

    Kept deliberately small (4 levels) so the model doesn't have to make
    fine-grained distinctions it can't reliably support.
    """

    MINOR = "minor"  # nitpick; wouldn't stop a reader
    MODERATE = "moderate"  # noticeable; worth fixing before this goes out
    MAJOR = "major"  # actively undermines the chapter/scene
    CRITICAL = "critical"  # breaks the story (plot hole, continuity break, etc.)


class DevelopmentalCategory(str, Enum):
    """Categories scoped to whole-manuscript / story-level concerns."""

    PLOT = "plot"
    PACING = "pacing"
    CHARACTER_ARC = "character_arc"
    STRUCTURE = "structure"
    STAKES = "stakes"
    THEME = "theme"


class LineCategory(str, Enum):
    """Categories scoped to prose-level concerns, judged one chapter at a time."""

    PROSE_RHYTHM = "prose_rhythm"
    VOICE_CONSISTENCY = "voice_consistency"
    SHOW_DONT_TELL = "show_dont_tell"
    DIALOGUE = "dialogue"
    POV = "pov"
    WORD_CHOICE = "word_choice"


class DevelopmentalFinding(BaseModel):
    category: DevelopmentalCategory
    severity: Severity
    location: str = Field(
        description="Where in the manuscript this applies, e.g. 'Chapter 3' or 'Chapters 2-4'."
    )
    note: str = Field(
        description="The editorial note: what the issue is and why it matters."
    )
    suggestion: str | None = Field(
        default=None,
        description="A concrete suggestion for addressing the issue, if there is one.",
    )


class LineFinding(BaseModel):
    category: LineCategory
    severity: Severity
    location: str = Field(
        description="Where in the chapter this applies, e.g. a paragraph or line reference."
    )
    excerpt: str | None = Field(
        default=None,
        description="Short quoted excerpt the finding refers to, if applicable.",
    )
    note: str = Field(
        description="The editorial note: what the issue is and why it matters."
    )
    suggestion: str | None = Field(
        default=None,
        description="A concrete suggestion for addressing the issue, if there is one.",
    )


class DevelopmentalEditReport(BaseModel):
    scope: str = Field(
        description="What was reviewed, e.g. 'Full manuscript' or a chapter range."
    )
    findings: list[DevelopmentalFinding]


class LineEditReport(BaseModel):
    chapter: str = Field(description="Which chapter this report covers.")
    findings: list[LineFinding]


class EditorialLetter(BaseModel):
    """Final human-readable synthesis, built from the structured reports above."""

    summary: str = Field(
        description="A short overview of the manuscript's overall state."
    )
    top_priorities: list[str] = Field(
        description="The handful of findings (across both layers) the writer should address first, in order."
    )
    developmental_notes: str = Field(
        description="Prose synthesis of the developmental findings."
    )
    line_notes: str = Field(
        description="Prose synthesis of the line-editing findings, organized by chapter."
    )
