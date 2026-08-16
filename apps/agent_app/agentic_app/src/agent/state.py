"""Shared state schema for the incident agent graph."""

from __future__ import annotations

from typing import TypedDict

from domain.models import IncidentMemory


class AgentState(TypedDict, total=False):
    """State threaded through the graph.

    - ``incident``: the raw incident report (input).
    - ``past_incidents``: memories recalled by ``memory_check``.
    - ``analysis``: the LLM's analysis produced by ``analyze``.
    """

    incident: str
    past_incidents: list[IncidentMemory]
    analysis: str
