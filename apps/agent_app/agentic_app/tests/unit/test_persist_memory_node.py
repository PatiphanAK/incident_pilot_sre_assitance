"""Unit tests for the persist_memory node (no network, no DB)."""

from __future__ import annotations

from agent.nodes.persist_memory_node import create_persist_memory_node, _summarize
from tests.unit.fakes.fake_memory_port import FakeMemoryPort


def test_persist_saves_summary_and_resolution() -> None:
    memory = FakeMemoryPort()
    node = create_persist_memory_node(memory)

    node(
        {
            "incident": "checkout service returned 500 errors for ten minutes",
            "analysis": "root cause: null pointer in pricing client",
        }
    )

    assert memory.summaries == ["checkout service returned 500 errors for ten minutes"]
    recalled = memory.find_similar("checkout service 500 errors")
    assert recalled[0].resolution == "root cause: null pointer in pricing client"


def test_persist_works_without_analysis() -> None:
    memory = FakeMemoryPort()
    node = create_persist_memory_node(memory)
    node({"incident": "latency spike at 03:00"})

    recalled = memory.find_similar("latency spike")
    assert recalled[0].resolution is None


def test_summarize_bounds_length() -> None:
    long_text = " ".join(f"word{i}" for i in range(200))
    assert len(_summarize(long_text)) <= 301

    short = "short incident"
    assert _summarize(short) == short


def test_summarize_collapses_whitespace() -> None:
    assert _summarize("  a   b\n\nc  ") == "a b c"
