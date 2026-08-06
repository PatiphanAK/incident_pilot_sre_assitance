# Incident Pilot Backend — Runtime & Design Spec

This is the **application runtime** for the Incident Pilot agent (`apps/agent_app/backend_run`): a Python/FastAPI service that hosts the agent's bounded contexts — the execution loop, the MCP boundary, Shared AgeMem, and the agent tools.

> **V1 design and architecture specification.** No production code is implemented yet — this document is the source of truth for this runtime's design.
>
> For the **project-wide overview** (the full request path, the two tiers, the monorepo layout), see the root [`Readme.md`](../../../Readme.md) and [`apps/agent_app/README.md`](../README.md). This file does not repeat that context — it specifies the runtime itself.

## Scope of This Runtime

This runtime owns the **agent-side** execution components:

- the **Agent Orchestration** loop and its adapters (Bedrock, SQS consumer, CockroachDB run state),
- the **MCP layer** (client + server) that mediates all tool and memory access,
- **Agent Tooling** (`get_log()`, `get_metric()`, `get_health()`),
- **Shared AgeMem** (Persistent Memory: Agent State + Vector Index).

> **Out of scope here** — owned elsewhere in the platform: the Static Frontend, API Gateway, and the thin Main Backend Lambda. These are described in [`apps/agent_app/README.md`](../README.md). The **Production Application (Demo Target)** in `apps/target_app` is a separate app this runtime observes read-only.

## Core Agent Loop

```text
Observe → Retrieve → Reason → Tool Call → Observe → ... → Store
```

A single investigation may last **5–20 minutes** and involve many Bedrock calls and many tool calls. Every design decision below exists to support that reality — which is why the loop runs on ECS Fargate behind SQS, not on Lambda.

## Stack (this runtime)

- **Python** + **FastAPI** — application runtime
- **AWS Bedrock** — foundation model reasoning (Claude / Nova)
- **Amazon SQS** — async execution boundary (worker consumes run jobs)
- **Amazon ECS Fargate** — long-running Agent Worker host
- **CockroachDB** — persistent application state (Agent State + relational memory)
- **Vector Index** — semantic retrieval for Shared AgeMem (cost-efficient V1 choice, behind a port)
- **MCP (Model Context Protocol)** — protocol boundary for tooling and memory access
- **boto3** — AWS SDK for read-only observation of the Demo Target
- **Docker** — local dev and container build

> Cost discipline: no standalone managed vector DB, no extra orchestration or streaming services. See *Cost & MVP Posture*.

## Critical Design Point: the Loop Does Not Run on Lambda

> **Lambda is an API/command submission layer, not the Agent Orchestrator.** The Lambda is described in [`apps/agent_app/README.md`](../README.md); this runtime is the long-running side.

Agent execution is long-running and dynamic. A single run performs many Bedrock calls and tool calls, often taking 5–20 minutes. Lambda's invocation lifecycle and 15-min hard cap are incompatible with that, so the loop lives here on ECS Fargate, decoupled from the request lifecycle via SQS. Execution time is bounded by the run, not by Lambda.

This runtime's worker:

- consume Agent Run jobs from SQS
- load persistent **Agent State** from CockroachDB
- execute the Agent loop via Agent Orchestration
- call Bedrock
- execute tools **through the MCP layer**
- retrieve and store memories **through the MCP layer**
- update run status
- handle failures and retries

The worker is **stateless at the process level** — all persistent state lives in CockroachDB, so any container can be replaced or scaled without losing a run.

Run statuses (written by this runtime): `queued · running · waiting_for_tool · completed · failed · cancelled`

## Agent Orchestration

An application-level component that owns the Agent execution loop. It is **not a fixed pipeline** — the foundation model decides the next tool dynamically via tool calling. There is no hardcoded sequence such as `CloudWatch Logs → Metrics → EC2`.

```text
Load Run (Agent State)
  ↓
Retrieve Relevant Memory      (via MCP → Persistent Memory)
  ↓
Observe System
  ↓
Reason with Bedrock
  ↓
Does the model request a tool?
  ├── Yes → Validate Tool Call → Execute Tool (via MCP) → Persist Tool Result → Reason Again (loop)
  └── No  → Generate Final Diagnosis → Store Useful Memory → Mark Run Completed
```

## MCP: the Protocol Boundary

> **MCP is a V1 architecture decision, not aspirational.**

The Agent Orchestrator talks to two kinds of capability through **MCP (Model Context Protocol)**: tools and memory. The LLM **must not** access AWS infrastructure or memory storage directly — every external interaction is mediated by the MCP layer.

```text
Agent Orchestration
   │  (MCP)
   ├──► Agent Tooling          → get_log(), get_metric(), get_health() → AWS SDK → Demo Target
   └──► Persistent Memory      → Agent State · Vector Index (Shared AgeMem)
```

Why MCP:

- **one protocol boundary** for both tools and memory — consistent discovery, schema, authorization, and audit.
- **isolation** — the orchestrator depends on MCP interfaces, never on AWS SDKs or DB drivers directly.
- **swappability** — tools and memory backends can be added, replaced, or mocked behind the same protocol.

V1 MCP surface (concrete tools exposed through the layer):

```text
get_log()      — retrieve logs from the Production Application (Demo Target)
get_metric()   — retrieve metrics from the Demo Target
get_health()   — check service health of the Demo Target
```

The tool layer enforces, for every MCP tool call:

- **explicit input schemas** — validated before execution.
- **validation** — reject malformed or unsafe arguments.
- **authorization** — each call checked against allowed scope.
- **timeout handling** — no tool may hang the run indefinitely.
- **error handling** — failures return structured results to the LLM, not crashes.
- **audit logging** — every invocation recorded in `tool_call_logs`.

> Additional tools (e.g. `RunbookTool`, `AWSResourceInspectionTool`, `MemorySearchTool`) are **future/optional** and slot in as additional MCP-exposed tools without redesign.

## Persistent Memory (Shared AgeMem)

Persistent Memory is the **Shared AgeMem** — long-term incident knowledge carried across runs and sessions, plus the Agent State of in-flight runs. It is accessed by the orchestrator **via MCP**.

Initial relational data model (CockroachDB):

```text
agents
agent_sessions
agent_runs
session_messages
tool_call_logs
memory_entries
memory_links
```

Concepts:

- **session** — conversation context.
- **run** — one Agent execution within a session.
- **memory** — long-term knowledge carried across runs and sessions.
- **tool_call_log** — tool execution / audit history.
- **Agent State** — the persisted, resumable state of a run (loop position, last observation, partial trace).

`memory_links` is a **self-referencing** relationship between `memory_entries`, typed:

```text
Memory A
  ├── related_to ──→ Memory B
  ├── derived_from ─→ Memory C
  ├── contradicts ──→ Memory D
  └── supersedes ──→ Memory E
```

### Vector Index (committed for V1)

> **Vector search is a V1 commitment**, not deferred.

Persistent Memory includes a **vector index** for semantic retrieval. Each `memory_entry` carries an embedding populated at write time; retrieval is similarity-based over those embeddings.

Cost-efficient V1 approach — **avoid an expensive managed vector database**:

- The **storage and retrieval implementation is hidden behind an abstraction** (a `MemoryRepository` / vector-store port in the memory domain).
- V1 default implementation: store embeddings **alongside the relational data** (e.g. embeddings in CockroachDB or a lightweight local index) and perform similarity search in-process. This is sufficient for an MVP/demo corpus and avoids a separate managed vector DB.
- Because the implementation sits behind a port, it can be **swapped later** (e.g. to pgvector, an in-memory ANN index, or a managed store) without changing the domain model or the MCP surface.

> The choice of the *specific* vector backend is intentionally the one place left flexible — but a vector index of *some* kind is part of V1.

## Failure Handling and Resume

This runtime assumes the ECS worker can crash at any time. Therefore **all Agent execution state is persisted in CockroachDB** (Agent State), and SQS provides retry and recovery.

| Failure                  | Handling                                                                                          |
| ------------------------ | ------------------------------------------------------------------------------------------------- |
| Worker crash             | Run state is in CockroachDB. SQS redelivers the job; a new worker loads state and resumes.         |
| Bedrock timeout          | Retry with backoff; after N attempts the run is marked `failed` with the partial trace intact.    |
| Tool timeout             | MCP tool returns a structured timeout result; the LLM may retry or pick another tool.             |
| Tool failure             | Structured error returned to the LLM via MCP; logged in `tool_call_logs`. Does not crash the run. |
| Duplicate SQS delivery   | Idempotent by `run_id`; a run already `running` is skipped.                                       |
| Partial Agent execution  | Each step (observation, tool result, memory write) is persisted before the next; a crash mid-loop loses at most one in-flight step. |
| Retry                    | Bounded retries with backoff; persistent progress prevents re-running completed steps.            |
| Idempotency              | `run_id` is the dedup key; tool calls are idempotent by `(run_id, step)` and de-duplicated.       |
| Failed runs              | Marked `failed`; partial trace + diagnosis retained in CockroachDB for review and re-run.         |

Goal: recover an Agent Run without losing persistent state. The run is resumable because every transition is recorded before it is acted on.

## Security Model

Incident Pilot V1 is **read-only and recommendation-first.**

The Agent may:

- inspect CloudWatch / logs / metrics / health of the Demo Target
- inspect AWS resources
- retrieve runbooks
- retrieve historical incidents (Shared AgeMem)
- analyze incidents
- provide recommendations

The Agent should **not** automatically perform destructive remediation.

Principles:

- **least-privilege IAM**
- **read-only AWS permissions by default**
- **strict tool input validation** at the MCP layer
- **explicit tool authorization** — each MCP call checked against allowed scope
- **audit logging** — every tool invocation recorded
- **no destructive actions by default**
- **human approval required** before any future remediation

The architecture is designed so **remediation capabilities can be added later without redesign** — they slot in as additional MCP-exposed tools behind the same authorization gate, with a human-approval port that stays a no-op in V1.

## Module Layout (this runtime)

Hexagonal architecture, organized by **business boundary**, not by technical layer. The `src/` directories exist but are not yet populated; this layout is the target.

```text
src/
├── agent/            # execution loop, orchestration, MCP boundary
│   ├── domain/       # ports: LLMPort, Tool/MCP, Memory, RunState (no AWS/DB/LLM imports)
│   ├── application/  # use cases: orchestrate run, execute tool, persist state
│   └── infrastructure/ # adapters: Bedrock, SQS consumer, CockroachDB run state,
│                      #            MCP client + server (tool & memory)
│
├── incident/         # incident context: what an incident is, runbooks
│   ├── domain/
│   ├── application/
│   └── infrastructure/
│
├── memory/           # Shared AgeMem: entries, links, vector retrieval + storage
│   ├── domain/       # MemoryRepository port (vector store abstraction)
│   ├── application/
│   └── infrastructure/ # CockroachDB relational store + V1 vector index impl
│
├── observability/    # CloudWatch logs/metrics/health as a tool data source
│   ├── domain/
│   ├── application/
│   └── infrastructure/
│
└── ...               # additional bounded contexts as needed
```

**Avoid** organizing the entire application by technical layer:

```text
# Do NOT do this
services/
repositories/
models/
utils/
```

Module responsibilities:

- **agent** — the execution loop, orchestrator, and MCP boundary (the central bounded context). `domain` defines ports (LLM, Tool/MCP, Memory, RunState); `application` drives the loop; `infrastructure` implements adapters (Bedrock, SQS, MCP client/server, CockroachDB run state).
- **incident** — incident entities and runbook knowledge. Owns what "an incident" means.
- **memory** — Shared AgeMem: `memory_entries` + `memory_links`, vector retrieval and storage. Owns long-term knowledge and the vector-store port.
- **observability** — CloudWatch logs/metrics/health as the tool data source for the Demo Target. Owns the "observe" side of the loop.

Dependency rule: `domain` depends on nothing outward; `application` depends on `domain`; `infrastructure` implements `domain` ports. This keeps the Agent loop testable without AWS, Bedrock, or a real vector store.

## Cost & MVP Posture

Optimized for **low AWS cost** and a **practical MVP/demo**:

- **No extra managed services.** Specifically no standalone managed vector DB — embeddings live alongside relational data behind a port in V1.
- **ECS Fargate** sized for a small worker count; queue depth governs concurrency so we don't pay for idle capacity.
- **CockroachDB** serves both run state and relational memory in one store.
- **Vector index** uses the cheapest sufficient backend for a demo corpus, swappable later.

## Local Development

```text
uv run fastapi dev
```

## Scope — Not Built Yet

This runtime is **design only** today. Not yet implemented:

- Bounded-context source under `src/` (the module layout above is the target)
- ECS Fargate worker + SQS consumer
- MCP layer (client + server for tools and memory)
- Agent Tooling (`get_log()`, `get_metric()`, `get_health()`)
- Shared AgeMem: relational store + V1 vector index
- Bedrock integration
- CockroachDB schema + migrations
- Docker production config

> Future / optional (not V1): remediation tools, additional MCP tools, managed vector DB migration, human-approval-gated actions.

**Suggested first milestone:** `agent` + `memory` bounded contexts running locally with mocked Bedrock and an in-memory vector store — proves the orchestration loop and MCP boundary before any AWS infra.

## Execution-Time Guarantee

This runtime supports a single Agent execution of **5–20 minutes** with many Bedrock calls and many tool calls, independent of any Lambda timeout: the run executes on ECS Fargate for as long as needed, and every step is persisted (Agent State) so it survives crashes and resumes via SQS redelivery.
