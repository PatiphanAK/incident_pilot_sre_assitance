"""ObservabilityPort — read the target app's telemetry (CloudWatch).

The agent layer (``agent/nodes/*``) may only call this port. Concrete
backends (the CloudWatch/boto3 adapter, an in-memory fake) live in
``adapters/`` or ``tests/`` and are injected at graph build time — no node
may import boto3 directly (hexagonal rule).
"""

from __future__ import annotations

from typing import Protocol

from domain.models import MetricDataPoint


class ObservabilityPort(Protocol):
    """Read-only window onto the target service's logs and metrics."""

    def fetch_recent_logs(
        self,
        log_group: str,
        filter_pattern: str = "",
        minutes: int = 15,
    ) -> list[str]:
        """Return the raw log messages from ``log_group`` in the last ``minutes``.

        ``filter_pattern`` is passed through to the backend (an empty string
        means no filtering). Implementations must degrade gracefully: a
        missing log group, missing credentials, or a permission error yields
        an empty list, never an exception — the graph must stay usable when
        observability is unavailable.
        """
        ...

    def get_metric(
        self,
        namespace: str,
        metric_name: str,
        dimensions: dict[str, str],
        minutes: int = 15,
        statistic: str = "Average",
    ) -> list[MetricDataPoint]:
        """Return the data points for one metric over the last ``minutes``.

        ``dimensions`` selects the series (e.g. ``{"Service": "stock_app"}`` or
        ``{"Service": "stock_app", "Database": "stock_db"}``). Implementations
        degrade gracefully as in :meth:`fetch_recent_logs` — an error or an
        absent series returns an empty list.
        """
        ...
