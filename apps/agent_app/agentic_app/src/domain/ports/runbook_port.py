"""RunbookPort — known remediation procedures for incident patterns.

The agent layer (``agent/nodes/*``) may only call this port. Concrete
backends (CockroachDB, an in-memory fake, ...) live in ``adapters/`` or
``tests/`` and are injected at graph build time.
"""

from __future__ import annotations

from typing import Protocol

from domain.models.runbook import ExecutionResult, Runbook


class RunbookPort(Protocol):
    """Runbook knowledge base + executor for known incident patterns."""

    def find_matching(self, incident_type: str) -> Runbook | None:
        """Return the runbook registered for ``incident_type``, if any.

        Exact pattern match is enough here — the semantic matching
        already happened in ``memory_check``; this just fetches the
        known procedure.
        """
        ...

    def execute(self, runbook: Runbook, params: dict) -> ExecutionResult:
        """Carry out the runbook's steps and report what was done."""
        ...
