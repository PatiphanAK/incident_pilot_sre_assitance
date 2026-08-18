"""Unit tests for the run_runbook node (no network, no DB)."""

from __future__ import annotations

from agent.nodes.run_runbook_node import create_run_runbook_node
from domain.models import Runbook
from tests.unit.fakes.fake_runbook_port import FakeRunbookPort

_STEPS = ["restart the deployment", "watch pool metrics"]


def _runbook() -> Runbook:
    return Runbook(
        id="00000000-0000-0000-0000-000000000002",
        incident_pattern="connection pool exhaustion",
        steps=_STEPS,
        blast_radius="low",
    )


def test_run_runbook_executes_matched_runbook_and_stores_simulated_steps() -> None:
    runbook = FakeRunbookPort([_runbook()])
    node = create_run_runbook_node(runbook)

    result = node({"incident_type": "connection pool exhaustion", "incident": "payments 503s"})

    assert result["execution"].status == "simulated"
    assert result["execution"].action == _STEPS
    # The executor received the incident context as params.
    assert runbook.execution_params == [
        {"incident_type": "connection pool exhaustion", "summary": "payments 503s"}
    ]


def test_run_runbook_marks_skipped_when_match_disappeared() -> None:
    runbook = FakeRunbookPort()  # empty knowledge base
    node = create_run_runbook_node(runbook)

    result = node({"incident_type": "connection pool exhaustion"})

    assert result["execution"].status == "skipped"
    assert result["execution"].action == []
    assert runbook.executed == []
