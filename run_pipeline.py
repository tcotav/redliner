"""Loads a manuscript directory, seeds it into session state, and runs the
edaitor pipeline end to end.

Why a script instead of `adk web` / `adk run`: those CLIs are built around
a conversational loop (you type, the agent replies). edaitor's job is to
act on manuscript files already sitting on disk with no back-and-forth
needed, so wiring the Runner directly gives a plain `python run_pipeline.py`
entry point instead of forcing a chat interface onto a batch job.

Usage:
    python run_pipeline.py [manuscript_dir]

Requires GOOGLE_API_KEY (or Vertex AI credentials) set — see .env.example.
"""

import asyncio
import json
import sys
from pathlib import Path

from dotenv import load_dotenv
from google.adk.runners import Runner
from google.adk.sessions import InMemorySessionService
from google.genai import types

load_dotenv()

from edaitor_agent.agent import root_agent

APP_NAME = "edaitor"
USER_ID = "local_user"


def load_manuscript(manuscript_dir: Path) -> dict:
    chapter_files = sorted(manuscript_dir.glob("*.txt"))
    if not chapter_files:
        raise FileNotFoundError(f"No .txt chapter files found in {manuscript_dir}")

    chapters = [
        {"name": f.stem, "text": f.read_text(encoding="utf-8")} for f in chapter_files
    ]
    full_text = "\n\n".join(f"## {c['name']}\n\n{c['text']}" for c in chapters)

    return {
        "chapters": chapters,
        "full_manuscript_text": full_text,
        "line_reports": [],
    }


async def main(manuscript_dir: str) -> None:
    session_service = InMemorySessionService()
    initial_state = load_manuscript(Path(manuscript_dir))

    session = await session_service.create_session(
        app_name=APP_NAME,
        user_id=USER_ID,
        state=initial_state,
    )

    runner = Runner(
        agent=root_agent, app_name=APP_NAME, session_service=session_service
    )

    # The pipeline works entirely off session state (the manuscript), not
    # off a user message -- but ADK's Runner is built around responding to
    # one, so we send a short trigger message to kick off the invocation.
    trigger = types.Content(
        role="user", parts=[types.Part(text="Please edit this manuscript.")]
    )

    async for event in runner.run_async(
        user_id=USER_ID, session_id=session.id, new_message=trigger
    ):
        if event.is_final_response() and event.content and event.content.parts:
            text = event.content.parts[0].text
            if text:
                print(f"\n--- {event.author} ---")
                print(text)

    final_session = await session_service.get_session(
        app_name=APP_NAME, user_id=USER_ID, session_id=session.id
    )
    letter = final_session.state.get("editorial_letter")
    if letter:
        print("\n\n=== Editorial Letter ===")
        print(json.dumps(letter, indent=2))


if __name__ == "__main__":
    manuscript_dir = sys.argv[1] if len(sys.argv) > 1 else "sample_manuscript"
    asyncio.run(main(manuscript_dir))
