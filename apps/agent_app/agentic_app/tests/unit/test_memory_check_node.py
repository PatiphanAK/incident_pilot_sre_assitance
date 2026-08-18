"""Unit tests for the memory_check node (no network, no DB)."""

from __future__ import annotations

from agent.nodes.memory_check_node import create_memory_check_node
from tests.unit.fakes.fake_memory_port import FakeMemoryPort


def test_memory_check_stores_relevant_past_incidents() -> None:
    memory = FakeMemoryPort()
    memory.save_incident(
        summary="database connection pool exhaustion caused 5xx errors",
        resolution="raised max pool size",
    )
    memory.save_incident(
        summary="kubernetes pod crashloop due to OOM killer",
        resolution="increased memory limits",
    )

    node = create_memory_check_node(memory, top_k=2)
    result = node({"incident": "the database is running out of connections"})

    assert len(result["past_incidents"]) == 2
    # Most-similar (database/connection overlap) must rank first.
    assert "database" in result["past_incidents"][0].summary.lower()
    for mem in result["past_incidents"]:
        assert mem.distance >= 0.0


def test_memory_check_returns_empty_when_nothing_stored() -> None:
    node = create_memory_check_node(FakeMemoryPort(), top_k=3)
    result = node({"incident": "service latency spike"})
    assert result["past_incidents"] == []


def test_memory_check_respects_top_k() -> None:
    memory = FakeMemoryPort()
    for i in range(5):
        memory.save_incident(summary=f"database incident number {i}")
    node = create_memory_check_node(memory, top_k=2)
    result = node({"incident": "database incident"})
    assert len(result["past_incidents"]) == 2
