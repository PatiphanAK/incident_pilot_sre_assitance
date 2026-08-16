"""decide node — route on confidence x blast radius.

Automation is only allowed when BOTH hold:

- the incident is *known*: a runbook matches ``incident_type`` and
  long-term memory recalled at least one similar past incident, and
- the procedure is *safe*: its blast radius is low or medium.

Anything else — novel incident, empty memory, or a high-blast-radius
runbook — goes to a human via the ``escalate`` branch.
"""

from __future__ import annotations

from agent.state import AgentState
from domain.ports.runbook_port import RunbookPort

_AUTOMATED_BLAST_RADIUS = ("low", "medium")


def create_decide_node(runbook: RunbookPort):
    """Return a node that stores the route in ``state["decision"]``."""

    def decide(state: AgentState) -> dict:
        match = runbook.find_matching(state.get("incident_type") or "")
        has_prior = len(state.get("past_incidents") or []) > 0
        if match is not None and has_prior and match.blast_radius in _AUTOMATED_BLAST_RADIUS:
            decision = "run_runbook"
        else:
            decision = "escalate"
        return {"decision": decision}

    return decide
