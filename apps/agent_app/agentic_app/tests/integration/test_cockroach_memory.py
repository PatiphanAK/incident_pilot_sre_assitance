"""Integration tests — real CockroachDB cluster + real embedding API.

These run against the same connection the app uses (``COCKROARCH_*``
env vars loaded from ``.env``). They skip themselves when the cluster
host is not configured, so ``uv run pytest`` stays green in
environments without cluster access.

    uv run pytest -m integration
"""

from __future__ import annotations

import os
import uuid
from pathlib import Path

import pytest
from dotenv import load_dotenv

load_dotenv(Path(__file__).resolve().parents[2] / ".env")

if not os.environ.get("COCKROARCH_DB_HOST"):
    pytest.skip(
        "COCKROARCH_DB_HOST not set — live cluster tests skipped",
        allow_module_level=True,
    )

from adapters.outbound.memory.cockroachdb_adapter import memory_from_env  # noqa: E402

pytestmark = pytest.mark.integration

# A realistic incident plus a paraphrase that shares meaning, not words.
SEEDED_SUMMARY = (
    "database connection pool exhaustion on the payments service caused "
    "intermittent 503 errors and latency spikes during peak traffic"
)
SEEDED_RESOLUTION = "raised the pool max connections and added a circuit breaker"
PARAPHRASED_QUERY = (
    "why did payment transactions start failing with server errors and "
    "slow responses when traffic was high"
)


@pytest.fixture(scope="module")
def memory():
    adapter = memory_from_env()
    yield adapter
    adapter.close()


def test_seeded_incident_is_retrievable_by_paraphrased_query(memory) -> None:
    """DoD: find_similar returns relevant results for a paraphrased
    query — proves retrieval is semantic, not keyword based."""
    marker = f"[itest-{uuid.uuid4().hex[:8]}] "
    memory.save_incident(summary=marker + SEEDED_SUMMARY, resolution=SEEDED_RESOLUTION)

    try:
        hits = memory.find_similar(PARAPHRASED_QUERY, top_k=3)
        assert hits, "no results returned for a semantically similar query"

        # The seeded incident must be in the top-k despite sharing few
        # distinctive keywords with the query.
        assert any(SEEDED_SUMMARY in h.summary for h in hits), (
            f"seeded incident not found in top {len(hits)}: "
            f"{[(h.summary, h.distance) for h in hits]}"
        )
        seeded = next(h for h in hits if SEEDED_SUMMARY in h.summary)
        assert seeded.resolution == SEEDED_RESOLUTION
        # A semantic hit on a normalized 1536-dim embedding should be
        # comfortably closer than orthogonal noise (L2 ~ 1.414).
        assert seeded.distance < 1.414
    finally:
        _delete(memory, marker + SEEDED_SUMMARY)


def test_unrelated_query_does_not_rank_seeded_incident_first(memory) -> None:
    marker = f"[itest-{uuid.uuid4().hex[:8]}] "
    memory.save_incident(summary=marker + SEEDED_SUMMARY, resolution=SEEDED_RESOLUTION)

    try:
        hits = memory.find_similar(
            "frontend css layout broken after deploying new header component",
            top_k=3,
        )
        assert all(SEEDED_SUMMARY not in h.summary for h in hits), (
            "unrelated query ranked the unrelated seeded incident in the results"
        )
    finally:
        _delete(memory, marker + SEEDED_SUMMARY)


def _delete(memory, summary: str) -> None:
    """Remove a seeded row so repeated runs stay idempotent."""
    with memory._pool.connection() as conn, conn.cursor() as cur:
        cur.execute("DELETE FROM observed_incidents WHERE summary = %s", (summary,))
