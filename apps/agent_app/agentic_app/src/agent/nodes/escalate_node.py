"""escalate node — hand the incident to a human operator.

Formats the escalation message with the recalled ``past_incidents`` as
context. No new port needed: message formatting is just another
:class:`domain.ports.llm_port.LLMPort` call.
"""

from __future__ import annotations

from domain.models import IncidentMemory
from domain.ports.llm_port import LLMPort

from agent.state import AgentState

_SYSTEM_PROMPT = (
    "You are an SRE incident agent writing an escalation message to the "
    "on-call human operator. Automated remediation was skipped because no "
    "safe runbook applies (the incident is novel, or the matching runbook "
    "has a high blast radius). Write a concise escalation: what happened, "
    "why automation did not act, and what the operator should investigate "
    "first. Reference the past incidents below if any are provided."
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


def create_escalate_node(llm: LLMPort):
    """Return a node that produces ``state["escalation"]`` via the LLM."""

    def escalate(state: AgentState) -> dict:
        prompt = (
            f"{_SYSTEM_PROMPT}\n\n"
            f"Reported incident (type: {state.get('incident_type') or 'unknown'}):\n"
            f"{state['incident']}\n\n"
            f"Agent analysis:\n{state.get('analysis') or '(none)'}\n\n"
            f"Relevant past incidents from long-term memory:\n"
            f"{_format_past_incidents(state.get('past_incidents') or [])}"
        )
        return {"escalation": llm.generate(prompt)}

    return escalate
