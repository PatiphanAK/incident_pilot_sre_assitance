"""Domain model for a single CloudWatch metric data point.

Plain frozen dataclass (like :class:`IncidentMemory`) — it's an internal
transfer object from the observability adapter to the agent nodes, so it
carries no infrastructure types and stays importable without the AWS SDK.
"""

from __future__ import annotations

from dataclasses import dataclass


@dataclass(frozen=True)
class MetricDataPoint:
    """One point in a CloudWatch metric's time series.

    ``timestamp`` is an ISO-8601 UTC string (e.g. ``2026-08-18T00:00:00Z``)
    rather than an epoch integer, so nodes can read it without importing
    ``datetime`` plumbing and the LLM-facing prompt stays human-readable.
    ``value`` is the statistic value (e.g. an average latency in ms or a
    0/1 up-flag) — units are the caller's concern, not the model's.
    """

    timestamp: str
    value: float
