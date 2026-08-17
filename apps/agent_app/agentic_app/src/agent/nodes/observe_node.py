"""observe node — fetch the target app's live telemetry before reasoning.

Sits after ``memory_check`` and before ``analyze``. It pulls recent logs and
a small watch-list of CloudWatch metrics via the :class:`ObservabilityPort`,
so the LLM's analysis is grounded in what the service is *actually* doing
(e.g. ``DatabaseUp`` flipping to 0, a ``RequestErrors`` spike) rather than
only the alert's text.

The node is deliberately tolerant of a missing port and of empty evidence:
when observability is unconfigured (``observability=None``) or returns
nothing, it stores empty collections and the graph proceeds exactly as it did
before this node existed. That keeps the demo, the CLI, and the no-credentials
local path all working.
"""

from __future__ import annotations

from typing import Any

from agent.state import AgentState
from domain.models import MetricDataPoint
from domain.ports.observability_port import ObservabilityPort

# Defaults point at the stock_app target (its known CloudWatch log group and
# metric namespace). Overridable per-alert via the ``log_group`` /
# ``metric_namespace`` payload fields.
_DEFAULT_LOG_GROUP = "/ecs/stock-app"
_DEFAULT_NAMESPACE = "stock_app"
_SERVICE = "stock_app"
_DBS = ("target_app", "stock_db", "order_db")

# How far back to look when gathering evidence for a single incident.
_OBSERVE_WINDOW_MIN = 15


def _watched_metrics() -> list[tuple[str, dict[str, str], str]]:
    """The (metric_name, dimensions, statistic) triples to fetch.

    This is the stock_app watch-list: HTTP volume/errors/latency plus the
    per-database health signals that most directly indicate a real incident.
    """
    service = {"Service": _SERVICE}
    specs: list[tuple[str, dict[str, str], str]] = [
        ("Requests", service, "Sum"),
        ("RequestErrors", service, "Sum"),
        ("RequestLatency", service, "Average"),
    ]
    for db in _DBS:
        db_dims = {"Service": _SERVICE, "Database": db}
        # DatabaseUp is a 0/1 flag; the *latest* datapoint is what matters, so
        # the node reads ``latest`` from the summary (see _summarize_metric).
        specs.append(("DatabaseUp", db_dims, "Average"))
        specs.append(("DatabaseErrors", db_dims, "Sum"))
    return specs


def _summarize_metric(
    name: str, dimensions: dict[str, str], statistic: str, points: list[MetricDataPoint]
) -> dict[str, Any]:
    """Reduce a metric's data points to a compact, LLM-readable summary."""
    summary: dict[str, Any] = {"metric": name, "dimensions": dimensions}
    if not points:
        summary["latest"] = None
        summary["note"] = "no data in window"
        return summary
    values = [p.value for p in points]
    summary.update(
        {
            "statistic": statistic,
            "latest": values[-1],
            "avg": round(sum(values) / len(values), 3),
            "max": max(values),
            "points": len(values),
        }
    )
    return summary


def create_observe_node(observability: ObservabilityPort | None):
    """Return a node that stores ``raw_logs`` and ``metric_observations``."""

    def observe(state: AgentState) -> dict:
        if observability is None:
            return {"raw_logs": [], "metric_observations": []}

        log_group = state.get("log_group") or _DEFAULT_LOG_GROUP
        namespace = state.get("metric_namespace") or _DEFAULT_NAMESPACE

        raw_logs = observability.fetch_recent_logs(
            log_group=log_group, minutes=_OBSERVE_WINDOW_MIN
        )
        observations = [
            _summarize_metric(
                name,
                dims,
                stat,
                observability.get_metric(
                    namespace=namespace,
                    metric_name=name,
                    dimensions=dims,
                    minutes=_OBSERVE_WINDOW_MIN,
                    statistic=stat,
                ),
            )
            for name, dims, stat in _watched_metrics()
        ]
        return {"raw_logs": raw_logs, "metric_observations": observations}

    return observe
