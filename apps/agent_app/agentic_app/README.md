# Task: Implement RAG-based long-term memory (CockroachDB)

## Context
- Project uses hexagonal architecture: `domain/`, `agent/` (LangGraph), `adapters/`
- CockroachDB Managed MCP Server is already connected — inspect the live
  cluster schema yourself via MCP before writing SQL. Do not guess column
  names or types.
- Goal: `recall_memory` and `persist_memory` nodes in `agent/nodes/` need a
  real RAG implementation backed by CockroachDB's distributed vector index.

## Constraints (do not violate)
- `agent/nodes/*` must only call `MemoryPort` (in `domain/ports/memory_port.py`).
  Never import a DB driver or embedding client directly inside a node.
- All CockroachDB access lives in
  `adapters/outbound/memory/cockroachdb_adapter.py`.
- Write a `FakeMemoryPort` in `tests/unit/fakes/` alongside the real adapter
  so agent nodes can be unit tested without a live DB connection.

## Steps

1. **Inspect the cluster via MCP** — confirm whether `observed_incidents`
   already exists. If not, create it:
   ```sql
   CREATE TABLE observed_incidents (
       id UUID DEFAULT gen_random_uuid() PRIMARY KEY,
       summary STRING NOT NULL,
       resolution STRING,
       embedding VECTOR(1536) NOT NULL,
       created_at TIMESTAMPTZ DEFAULT now(),
       VECTOR INDEX (embedding)
   );
   ```

2. **Pick and wire an embedding function** — expose it as
   `adapters/outbound/embedding/embedder.py` with a single function
   `embed(text: str) -> list[float]`. Use whichever provider is already
   configured as the primary LLM (keep it swappable — don't hardcode a
   provider inside the adapter).

3. **Implement `CockroachDBMemoryAdapter`** in
   `adapters/outbound/memory/cockroachdb_adapter.py`:
   - `find_similar(text: str, top_k: int = 3) -> list[dict]` — embed the
     query, run a `ORDER BY embedding <-> %s LIMIT %s` search, return
     `summary`, `resolution`, `distance`.
   - `save_incident(summary: str, resolution: str | None) -> None` —
     embed and insert a new row.
   - Use a connection pool (psycopg pool), not a new connection per call.

4. **Wire nodes**:
   - `agent/nodes/memory_check_node.py` → call `memory.find_similar(...)`,
     store result in `state["past_incidents"]`.
   - `agent/nodes/persist_memory_node.py` → call
     `memory.save_incident(...)` after the response is generated.

5. **Tests**:
   - Unit: run the graph with `FakeMemoryPort` (in-memory list, cosine
     similarity in pure Python) — no network calls.
   - Integration: run against the real CockroachDB cluster (via the same
     connection string the app uses, not MCP) and assert a seeded incident
     is retrievable by a semantically similar query.

## Definition of done
- `find_similar` returns relevant results for a paraphrased query (not
  exact string match) — prove RAG is actually semantic, not keyword search.
- No adapter or driver imports appear anywhere under `agent/` or `domain/`.
- Unit tests pass with zero live DB connections.

Instructions to connect via OAuth
```claude mcp add cockroachdb-cloud https://cockroachlabs.cloud/mcp --transport http --header "mcp-cluster-id: cbbc1b52-ad45-42d2-8ac6-0bb7f9088668"```

## Task 2 — runbook knowledge, decide node, alerts webhook

The graph gained a decision point after the reasoning step (LangGraph
conditional edges keyed on `state["decision"]`):

```
START → memory_check → analyze → decide ─┬→ run_runbook → persist_memory → END
                                          └→ escalate ───→ persist_memory → END
```

`decide` allows automation only when the incident is **known** (a runbook
matches `incident_type` AND long-term memory recalled a similar past
incident) **and** safe (runbook `blast_radius` is low/medium). Everything
else — novel incident or high blast radius — escalates to a human
(`escalate` formats the message via `llm_port`, no new port).

New pieces (hexagonal rule unchanged — nodes only call ports):

- `domain/ports/runbook_port.py` + `domain/models/runbook.py`
  (`Runbook`, `ExecutionResult`).
- `adapters/outbound/runbook/cockroachdb_runbook_adapter.py` — exact
  `incident_pattern` lookup; `execute` is **simulate-only** (logs steps,
  returns `status="simulated"`) — deliberate hackathon scope, no real
  remediation is wired.
- `runbooks` table — `migrations/003_create_runbooks.sql`, seeded with 3
  demo patterns (`connection pool exhaustion`/low, `disk usage high`/medium,
  `primary region outage`/high — the high one proves escalate-on-high).
- `POST /alerts` webhook — `adapters/inbound/http/` (FastAPI), same schema
  for Grafana and synthetic demo payloads; `source` is logged, never
  branched on.

Run the API:

```bash
cd apps/agent_app/agentic_app
uv run uvicorn adapters.inbound.http.main:app --port 8000
```

Demo the decision logic (memory starts empty, so the **first** alert of a
kind is novel → escalate; it is persisted, so the **second** recalls it and
runs the runbook):

```bash
curl -sX POST localhost:8000/alerts -H 'Content-Type: application/json' -d \
  '{"source":"grafana","incident_type":"connection pool exhaustion","summary":"payments service running out of database connections"}'
# → "decision": "escalate"

# same payload again →
# → "decision": "run_runbook", "execution": {"status": "simulated", "action": [...seeded steps...]}
```

Tests: `uv run --extra dev pytest -m unit` (fakes only, no I/O) ·
`uv run --extra dev pytest -m integration` (live cluster) · full suite:
`uv run --extra dev pytest`.

## Task 3 — observability tool (CloudWatch logs + metrics)

The agent can now **observe the target app directly**. A new `observe` node
(between `memory_check` and `analyze`) reads the stock app's live CloudWatch
telemetry through the new `ObservabilityPort` + `CloudWatchAdapter` (boto3)
and folds it into the LLM analysis — so the agent reasons over what the service
is *actually* doing, not just the alert text.

```
START → memory_check → observe → analyze → decide → run_runbook/escalate → persist_memory → END
```

- **Logs** — `fetch_recent_logs` (CloudWatch Logs `filter_log_events`), default
  group `/ecs/stock-app`.
- **Metrics** — `get_metric` (CloudWatch `get_metric_statistics`), namespace
  `stock_app`: `Requests`, `RequestErrors`, `RequestLatency`, plus per-database
  `DatabaseUp` / `DatabaseErrors`.
- Both are optional per-alert: `log_group` and `metric_namespace` on
  `POST /alerts` (the defaults above apply when omitted). The adapter degrades
  to empty evidence and **never raises** when credentials/permissions are
  missing, so the graph stays usable even with observability unavailable.
- Hexagonal rule intact: the node calls only the port; `boto3` lives solely in
  `adapters/outbound/observability/`. `boto3`/`botocore` are now forbidden under
  `agent/` and `domain/` (enforced by `test_architecture.py`).

### IAM (least privilege, read-only)

The identity that runs the agent needs only these, scoped where possible:

```json
{
  "Effect": "Allow",
  "Action": ["logs:FilterLogEvents", "cloudwatch:GetMetricStatistics"],
  "Resource": [
    "arn:aws:logs:ap-southeast-1:<account>:log-group:/ecs/stock-app:*",
    "arn:aws:cloudwatch:ap-southeast-1:<account>:metric:*"
  ]
}
```

Request `logs:FilterLogEvents` + `cloudwatch:GetMetricStatistics` only — do not
grant `logs:*` / `cloudwatch:*`.

### Local dev credentials

For local dev (not on Lambda/ECS with an IAM role), set the standard AWS env
vars — boto3 picks them up automatically via its default chain; there is no
custom credential-loading path:

```bash
export AWS_ACCESS_KEY_ID=... AWS_SECRET_ACCESS_KEY=... AWS_REGION=ap-southeast-1
```

### Demo (grounded analysis)

```bash
curl -sX POST localhost:8000/alerts -H 'Content-Type: application/json' -d \
  '{"source":"synthetic","incident_type":"disk usage high","summary":"order db slow",
    "log_group":"/ecs/stock-app","metric_namespace":"stock_app"}'
```

The response's `analysis` now cites the fetched logs/metrics (e.g. a
`DatabaseUp` that dropped to 0 or a `RequestErrors` spike).
