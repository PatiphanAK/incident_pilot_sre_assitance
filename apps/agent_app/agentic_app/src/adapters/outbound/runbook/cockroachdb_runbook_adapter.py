"""CockroachDB adapter for :class:`domain.ports.runbook_port.RunbookPort`.

Backed by the ``runbooks`` table (see ``migrations/003_create_runbooks.sql``)::

    CREATE TABLE runbooks (
        id UUID DEFAULT gen_random_uuid() PRIMARY KEY,
        incident_pattern STRING NOT NULL,
        steps JSONB NOT NULL,
        blast_radius STRING NOT NULL,
        created_at TIMESTAMPTZ DEFAULT now()
    );

``execute`` is deliberately **simulate-only**: it logs the steps and
reports ``status="simulated"`` — no real infrastructure is touched. This
is a scope decision for the hackathon; do not wire real remediation
here without revisiting the blast-radius gating in the decide node.
"""

from __future__ import annotations

import logging
from typing import Any

from psycopg_pool import ConnectionPool

from adapters.outbound.cockroachdb_dsn import build_dsn
from domain.models.runbook import ExecutionResult, Runbook

logger = logging.getLogger(__name__)

_TABLE = "runbooks"


class CockroachDBRunbookAdapter:
    """Runbook knowledge base + simulated executor backed by CockroachDB."""

    def __init__(self, conninfo: str, pool_size: int = 2) -> None:
        # Lazily-filled pool: constructing this does not open a connection,
        # the first checkout does.
        self._pool = ConnectionPool(
            conninfo, min_size=0, max_size=pool_size, timeout=30, open=True
        )

    def find_matching(self, incident_type: str) -> Runbook | None:
        """Exact ``incident_pattern`` lookup — the semantic matching
        already happened in ``memory_check``; this just fetches the known
        procedure."""
        query = (
            f"SELECT id, incident_pattern, steps, blast_radius FROM {_TABLE} "
            "WHERE incident_pattern = %s LIMIT 1"
        )
        with self._pool.connection() as conn, conn.cursor() as cur:
            cur.execute(query, (incident_type,))
            row = cur.fetchone()
        if row is None:
            return None
        return self._to_runbook(row)

    def execute(self, runbook: Runbook, params: dict) -> ExecutionResult:
        """Simulate the runbook — log the steps, touch nothing."""
        logger.info(
            "[runbook:simulated] %s (%s blast radius) — params=%s",
            runbook.incident_pattern,
            runbook.blast_radius,
            params,
        )
        for i, step in enumerate(runbook.steps, start=1):
            logger.info("[runbook:simulated]   step %d: %s", i, step)
        return ExecutionResult(status="simulated", action=list(runbook.steps))

    @staticmethod
    def _to_runbook(row: tuple[Any, ...]) -> Runbook:
        id_, pattern, steps, blast_radius = row
        return Runbook(
            id=str(id_),
            incident_pattern=pattern,
            steps=list(steps),
            blast_radius=blast_radius,
        )

    def close(self) -> None:
        self._pool.close()


def runbook_from_env() -> CockroachDBRunbookAdapter:
    """Build the adapter from the ``COCKROARCH_*`` environment."""
    return CockroachDBRunbookAdapter(build_dsn())
