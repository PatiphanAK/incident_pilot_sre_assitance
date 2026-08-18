"""End-to-end graph test using only fakes — zero live connections."""

from __future__ import annotations

from agent.graph import build_graph
from domain.models import Runbook
from tests.unit.fakes.fake_llm import FakeLLM
from tests.unit.fakes.fake_memory_port import FakeMemoryPort
from tests.unit.fakes.fake_runbook_port import FakeRunbookPort

_RUNBOOK = Runbook(
    id="00000000-0000-0000-0000-000000000003",
    incident_pattern="connection pool exhaustion",
    steps=["restart the deployment", "watch pool metrics"],
    blast_radius="low",
)


def test_graph_runs_end_to_end_and_closes_the_memory_loop() -> None:
    memory = FakeMemoryPort()
    # Seed a historical incident the agent should recall.
    memory.save_incident(
        summary="database connection pool exhaustion caused 5xx errors",
        resolution="raised max pool size and added retry budget",
    )
    llm = FakeLLM(response="likely root cause: pool exhaustion")

    graph = build_graph(
        llm=llm, memory=memory, runbook=FakeRunbookPort([_RUNBOOK]), top_k=3
    )
    # No incident_type → no runbook match → the escalate branch runs.
    result = graph.invoke({"incident": "the database is running out of connections"})

    # 1. Recall: the seeded incident was found and stored in state.
    assert len(result["past_incidents"]) == 1
    assert "database" in result["past_incidents"][0].summary.lower()

    # 2. Reason: the LLM was called with the recalled context.
    assert result["analysis"] == "likely root cause: pool exhaustion"
    assert "database connection pool exhaustion" in llm.prompts[0]

    # 3. Decide + escalate: free-text input has no incident_type.
    assert result["decision"] == "escalate"
    assert result["escalation"] == llm.response

    # 4. Persist: the new incident was stored for future recall.
    assert len(memory.summaries) == 2
    assert "the database is running out of connections" in memory.summaries


def test_graph_with_empty_memory_still_completes() -> None:
    memory = FakeMemoryPort()
    llm = FakeLLM()
    graph = build_graph(llm=llm, memory=memory, runbook=FakeRunbookPort([_RUNBOOK]))

    result = graph.invoke({"incident": "first ever incident, nothing stored yet"})

    assert result["past_incidents"] == []
    assert result["analysis"] == llm.response
    assert result["decision"] == "escalate"
    assert len(memory.summaries) == 1


def test_graph_runs_the_runbook_for_known_low_blast_incident() -> None:
    memory = FakeMemoryPort()
    memory.save_incident(
        summary="database connection pool exhaustion caused 5xx errors",
        resolution="raised max pool size",
    )
    llm = FakeLLM(response="likely root cause: pool exhaustion")
    runbook = FakeRunbookPort([_RUNBOOK])

    graph = build_graph(llm=llm, memory=memory, runbook=runbook)
    result = graph.invoke(
        {
            "incident": "the database is running out of connections",
            "incident_type": "connection pool exhaustion",
        }
    )

    assert result["decision"] == "run_runbook"
    assert result["execution"].status == "simulated"
    assert result["execution"].action == _RUNBOOK.steps
    assert len(runbook.executed) == 1
    # Memory still closes the loop on the automated branch.
    assert len(memory.summaries) == 2


def test_graph_escalates_known_high_blast_incident() -> None:
    memory = FakeMemoryPort()
    memory.save_incident(summary="region outage last month", resolution="failed over")
    high = _RUNBOOK.model_copy(update={"blast_radius": "high"})
    runbook = FakeRunbookPort([high])
    llm = FakeLLM()

    graph = build_graph(llm=llm, memory=memory, runbook=runbook)
    result = graph.invoke(
        {
            "incident": "whole region is down",
            "incident_type": "connection pool exhaustion",
        }
    )

    assert result["decision"] == "escalate"
    assert "escalation" in result
    assert runbook.executed == []  # the runbook must NOT have run
