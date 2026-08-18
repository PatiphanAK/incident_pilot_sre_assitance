"""Shared state schema for the incident agent graph."""

from __future__ import annotations

from typing import TypedDict

from domain.models import ExecutionResult, IncidentMemory


class AgentState(TypedDict, total=False):
    """State threaded through the graph.

    - ``incident``: the raw incident report (input).
    - ``incident_type``: the alert's classified type, used to look up a
      runbook (free-text CLI runs leave it unset).
    - ``log_group`` / ``metric_namespace``: where to fetch observability
      evidence; optional, defaulting to the stock_app target in ``observe``.
    - ``past_incidents``: memories recalled by ``memory_check``.
    - ``raw_logs``: recent log lines fetched by ``observe``.
    - ``metric_observations``: compact per-metric summaries from ``observe``.
    - ``analysis``: the LLM's analysis produced by ``analyze``.
    - ``decision``: route chosen by ``decide`` — ``"run_runbook"`` or
      ``"escalate"``.
    - ``execution``: what the ``run_runbook`` branch did (simulated).
    - ``escalation``: the human-facing escalation message from the
      ``escalate`` branch.
    """

    incident: str
    incident_type: str
    log_group: str
    metric_namespace: str
    past_incidents: list[IncidentMemory]
    raw_logs: list[str]
    metric_observations: list[dict]
    analysis: str
    decision: str
    execution: ExecutionResult
    escalation: str
