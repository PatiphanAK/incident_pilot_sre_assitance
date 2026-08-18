"""Domain models for runbook knowledge and (simulated) execution.

Pydantic here (unlike the plain-dataclass incident models) because these
objects cross the HTTP boundary: the alerts router returns the execution
result directly in its JSON response.
"""

from __future__ import annotations

from pydantic import BaseModel


class Runbook(BaseModel):
    """A known remediation procedure for one incident pattern."""

    id: str
    incident_pattern: str
    steps: list[str]
    blast_radius: str  # "low" | "medium" | "high"


class ExecutionResult(BaseModel):
    """Outcome of handing a runbook to the executor port."""

    status: str  # "simulated" | "executed" | "skipped"
    action: list[str]
