"""In-memory FakeRunbookPort for unit testing agent nodes.

Implements :class:`domain.ports.runbook_port.RunbookPort` with a plain
dict keyed by ``incident_pattern`` — zero network calls, zero DB
connections. ``execute`` mirrors the real adapter's simulate-only
behavior and records what it was asked to run so tests can assert on it.
"""

from __future__ import annotations

from domain.models.runbook import ExecutionResult, Runbook
from domain.ports.runbook_port import RunbookPort


class FakeRunbookPort(RunbookPort):
    """In-memory runbook knowledge base + simulated executor."""

    def __init__(self, runbooks: list[Runbook] | None = None) -> None:
        self._by_pattern = {r.incident_pattern: r for r in (runbooks or [])}
        self.executed: list[Runbook] = []
        self.execution_params: list[dict] = []

    def find_matching(self, incident_type: str) -> Runbook | None:
        return self._by_pattern.get(incident_type)

    def execute(self, runbook: Runbook, params: dict) -> ExecutionResult:
        self.executed.append(runbook)
        self.execution_params.append(params)
        return ExecutionResult(status="simulated", action=list(runbook.steps))
