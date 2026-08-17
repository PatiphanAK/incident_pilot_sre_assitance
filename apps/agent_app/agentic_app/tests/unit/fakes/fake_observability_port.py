"""In-memory FakeObservabilityPort for unit testing agent nodes.

Implements :class:`domain.ports.observability_port.ObservabilityPort` with
canned log lines and metric data points — zero AWS calls, zero network. It
records every request so tests can assert *what* the agent asked to observe
(log group, namespace, metric names), not just the returned values.

Canned metrics are keyed by metric name; each maps to a list of values that
become one data point each (timestamps are synthetic). An empty value list
yields an empty data-point list, which exercises the node's "no data" path.
"""

from __future__ import annotations

from domain.models import MetricDataPoint
from domain.ports.observability_port import ObservabilityPort

_DEFAULT_LOGS = [
    '{"level":"info","msg":"health check ok","db":"stock_db"}',
    '{"level":"error","msg":"db ping failed","db":"order_db","err":"context deadline exceeded"}',
]

# A small "something is wrong" default: a request-error spike and a database
# that dipped down, so out-of-the-box tests exercise the interesting path.
_DEFAULT_METRICS = {
    "Requests": [120.0],
    "RequestErrors": [17.0],
    "RequestLatency": [342.0],
    "DatabaseUp": [0.0],
    "DatabaseErrors": [5.0],
}


class FakeObservabilityPort(ObservabilityPort):
    """Canned CloudWatch evidence + a call log for assertions."""

    def __init__(
        self,
        logs: list[str] | None = None,
        metrics: dict[str, list[float]] | None = None,
    ) -> None:
        self._logs = list(logs) if logs is not None else list(_DEFAULT_LOGS)
        self._metrics = dict(metrics) if metrics is not None else dict(_DEFAULT_METRICS)
        self.log_calls: list[dict] = []
        self.metric_calls: list[dict] = []

    def fetch_recent_logs(
        self,
        log_group: str,
        filter_pattern: str = "",
        minutes: int = 15,
    ) -> list[str]:
        self.log_calls.append(
            {"log_group": log_group, "filter_pattern": filter_pattern, "minutes": minutes}
        )
        return list(self._logs)

    def get_metric(
        self,
        namespace: str,
        metric_name: str,
        dimensions: dict[str, str],
        minutes: int = 15,
        statistic: str = "Average",
    ) -> list[MetricDataPoint]:
        self.metric_calls.append(
            {
                "namespace": namespace,
                "metric_name": metric_name,
                "dimensions": dimensions,
                "minutes": minutes,
                "statistic": statistic,
            }
        )
        values = self._metrics.get(metric_name, [])
        return [
            MetricDataPoint(timestamp=f"t{i}", value=v) for i, v in enumerate(values)
        ]
