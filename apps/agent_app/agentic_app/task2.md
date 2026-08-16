# Task: Runbook knowledge + Alerts inbound adapter

## Context
- RAG memory is done and proven (27/27 tests, live distance cutoff at 1.1).
- Graph currently: `triage -> recall_memory -> respond -> persist_memory`.
- Env vars use the `COCKROARCH_*` prefix (typo, but consistent everywhere —
  keep using it, do not "fix" it in this task).
- Hexagonal rule still applies: nodes only call ports, never adapters/drivers
  directly.

## Part A — `runbook_port`

1. **Port** — `domain/ports/runbook_port.py`
   ```python
   from typing import Protocol
   from domain.models.runbook import Runbook, ExecutionResult

   class RunbookPort(Protocol):
       def find_matching(self, incident_type: str) -> Runbook | None: ...
       def execute(self, runbook: Runbook, params: dict) -> ExecutionResult: ...
   ```

2. **Model** — `domain/models/runbook.py`
   ```python
   from pydantic import BaseModel

   class Runbook(BaseModel):
       id: str
       incident_pattern: str
       steps: list[str]
       blast_radius: str  # "low" | "medium" | "high"

   class ExecutionResult(BaseModel):
       status: str  # "simulated" | "executed" | "skipped"
       action: list[str]
   ```

3. **Schema** — create via the app's own psycopg connection (same pattern used
   for `observed_incidents`), not by guessing:
   ```sql
   CREATE TABLE runbooks (
       id UUID DEFAULT gen_random_uuid() PRIMARY KEY,
       incident_pattern STRING NOT NULL,
       steps JSONB NOT NULL,
       blast_radius STRING NOT NULL,
       created_at TIMESTAMPTZ DEFAULT now()
   );
   ```
   Seed 2-3 example rows for demo purposes (e.g. "connection pool exhaustion"
   -> low blast radius -> restart steps).

4. **Adapter** — `adapters/outbound/runbook/cockroachdb_runbook_adapter.py`
   - `find_matching`: simple `WHERE incident_pattern = %s` lookup for now
     (keyword match is fine here — the semantic matching already happened in
     `recall_memory`; this just fetches the known procedure).
   - `execute`: **simulate only** — do not call any real infra. Log the
     steps and return `ExecutionResult(status="simulated", action=runbook.steps)`.
     This is a deliberate scope decision for the hackathon — do not wire real
     remediation.

## Part B — `decide` node (confidence x blast radius)

Insert between `recall_memory` and the final action:

```
triage -> recall_memory -> decide -> run_runbook   (known + low/medium blast radius)
                                  -> escalate       (novel OR high blast radius)
```

- `decide` node logic (keep it simple, no ML):
  ```python
  def decide(state: AgentState, runbook: RunbookPort) -> AgentState:
      match = runbook.find_matching(state["incident_type"])
      has_prior = len(state["past_incidents"]) > 0
      if match and has_prior and match.blast_radius in ("low", "medium"):
          state["decision"] = "run_runbook"
      else:
          state["decision"] = "escalate"
      return state
  ```
- Use LangGraph conditional edges (`add_conditional_edges`) keyed on
  `state["decision"]`.
- `escalate` node just formats a message with `past_incidents` context —
  no new port needed, reuses `llm_port`.

## Part C — Alerts inbound adapter

`adapters/inbound/http/routers/alerts.py`:

```python
from fastapi import APIRouter
from pydantic import BaseModel

router = APIRouter()

class AlertPayload(BaseModel):
    source: str            # "grafana" | "synthetic"
    incident_type: str
    summary: str
    severity: str | None = None

@router.post("/alerts")
def receive_alert(payload: AlertPayload):
    initial_state = {
        "incident_summary": payload.summary,
        "incident_type": payload.incident_type,
    }
    result = graph.invoke(initial_state, config={"configurable": {"thread_id": ...}})
    return result
```

- Accept the same schema whether it comes from a real Grafana webhook or a
  synthetic payload sent during demo — do not branch logic on `source`
  except for logging.
- Wire this router into `adapters/inbound/http/main.py`.

## Definition of done
- `uv run pytest -m unit` — `decide` node tested with `FakeRunbookPort`
  covering both branches (run_runbook vs escalate).
- `POST /alerts` with a seeded `incident_pattern` returns a `run_runbook`
  decision with simulated steps in the response.
- `POST /alerts` with an unseen incident type returns an `escalate` decision.
- No adapter/driver imports under `agent/` or `domain/`.
