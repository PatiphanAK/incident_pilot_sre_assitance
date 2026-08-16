"""Unit tests for the escalate node (no network, no DB)."""

from __future__ import annotations

from agent.nodes.escalate_node import create_escalate_node
from domain.models import IncidentMemory
from tests.unit.fakes.fake_llm import FakeLLM


def test_escalate_formats_message_with_past_incident_context() -> None:
    llm = FakeLLM(response="please investigate manually")
    node = create_escalate_node(llm)
    past = [
        IncidentMemory(
            summary="pool exhaustion on the payments db",
            resolution="bumped pool size",
            distance=0.3,
        )
    ]

    result = node(
        {
            "incident": "payments 503s",
            "incident_type": "unknown outage",
            "analysis": "root cause unclear",
            "past_incidents": past,
        }
    )

    assert result["escalation"] == "please investigate manually"
    prompt = llm.prompts[0]
    assert "payments 503s" in prompt
    assert "unknown outage" in prompt
    assert "root cause unclear" in prompt
    assert "pool exhaustion on the payments db" in prompt
    assert "bumped pool size" in prompt


def test_escalate_handles_empty_memory() -> None:
    llm = FakeLLM()
    node = create_escalate_node(llm)

    result = node({"incident": "first ever incident"})

    assert result["escalation"] == llm.response
    assert "No relevant past incidents" in llm.prompts[0]
