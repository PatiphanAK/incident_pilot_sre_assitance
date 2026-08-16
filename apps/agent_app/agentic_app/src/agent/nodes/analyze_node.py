"""analyze node — reason over the incident with recalled context."""

from __future__ import annotations

from domain.models import IncidentMemory
from domain.ports.llm_port import LLMPort

from agent.state import AgentState

_SYSTEM_PROMPT = (
    "You are an SRE incident-analysis agent. Analyze the reported "
    "incident: identify the likely root cause, list concrete next "
    "investigation steps, and cite any relevant past incidents that are "
    "provided. Be concise and actionable."
)


def _format_past_incidents(past: list[IncidentMemory]) -> str:
    if not past:
        return "No relevant past incidents found in long-term memory."
    lines = []
    for i, mem in enumerate(past, start=1):
        lines.append(f"{i}. {mem.summary}")
        if mem.resolution:
            lines.append(f"   resolution: {mem.resolution}")
    return "\n".join(lines)


def create_analyze_node(llm: LLMPort):
    """Return a node that produces ``state["analysis"]`` via the LLM."""

    def analyze(state: AgentState) -> dict:
        prompt = (
            f"{_SYSTEM_PROMPT}\n\n"
            f"Reported incident:\n{state['incident']}\n\n"
            f"Relevant past incidents from long-term memory:\n"
            f"{_format_past_incidents(state.get('past_incidents') or [])}"
        )
        return {"analysis": llm.generate(prompt)}

    return analyze
