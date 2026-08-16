"""End-to-end graph test using only fakes — zero live connections."""

from __future__ import annotations

from agent.graph import build_graph
from tests.unit.fakes.fake_llm import FakeLLM
from tests.unit.fakes.fake_memory_port import FakeMemoryPort


def test_graph_runs_end_to_end_and_closes_the_memory_loop() -> None:
    memory = FakeMemoryPort()
    # Seed a historical incident the agent should recall.
    memory.save_incident(
        summary="database connection pool exhaustion caused 5xx errors",
        resolution="raised max pool size and added retry budget",
    )
    llm = FakeLLM(response="likely root cause: pool exhaustion")

    graph = build_graph(llm=llm, memory=memory, top_k=3)
    result = graph.invoke({"incident": "the database is running out of connections"})

    # 1. Recall: the seeded incident was found and stored in state.
    assert len(result["past_incidents"]) == 1
    assert "database" in result["past_incidents"][0].summary.lower()

    # 2. Reason: the LLM was called with the recalled context.
    assert result["analysis"] == "likely root cause: pool exhaustion"
    assert "database connection pool exhaustion" in llm.prompts[0]

    # 3. Persist: the new incident was stored for future recall.
    assert len(memory.summaries) == 2
    assert "the database is running out of connections" in memory.summaries


def test_graph_with_empty_memory_still_completes() -> None:
    memory = FakeMemoryPort()
    llm = FakeLLM()
    graph = build_graph(llm=llm, memory=memory)

    result = graph.invoke({"incident": "first ever incident, nothing stored yet"})

    assert result["past_incidents"] == []
    assert result["analysis"] == llm.response
    assert len(memory.summaries) == 1
