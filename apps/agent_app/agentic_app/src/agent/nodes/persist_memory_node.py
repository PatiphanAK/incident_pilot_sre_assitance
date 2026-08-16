"""persist_memory node — store the analyzed incident for future recall."""

from __future__ import annotations

from agent.state import AgentState
from domain.ports.memory_port import MemoryPort

_MAX_SUMMARY_CHARS = 300


def _summarize(incident: str) -> str:
    """Condense the incident report to a compact summary.

    The summary is the text that gets embedded and later matched
    against, so it should carry the salient signal: service names,
    symptoms, timing — first sentence plus a bounded tail.
    """
    text = " ".join(incident.split())
    if len(text) <= _MAX_SUMMARY_CHARS:
        return text
    return text[:_MAX_SUMMARY_CHARS].rsplit(" ", 1)[0] + "…"


def create_persist_memory_node(memory: MemoryPort):
    """Return a node that persists the incident (summary + the generated
    analysis as its resolution) after the response is produced.
    """

    def persist_memory(state: AgentState) -> dict:
        memory.save_incident(
            summary=_summarize(state["incident"]),
            resolution=state.get("analysis"),
        )
        return {}

    return persist_memory
