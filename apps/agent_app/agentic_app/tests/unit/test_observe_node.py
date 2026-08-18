"""Unit tests for the observe node (no network, no AWS)."""

from __future__ import annotations

from agent.nodes.observe_node import create_observe_node
from tests.unit.fakes.fake_observability_port import FakeObservabilityPort


def test_observe_populates_logs_and_metrics() -> None:
    obs = FakeObservabilityPort()
    node = create_observe_node(obs)

    result = node({"incident": "payments are failing"})

    # Logs come straight from the port.
    assert result["raw_logs"] == obs._logs
    # One compact summary per watched metric (3 HTTP + 2 x 3 databases).
    assert len(result["metric_observations"]) == 9
    # The agent observed the stock_app defaults (state gave none).
    assert obs.log_calls[0]["log_group"] == "/ecs/stock-app"
    assert obs.metric_calls[0]["namespace"] == "stock_app"


def test_observe_uses_payload_overrides() -> None:
    obs = FakeObservabilityPort()
    node = create_observe_node(obs)

    node({"incident": "x", "log_group": "/custom/group", "metric_namespace": "ns2"})

    assert obs.log_calls[0]["log_group"] == "/custom/group"
    assert all(call["namespace"] == "ns2" for call in obs.metric_calls)


def test_observe_empty_when_port_missing() -> None:
    node = create_observe_node(None)
    result = node({"incident": "x"})
    assert result["raw_logs"] == []
    assert result["metric_observations"] == []


def test_observe_marks_metrics_with_no_data() -> None:
    # Requests/DatabaseUp have data; RequestLatency deliberately empty.
    obs = FakeObservabilityPort(metrics={"Requests": [1.0], "DatabaseUp": [0.0]})
    node = create_observe_node(obs)

    result = node({"incident": "x"})
    by_metric = {entry["metric"]: entry for entry in result["metric_observations"]}
    assert by_metric["Requests"]["latest"] == 1.0
    assert by_metric["RequestLatency"]["latest"] is None
    assert by_metric["RequestLatency"]["note"] == "no data in window"


def test_observe_summary_math() -> None:
    obs = FakeObservabilityPort(metrics={"RequestErrors": [1.0, 2.0, 9.0]})
    node = create_observe_node(obs)

    result = node({"incident": "x"})
    errors = next(e for e in result["metric_observations"] if e["metric"] == "RequestErrors")
    assert errors["latest"] == 9.0
    assert errors["max"] == 9.0
    assert errors["avg"] == 4.0
    assert errors["points"] == 3
