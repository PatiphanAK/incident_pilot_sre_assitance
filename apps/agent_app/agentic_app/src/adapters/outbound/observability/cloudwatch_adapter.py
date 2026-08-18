"""CloudWatch adapter for :class:`domain.ports.observability_port.ObservabilityPort`.

All CloudWatch access in the app lives here — no other layer may import
``boto3``. Two read-only calls back the port:

- ``logs.filter_log_events``  -> ``fetch_recent_logs``
- ``cloudwatch.get_metric_statistics`` -> ``get_metric``

**No region is hardcoded.** Clients are built from boto3's default
credential/region chain (``AWS_REGION`` / profile / IAM role), the same way
any other AWS SDK usage in this repo would be. The clients are created
lazily on first use, so merely constructing the adapter (or building the
graph) never touches the network and never requires credentials.

**Graceful degradation.** Every AWS failure — missing credentials, a log
group that doesn't exist, a permission denial, a network error — is
translated to an empty result (and a warning log) rather than an exception.
This keeps the agent graph runnable even when observability is
unavailable, matching the port contract.
"""

from __future__ import annotations

import logging
import time
from datetime import datetime, timedelta, timezone
from typing import Any, Callable

import botocore.exceptions

from domain.models import MetricDataPoint
from domain.ports.observability_port import ObservabilityPort

logger = logging.getLogger(__name__)

# Cap the number of log lines surfaced to the agent so an unbounded log
# group can't blow up the LLM prompt. ``filter_log_events`` returns events
# oldest-first, so keeping the tail keeps the *most recent* lines.
_MAX_LOG_LINES = 500

# Every AWS failure we want to degrade on: ``ClientError`` (API errors such as
# ResourceNotFoundException / AccessDenied) and ``BotoCoreError`` (missing
# credentials, network errors, ...). Note: in current botocore, ``ClientError``
# subclasses ``Exception`` directly and is NOT a ``BotoCoreError``, so both are
# listed explicitly.
_AWS_ERRORS = (botocore.exceptions.ClientError, botocore.exceptions.BotoCoreError)


class CloudWatchAdapter(ObservabilityPort):
    """CloudWatch-backed observability port (logs + metrics)."""

    def __init__(self, client_factory: Callable[[str], Any] | None = None) -> None:
        # ``client_factory`` exists purely for testability: unit tests inject a
        # fake returning canned clients. Production uses ``boto3.client``.
        self._client_factory = client_factory
        self._logs: Any = None
        self._cloudwatch: Any = None

    # -- lazy clients -------------------------------------------------------
    def _logs_client(self) -> Any:
        if self._logs is None:
            self._logs = (self._client_factory or _default_client)("logs")
        return self._logs

    def _cloudwatch_client(self) -> Any:
        if self._cloudwatch is None:
            self._cloudwatch = (self._client_factory or _default_client)("cloudwatch")
        return self._cloudwatch

    # -- ObservabilityPort --------------------------------------------------
    def fetch_recent_logs(
        self,
        log_group: str,
        filter_pattern: str = "",
        minutes: int = 15,
    ) -> list[str]:
        now_ms = int(time.time() * 1000)
        start_ms = now_ms - minutes * 60 * 1000
        kwargs: dict[str, Any] = {
            "logGroupName": log_group,
            "startTime": start_ms,
            "endTime": now_ms,
        }
        if filter_pattern:
            kwargs["filterPattern"] = filter_pattern
        try:
            resp = self._logs_client().filter_log_events(**kwargs)
        except _AWS_ERRORS as exc:
            # Covers API errors (ResourceNotFoundException, AccessDenied) and
            # credential/network errors — one catch degrades all of them to
            # "no data".
            logger.warning(
                "fetch_recent_logs(%s) failed, returning no logs: %s", log_group, exc
            )
            return []
        messages = [event.get("message", "") for event in resp.get("events", [])]
        return messages[-_MAX_LOG_LINES:]

    def get_metric(
        self,
        namespace: str,
        metric_name: str,
        dimensions: dict[str, str],
        minutes: int = 15,
        statistic: str = "Average",
    ) -> list[MetricDataPoint]:
        end = datetime.now(timezone.utc)
        start = end - timedelta(minutes=minutes)
        # Target up to ~20 data points across the window; period must be a
        # multiple of 60s and at least 60s.
        period = max(60, (minutes * 60) // 20)
        try:
            resp = self._cloudwatch_client().get_metric_statistics(
                Namespace=namespace,
                MetricName=metric_name,
                Dimensions=[{"Name": k, "Value": v} for k, v in dimensions.items()],
                StartTime=start,
                EndTime=end,
                Period=period,
                Statistics=[statistic],
            )
        except _AWS_ERRORS as exc:
            logger.warning(
                "get_metric(%s/%s) failed, returning no datapoints: %s",
                namespace,
                metric_name,
                exc,
            )
            return []
        points: list[MetricDataPoint] = []
        for dp in resp.get("Datapoints", []):
            if statistic not in dp:
                continue
            ts = dp["Timestamp"].strftime("%Y-%m-%dT%H:%M:%SZ")
            points.append(MetricDataPoint(timestamp=ts, value=float(dp[statistic])))
        return points


def _default_client(service: str) -> Any:
    """Create a boto3 client from the default credential/region chain.

    Imported lazily so module import (and graph construction) has no hard
    dependency on boto3 being importable at that exact moment — the SDK is a
    declared project dependency, but this keeps the adapter's import graph
    light and mirrors the memory adapter's lazy-connection philosophy.
    """
    import boto3

    return boto3.client(service)


def observability_from_env() -> CloudWatchAdapter:
    """Build the adapter using the standard AWS default chain.

    No custom credential-loading path: whatever credentials/region boto3
    would pick up on its own (env vars, profile, or the ambient IAM role on
    Lambda/ECS) is exactly what this uses.
    """
    return CloudWatchAdapter()
