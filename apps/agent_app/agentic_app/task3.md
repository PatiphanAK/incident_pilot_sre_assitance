Task: Observability tool — CloudWatch Logs adapter
Context
RAG memory (memory_port) and runbook (runbook_port) are done.
Graph currently: triage -> recall_memory -> decide -> run_runbook/escalate -> persist_memory.
triage currently classifies from whatever text is in the alert payload only — it has no real log context yet. This task fixes that.
Target app's logs live in CloudWatch Logs, not Grafana. Grafana is only the dashboard layer for humans; the agent should read CloudWatch directly via boto3 — no Grafana API dependency.
Hexagonal rule still applies: triage node calls ObservabilityPort only, never boto3 directly.
Part A — observability_port

domain/ports/observability_port.py:

python
from typing import Protocol

class ObservabilityPort(Protocol):
    def fetch_recent_logs(
        self,
        log_group: str,
        filter_pattern: str = "",
        minutes: int = 15,
    ) -> list[str]: ...
Part B — CloudWatchAdapter

adapters/outbound/observability/cloudwatch_adapter.py:

Use boto3.client("logs"). Do not hardcode a region — read it from the standard AWS env vars / default boto3 chain, same as any other AWS SDK usage in this repo.
Implement fetch_recent_logs using filter_log_events:
startTime = now - minutes (in ms since epoch)
endTime = now (in ms since epoch)
Return just the list of message strings from events.
Handle ResourceNotFoundException (log group doesn't exist) by returning an empty list rather than raising — triage should degrade gracefully, not crash the graph.
Add a FakeObservabilityPort in tests/unit/fakes/ that returns a fixed list of log lines, for unit testing triage without AWS calls.
Part C — Wire into triage node

Update agent/nodes/triage_node.py:

python
def triage(state: AgentState, observability: ObservabilityPort, llm: LLMPort) -> AgentState:
    logs = observability.fetch_recent_logs(
        log_group=state["log_group"],
        filter_pattern=state.get("filter_pattern", ""),
    )
    state["raw_logs"] = logs
    classification_input = "\n".join(logs) if logs else state["incident_summary"]
    state["incident_type"] = llm.classify(classification_input)
    return state
state["log_group"] should come from the alert payload (add it to AlertPayload in adapters/inbound/http/routers/alerts.py — required field, no default, since without it there's nothing to fetch).
If logs is empty, fall back to incident_summary from the alert itself rather than failing — keeps the graph usable even before CloudWatch is fully wired for a given demo environment.
Part D — IAM / config
Document required permission in README.md: logs:FilterLogEvents (read-only, scoped to the specific log group if possible — do not request logs:*).
If running locally for dev (not on Lambda/ECS with an IAM role), note that standard AWS credential env vars (AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, AWS_REGION) are picked up by boto3 automatically — do not invent a custom credential-loading path.

Definition of done
uv run pytest -m unit — triage tested with FakeObservabilityPort, asserting incident_type classification uses raw_logs when present.
A test for the empty-log-group case (ResourceNotFoundException) confirms the graph still completes instead of raising.
No boto3 import anywhere under agent/ or domain/.
README.md lists the IAM permission needed.
