"""Unit tests for the decide node (no network, no DB)."""

from __future__ import annotations

from agent.nodes.decide_node import create_decide_node
from domain.models import IncidentMemory, Runbook
from tests.unit.fakes.fake_runbook_port import FakeRunbookPort


def _runbook(pattern: str, blast_radius: str) -> Runbook:
    return Runbook(
        id="00000000-0000-0000-0000-000000000001",
        incident_pattern=pattern,
        steps=["step one"],
        blast_radius=blast_radius,
    )


def _prior() -> list[IncidentMemory]:
    return [IncidentMemory(summary="same thing last week", resolution=None, distance=0.4)]


def test_known_low_blast_incident_runs_the_runbook() -> None:
    runbook = FakeRunbookPort([_runbook("connection pool exhaustion", "low")])
    node = create_decide_node(runbook)

    result = node({"incident_type": "connection pool exhaustion", "past_incidents": _prior()})

    assert result["decision"] == "run_runbook"


def test_known_medium_blast_incident_runs_the_runbook() -> None:
    runbook = FakeRunbookPort([_runbook("disk usage high", "medium")])
    node = create_decide_node(runbook)

    result = node({"incident_type": "disk usage high", "past_incidents": _prior()})

    assert result["decision"] == "run_runbook"


def test_unseen_incident_type_escalates() -> None:
    runbook = FakeRunbookPort([_runbook("connection pool exhaustion", "low")])
    node = create_decide_node(runbook)

    result = node({"incident_type": "something brand new", "past_incidents": _prior()})

    assert result["decision"] == "escalate"


def test_high_blast_radius_escalates_even_when_known() -> None:
    runbook = FakeRunbookPort([_runbook("primary region outage", "high")])
    node = create_decide_node(runbook)

    result = node({"incident_type": "primary region outage", "past_incidents": _prior()})

    assert result["decision"] == "escalate"


def test_known_incident_without_prior_memory_escalates() -> None:
    """Novel = no similar past incident, even if a runbook pattern matches."""
    runbook = FakeRunbookPort([_runbook("connection pool exhaustion", "low")])
    node = create_decide_node(runbook)

    result = node({"incident_type": "connection pool exhaustion", "past_incidents": []})

    assert result["decision"] == "escalate"


def test_missing_incident_type_escalates() -> None:
    """Free-text callers (the CLI) pass no incident_type — always escalate."""
    runbook = FakeRunbookPort([_runbook("connection pool exhaustion", "low")])
    node = create_decide_node(runbook)

    result = node({"past_incidents": _prior()})

    assert result["decision"] == "escalate"
