"""Unit tests for the CloudWatch adapter (mocked boto3 clients, no live AWS).

The adapter takes an injectable ``client_factory`` so these tests supply canned
clients instead of real boto3 clients. This exercises the request mapping and
the graceful-degradation contract (any AWS failure -> empty result, no raise).
"""

from __future__ import annotations

from datetime import datetime, timezone
from unittest.mock import MagicMock

import botocore.exceptions

from adapters.outbound.observability.cloudwatch_adapter import CloudWatchAdapter


def _factory(logs_client: MagicMock, cw_client: MagicMock):
    calls: list[str] = []

    def factory(service: str) -> MagicMock:
        calls.append(service)
        return {"logs": logs_client, "cloudwatch": cw_client}[service]

    return CloudWatchAdapter(client_factory=factory), calls


def test_fetch_recent_logs_returns_messages() -> None:
    logs = MagicMock()
    logs.filter_log_events.return_value = {
        "events": [{"message": "line-a"}, {"message": "line-b"}]
    }
    adapter, _ = _factory(logs, MagicMock())

    assert adapter.fetch_recent_logs("/ecs/stock-app", minutes=10) == ["line-a", "line-b"]


def test_fetch_recent_logs_passes_filter_pattern_when_set() -> None:
    logs = MagicMock()
    logs.filter_log_events.return_value = {"events": []}
    adapter, _ = _factory(logs, MagicMock())

    adapter.fetch_recent_logs("/g", filter_pattern="ERROR")
    _, kwargs = logs.filter_log_events.call_args
    assert kwargs["filterPattern"] == "ERROR"


def test_fetch_recent_logs_degrades_on_missing_group() -> None:
    logs = MagicMock()
    logs.filter_log_events.side_effect = botocore.exceptions.ClientError(
        {"Error": {"Code": "ResourceNotFoundException", "Message": "no group"}},
        "FilterLogEvents",
    )
    adapter, _ = _factory(logs, MagicMock())

    assert adapter.fetch_recent_logs("/does/not/exist") == []


def test_fetch_recent_logs_degrades_on_missing_credentials() -> None:
    logs = MagicMock()
    logs.filter_log_events.side_effect = botocore.exceptions.NoCredentialsError()
    adapter, _ = _factory(logs, MagicMock())

    assert adapter.fetch_recent_logs("/ecs/stock-app") == []


def test_get_metric_maps_datapoints() -> None:
    cw = MagicMock()
    ts = datetime(2026, 8, 18, 12, 0, 0, tzinfo=timezone.utc)
    cw.get_metric_statistics.return_value = {
        "Datapoints": [
            {"Timestamp": ts, "Average": 1.5},
            {"Timestamp": ts, "Average": 2.5},
            {"Timestamp": ts},  # missing the statistic -> skipped
        ]
    }
    adapter, _ = _factory(MagicMock(), cw)

    points = adapter.get_metric(
        "stock_app", "RequestLatency", {"Service": "stock_app"}, statistic="Average"
    )
    assert [p.value for p in points] == [1.5, 2.5]
    assert points[0].timestamp == "2026-08-18T12:00:00Z"


def test_get_metric_sends_dimensions() -> None:
    cw = MagicMock()
    cw.get_metric_statistics.return_value = {"Datapoints": []}
    adapter, _ = _factory(MagicMock(), cw)

    adapter.get_metric(
        "stock_app", "DatabaseUp", {"Service": "stock_app", "Database": "stock_db"}
    )
    _, kwargs = cw.get_metric_statistics.call_args
    assert kwargs["Dimensions"] == [
        {"Name": "Service", "Value": "stock_app"},
        {"Name": "Database", "Value": "stock_db"},
    ]


def test_get_metric_degrades_on_client_error() -> None:
    cw = MagicMock()
    cw.get_metric_statistics.side_effect = botocore.exceptions.ClientError(
        {"Error": {"Code": "AccessDenied", "Message": "denied"}}, "GetMetricStatistics"
    )
    adapter, _ = _factory(MagicMock(), cw)

    assert adapter.get_metric("stock_app", "X", {}) == []


def test_clients_are_lazy() -> None:
    logs = MagicMock()
    cw = MagicMock()
    adapter, calls = _factory(logs, cw)

    # Constructing the adapter must not touch the network / create clients.
    assert calls == []
    adapter.fetch_recent_logs("/g")
    assert calls == ["logs"]
    adapter.get_metric("ns", "m", {})
    assert calls == ["logs", "cloudwatch"]
