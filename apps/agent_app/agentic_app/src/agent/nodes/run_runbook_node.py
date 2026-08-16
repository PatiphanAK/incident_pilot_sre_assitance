"""run_runbook node — execute the matched runbook (simulated)."""

from __future__ import annotations

from agent.state import AgentState
from domain.models.runbook import ExecutionResult
from domain.ports.runbook_port import RunbookPort


def create_run_runbook_node(runbook: RunbookPort):
    """Return a node that executes the runbook matching
    ``state["incident_type"]`` and stores the result in
    ``state["execution"]``.

    The adapter simulates execution — this node only routes data. If the
    matching row vanished between ``decide`` and here, the run is marked
    ``skipped`` rather than raising: the escalation context is still in
    state for the caller.
    """

    def run_runbook(state: AgentState) -> dict:
        match = runbook.find_matching(state.get("incident_type") or "")
        if match is None:
            return {"execution": ExecutionResult(status="skipped", action=[])}
        execution = runbook.execute(
            match,
            params={
                "incident_type": state.get("incident_type") or "",
                "summary": state.get("incident") or "",
            },
        )
        return {"execution": execution}

    return run_runbook
