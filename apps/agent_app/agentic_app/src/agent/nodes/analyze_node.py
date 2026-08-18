"""analyze node — reason over the incident with recalled context + live telemetry."""

from __future__ import annotations

from domain.models import IncidentMemory
from domain.ports.llm_port import LLMPort

from agent.state import AgentState

_SYSTEM_PROMPT = (
    "You are an SRE incident-analysis agent. Analyze the reported "
    "incident: identify the likely root cause, list concrete next "
    "investigation steps, and cite any relevant past incidents or live "
    "telemetry that are provided. Be concise and actionable."
)

# Cap how many raw log lines make it into the prompt (the adapter already
# caps at 500; this is a second, prompt-shaping bound).
_MAX_LOG_LINES_IN_PROMPT = 80


def _format_past_incidents(past: list[IncidentMemory]) -> str:
    if not past:
        return "No relevant past incidents found in long-term memory."
    lines = []
    for i, mem in enumerate(past, start=1):
        lines.append(f"{i}. {mem.summary}")
        if mem.resolution:
            lines.append(f"   resolution: {mem.resolution}")
    return "\n".join(lines)


def _format_metrics(observations: list[dict]) -> str:
    """Render the compact metric summaries fetched by ``observe``."""
    if not observations:
        return ""
    lines = []
    for obs in observations:
        dims = obs.get("dimensions", {})
        dim_str = "/".join(f"{k}={v}" for k, v in dims.items())
        if obs.get("latest") is None:
            lines.append(f"- {obs['metric']} ({dim_str}): no data in window")
        else:
            latest = obs["latest"]
            extra = ""
            if "avg" in obs:
                extra = f", avg={obs['avg']}, max={obs['max']}"
            lines.append(f"- {obs['metric']} ({dim_str}): latest={latest}{extra}")
    return "\n".join(lines)


def _format_logs(logs: list[str]) -> str:
    """Render the most recent raw log lines fetched by ``observe``."""
    if not logs:
        return ""
    recent = logs[-_MAX_LOG_LINES_IN_PROMPT:]
    return "\n".join(recent)


def create_analyze_node(llm: LLMPort):
    """Return a node that produces ``state["analysis"]`` via the LLM."""

    def analyze(state: AgentState) -> dict:
        parts = [
            _SYSTEM_PROMPT,
            "",
            f"Reported incident:\n{state['incident']}",
            "",
            "Relevant past incidents from long-term memory:",
            _format_past_incidents(state.get("past_incidents") or []),
        ]
        metrics_text = _format_metrics(state.get("metric_observations") or [])
        if metrics_text:
            parts += ["", "Live CloudWatch metrics (recent window):", metrics_text]
        logs_text = _format_logs(state.get("raw_logs") or [])
        if logs_text:
            parts += ["", "Recent application logs:", logs_text]
        return {"analysis": llm.generate("\n".join(parts))}

    return analyze
