# Incident Pilot SRE Assistance AI Agent

This is the **Main Backend** of Incident Pilot — the AI agent platform that investigates production incidents. It observes a target system, retrieves historical incident knowledge from **Shared AgeMem**, reasons over the incident with a foundation model (Claude / Nova via AWS Bedrock), inspects infrastructure through an **MCP** tool layer, and stores new operational knowledge for future incidents.

> This app is the **Agent platform** (the upper tier in `Architecture.png`). It is deliberately separate from the system it investigates — the **Production Application (Demo Target)** in [`apps/target_app`](../target_app). The Agent only *observes* the target; the target contains no agent code.

## What This App Contains

```text
apps/agent_app/                 ← you are here (Main Backend)
├── README.md                   ← this file: platform overview & navigation
└── backend_run/                ← application runtime (FastAPI)
    ├── README.md               ← detailed V1 architecture & design (source of truth)
    ├── main.py                 ← FastAPI entrypoint (local dev)
    ├── pyproject.toml          ← Python deps (uv): fastapi, boto3
    └── src/                    ← bounded contexts (planned, see below)
```

- **`backend_run/`** — the Python/FastAPI application runtime. This is where the Agent's bounded contexts live (orchestration, MCP, memory, tools). Its [README](backend_run/README.md) is the **detailed V1 design spec** — request path, agent loop, MCP boundary, memory/vector index, failure handling, module layout.
- **Planned (not yet built)** — the API Gateway + Lambda command layer, the Static Frontend, and the ECS Fargate worker image are part of this platform but not yet scaffolded here. See *What is not built yet*.

## Where It Fits (Monorepo)

```text
aws_cockroach/                  ← monorepo (Hexagonal Architecture, mono repo)
├── Readme.md                   ← project-wide overview
├── Architecture.png            ← the canonical architecture diagram
└── apps/
    ├── agent_app/              ← THIS — Incident Pilot AI Agent (Main Backend)
    │   └── backend_run/        ← FastAPI runtime + bounded contexts
    └── target_app/             ← Production Application (Demo Target) — observed, not owned
```

The Agent platform and the Demo Target are **two separate apps** in the same monorepo. The Agent crosses the boundary read-only, via the MCP tool layer and the AWS SDK, to read the target's logs, metrics, and health.

## Architecture at a Glance

```text
DevOps/SRE
  ↓
Static Frontend
  ↓ (REST / HTTPS)
API Gateway                       ← public API boundary
  ↓
Main Backend (Lambda)             ← thin command-submission layer (this app)
  ↓ (HTTPS)
AWS SQS                           ← async execution boundary
  ↓ (HTTPS)
ECS Fargate Agent Worker          ← long-running worker (this app)
  ↓ (MCP)
Agent Orchestration
  ├── Foundation Model (Bedrock: Claude / Nova)
  ├── Persistent Memory (MCP)     ← Shared AgeMem: Agent State + Vector Index
  └── Agent Tooling (MCP)         ← get_log() / get_metric() / get_health()
        └── AWS SDK → apps/target_app   (read-only observation)
```

**Three load-bearing decisions (V1):**

1. **Lambda is not the Orchestrator** — Lambda returns `202` in seconds; the 5–20 min agent loop runs on ECS Fargate behind SQS.
2. **MCP is the one protocol boundary** — the LLM never touches AWS or memory directly; both tooling and memory are MCP-mediated, validated, authorized, and audited.
3. **Vector search is V1** — but the vector backend is hidden behind a `MemoryRepository` port (cheap MVP impl, no managed vector DB).

> Full detail — request path, agent loop, MCP surface, data model, failure/resume table, security model, DDD module layout, Mermaid diagrams — lives in [`backend_run/README.md`](backend_run/README.md). That file is the authoritative design spec.

## Planned Bounded Contexts (`backend_run/src/`)

Hexagonal architecture, organized by business boundary (not by technical layer). The directories exist but are not yet populated; this layout is the target.

```text
src/
├── agent/            # execution loop, orchestration, MCP boundary
│   ├── domain/       # ports: LLMPort, Tool/MCP, Memory, RunState (no AWS/DB/LLM imports)
│   ├── application/  # use cases: orchestrate run, execute tool, persist state
│   └── infrastructure/ # adapters: Bedrock, SQS consumer, CockroachDB, MCP client+server
├── incident/         # what an incident is, runbooks
├── memory/           # Shared AgeMem: entries, links, vector retrieval + storage
├── observability/    # CloudWatch logs/metrics/health as a tool data source
└── ...
```

## Local Development

```text
cd apps/agent_app/backend_run
uv run fastapi dev
```

## What Is Not Built Yet

This app is **design only** today. Not yet implemented:

- Bounded-context source under `backend_run/src/` (the module layout above is the target)
- API Gateway + Main Backend Lambda (command layer)
- Static Frontend (DevOps/SRE UI)
- ECS Fargate worker image + deployment
- MCP layer (client + server for tools and memory)
- Agent Tooling (`get_log()`, `get_metric()`, `get_health()`)
- Shared AgeMem: relational store + V1 vector index
- Bedrock integration
- CockroachDB schema + migrations
- Terraform, Docker production config

> Future / optional (not V1): remediation tools, additional MCP tools, managed vector DB migration, human-approval-gated actions.

**Suggested first milestone:** local-dev loop with `agent` + `memory` bounded contexts, mocked Bedrock, and an in-memory vector store — proves the orchestration loop and MCP boundary before any AWS infra.

## Source of Truth

- [`backend_run/README.md`](backend_run/README.md) — detailed V1 architecture & design.
- [`Architecture.png`](../../Architecture.png) (repo root) — canonical diagram this app implements.
- [`/Readme.md`](../../Readme.md) (repo root) — monorepo overview.
