"""Unit tests for the alerts inbound router — fakes only, no server process.

Covers the DoD scenarios through real HTTP calls (TestClient): a seeded
incident pattern routes to run_runbook with simulated steps in the
response, an unseen incident type routes to escalate.
"""

from __future__ import annotations

from fastapi import FastAPI
from fastapi.testclient import TestClient

from adapters.inbound.http.routers.alerts import create_alerts_router
from agent.graph import build_graph
from domain.models import Runbook
from tests.unit.fakes.fake_llm import FakeLLM
from tests.unit.fakes.fake_memory_port import FakeMemoryPort
from tests.unit.fakes.fake_observability_port import FakeObservabilityPort
from tests.unit.fakes.fake_runbook_port import FakeRunbookPort

_RUNBOOK = Runbook(
    id="00000000-0000-0000-0000-000000000004",
    incident_pattern="connection pool exhaustion",
    steps=["restart the deployment", "watch pool metrics"],
    blast_radius="low",
)


def _client(memory: FakeMemoryPort, runbook: FakeRunbookPort) -> TestClient:
    graph = build_graph(llm=FakeLLM(), memory=memory, runbook=runbook)
    app = FastAPI()
    app.include_router(create_alerts_router(graph))
    return TestClient(app)


def test_alert_with_seeded_pattern_returns_run_runbook_with_simulated_steps() -> None:
    memory = FakeMemoryPort()
    memory.save_incident(
        summary="database connection pool exhaustion caused 5xx errors",
        resolution="raised max pool size",
    )
    client = _client(memory, FakeRunbookPort([_RUNBOOK]))

    response = client.post(
        "/alerts",
        json={
            "source": "grafana",
            "incident_type": "connection pool exhaustion",
            "summary": "payments service running out of database connections",
            "severity": "warning",
        },
    )

    assert response.status_code == 200
    data = response.json()
    assert data["decision"] == "run_runbook"
    assert data["execution"]["status"] == "simulated"
    assert data["execution"]["action"] == _RUNBOOK.steps


def test_alert_with_unseen_incident_type_returns_escalate() -> None:
    memory = FakeMemoryPort()  # nothing stored → novel incident
    client = _client(memory, FakeRunbookPort([_RUNBOOK]))

    response = client.post(
        "/alerts",
        json={
            "source": "synthetic",
            "incident_type": "certificate rotation failure",
            "summary": "tls certs expired on the edge tier",
        },
    )

    assert response.status_code == 200
    data = response.json()
    assert data["decision"] == "escalate"
    assert data["escalation"]
    # The incident itself is still persisted for future recall.
    assert len(memory.summaries) == 1


def test_alert_persists_the_incident_so_the_next_one_has_prior_memory() -> None:
    """The demo narrative: first alert escalates, the repeat runs the book."""
    memory = FakeMemoryPort()
    runbook = FakeRunbookPort([_RUNBOOK])
    client = _client(memory, runbook)
    payload = {
        "source": "synthetic",
        "incident_type": "connection pool exhaustion",
        "summary": "payments service running out of database connections",
    }

    first = client.post("/alerts", json=payload).json()
    assert first["decision"] == "escalate"  # novel: no prior memory yet

    second = client.post("/alerts", json=payload).json()
    assert second["decision"] == "run_runbook"
    assert second["execution"]["action"] == _RUNBOOK.steps


def test_alert_rejects_payload_missing_required_fields() -> None:
    client = _client(FakeMemoryPort(), FakeRunbookPort())

    response = client.post("/alerts", json={"source": "synthetic"})

    assert response.status_code == 422


def test_alert_with_log_group_feeds_logs_into_analysis() -> None:
    """End-to-end: a log_group in the alert reaches the observe port, and the
    fetched log lines are folded into the analyze LLM prompt."""
    memory = FakeMemoryPort()
    llm = FakeLLM()
    observability = FakeObservabilityPort(logs=["db ping failed on order_db"])
    graph = build_graph(
        llm=llm, memory=memory, runbook=FakeRunbookPort(), observability=observability
    )
    app = FastAPI()
    app.include_router(create_alerts_router(graph))
    client = TestClient(app)

    response = client.post(
        "/alerts",
        json={
            "source": "synthetic",
            "incident_type": "db down",
            "summary": "order database is slow",
            "log_group": "/ecs/stock-app",
        },
    )

    assert response.status_code == 200
    # The agent asked the observability port for the requested log group.
    assert observability.log_calls[0]["log_group"] == "/ecs/stock-app"
    # And the fetched log line reached the analyze prompt.
    assert "db ping failed on order_db" in llm.prompts[0]
