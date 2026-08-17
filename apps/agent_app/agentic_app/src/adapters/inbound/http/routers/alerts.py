"""Alerts inbound adapter — ``POST /alerts`` webhook.

Accepts alert payloads from a real Grafana webhook or a synthetic demo
payload with the same schema. The graph is injected by the caller (real
adapters in ``main.py``, fakes in unit tests) — the router owns no I/O.
"""

from __future__ import annotations

import logging

from fastapi import APIRouter
from pydantic import BaseModel

logger = logging.getLogger(__name__)


class AlertPayload(BaseModel):
    """One inbound alert, whatever produced it."""

    source: str  # "grafana" | "synthetic" | ...
    incident_type: str
    summary: str
    severity: str | None = None
    # Where to fetch live evidence from. Optional: when omitted, the observe
    # node falls back to the stock_app defaults (/ecs/stock-app, stock_app).
    log_group: str | None = None
    metric_namespace: str | None = None


def create_alerts_router(graph) -> APIRouter:
    """Build the alerts router around a compiled agent graph.

    ``source`` is logged but never branched on — a synthetic demo alert
    and a real Grafana alert flow through the exact same path.
    """

    router = APIRouter()

    @router.post("/alerts")
    def receive_alert(payload: AlertPayload) -> dict:
        logger.info(
            "alert received from %s (incident_type=%s, severity=%s, log_group=%s)",
            payload.source,
            payload.incident_type,
            payload.severity,
            payload.log_group,
        )
        # No checkpointer is compiled into the graph, so there is no
        # thread_id to resume — each alert is a fresh invocation.
        # log_group / metric_namespace are optional; the observe node applies
        # the stock_app defaults when they are absent.
        initial_state = {
            "incident": payload.summary,
            "incident_type": payload.incident_type,
        }
        if payload.log_group is not None:
            initial_state["log_group"] = payload.log_group
        if payload.metric_namespace is not None:
            initial_state["metric_namespace"] = payload.metric_namespace
        return graph.invoke(initial_state)

    return router
