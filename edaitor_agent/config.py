"""Shared config. Kept in one place so swapping models is a one-line change."""

import os

# Override via .env / environment if you have access to a different model.
MODEL_NAME = os.environ.get("EDAITOR_MODEL", "gemini-2.5-flash")
