"""Unit tests for the analyze node (no network, no DB)."""

from __future__ import annotations

from agent.nodes.analyze_node import create_analyze_node
from domain.models import IncidentMemory
from tests.unit.fakes.fake_llm import FakeLLM


def test_analyze_calls_llm_and_returns_analysis() -> None:
    llm = FakeLLM(response="root cause: pool exhaustion")
    node = create_analyze_node(llm)

    result = node({"incident": "orders API returning 503"})

    assert result["analysis"] == "root cause: pool exhaustion"
    assert len(llm.prompts) == 1
    assert "orders API returning 503" in llm.prompts[0]


def test_analyze_includes_recalled_incidents_in_prompt() -> None:
    llm = FakeLLM()
    node = create_analyze_node(llm)
    past = [
        IncidentMemory(
            summary="pool exhaustion on the payments db",
            resolution="bumped pool size",
            distance=0.2,
        )
    ]
    node({"incident": "payments 503s", "past_incidents": past})

    assert "pool exhaustion on the payments db" in llm.prompts[0]
    assert "bumped pool size" in llm.prompts[0]


def test_analyze_handles_missing_past_incidents() -> None:
    llm = FakeLLM()
    node = create_analyze_node(llm)
    result = node({"incident": "disk full on node-7"})

    assert result["analysis"] == llm.response
    assert "No relevant past incidents" in llm.prompts[0]
