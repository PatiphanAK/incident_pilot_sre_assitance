"""CockroachDB adapter for :class:`domain.ports.memory_port.MemoryPort`.

CockroachDB access lives in the outbound adapters (this one and the
runbook adapter) — no other layer may import ``psycopg``. Vector search
uses CockroachDB's distributed vector index (HNSW) via the ``<->`` L2
distance operator::

    CREATE TABLE observed_incidents (
        id UUID DEFAULT gen_random_uuid() PRIMARY KEY,
        summary STRING NOT NULL,
        resolution STRING,
        embedding VECTOR(1536) NOT NULL,
        created_at TIMESTAMPTZ DEFAULT now(),
        VECTOR INDEX (embedding)
    );

The table must exist before use; see the app README for the
create-if-missing step (done via the CockroachDB Cloud MCP).
"""

from __future__ import annotations

import json
from typing import Callable

from psycopg_pool import ConnectionPool

from adapters.outbound.cockroachdb_dsn import build_dsn
from adapters.outbound.embedding.embedder import embed as _default_embed
from domain.models import IncidentMemory
from domain.ports.memory_port import MemoryPort

_TABLE = "observed_incidents"

# Default L2 cutoff for recall. Embeddings are unit-normalized, so L2
# distance runs 0 (identical) to 2 (opposite), with ~1.414 (sqrt(2)) at
# orthogonal. Measured with text-embedding-v4: a paraphrased query sits
# around 0.77 while unrelated text sits around 1.27 — a threshold in
# between keeps recall useful without surfacing unrelated incidents just
# because the memory table is small. Pass ``max_distance=None`` to disable
# the cutoff.
_DEFAULT_MAX_DISTANCE = 1.1


class CockroachDBMemoryAdapter(MemoryPort):
    """RAG long-term memory backed by CockroachDB's vector index."""

    def __init__(
        self,
        conninfo: str,
        embed_fn: Callable[[str], list[float]] = _default_embed,
        pool_size: int = 4,
        max_distance: float | None = _DEFAULT_MAX_DISTANCE,
    ) -> None:
        self._embed = embed_fn
        self._max_distance = max_distance
        # Lazily-filled pool: constructing this does not open a connection,
        # the first checkout does.
        self._pool = ConnectionPool(
            conninfo, min_size=0, max_size=pool_size, timeout=30, open=True
        )

    def find_similar(self, text: str, top_k: int = 3) -> list[IncidentMemory]:
        vector = json.dumps(self._embed(text))
        if self._max_distance is None:
            where = ""
            params: list = [vector, top_k]
        else:
            where = " WHERE embedding <-> %s::vector < %s"
            params = [vector, vector, self._max_distance, top_k]
        query = (
            "SELECT summary, resolution, embedding <-> %s::vector AS distance "
            f"FROM {_TABLE}{where} ORDER BY distance LIMIT %s"
        )
        with self._pool.connection() as conn, conn.cursor() as cur:
            cur.execute(query, params)
            return [
                IncidentMemory(summary=row[0], resolution=row[1], distance=float(row[2]))
                for row in cur.fetchall()
            ]

    def save_incident(self, summary: str, resolution: str | None = None) -> None:
        vector = json.dumps(self._embed(summary))
        query = (
            f"INSERT INTO {_TABLE} (summary, resolution, embedding) "
            "VALUES (%s, %s, %s::vector)"
        )
        with self._pool.connection() as conn, conn.cursor() as cur:
            cur.execute(query, (summary, resolution, vector))

    def close(self) -> None:
        self._pool.close()


def memory_from_env() -> CockroachDBMemoryAdapter:
    """Build the adapter from the ``COCKROARCH_*`` environment."""
    return CockroachDBMemoryAdapter(build_dsn())
