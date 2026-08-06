# Incident Pilot SRE Assistance

AI-powered SRE incident investigation agent. The agent observes a production application, retrieves historical incident knowledge from **Shared AgeMem**, reasons over the incident with a foundation model (Claude / Nova via AWS Bedrock), inspects infrastructure through an **MCP** tool layer, and stores new operational knowledge for future incidents.

## Architecture

![Architecture](Architecture.png)

Request → execution path (V1):

```text
DevOps/SRE → Static Frontend → API Gateway → Main Backend (Lambda) → AWS SQS
  → ECS Fargate Agent Worker → Agent Orchestration
       ├── Foundation Model (Bedrock: Claude / Nova)
       ├── Persistent Memory (MCP)  → Agent State + Vector Index (Shared AgeMem)
       └── Agent Tooling (MCP)      → get_log()/get_metric()/get_health() → AWS SDK → Demo Target
```

Key decisions:
- **Lambda is not the Orchestrator** — it is a thin command-submission layer; the 5–20 min agent loop runs on ECS Fargate behind SQS.
- **MCP is the protocol boundary** for both tooling and memory — the LLM never touches AWS or memory directly.
- **Vector search is V1**, behind a `MemoryRepository` port (cost-efficient MVP impl, no managed vector DB).

## Design Patterns

- **Code:** Hexagonal Architecture (ports & adapters), organized by business boundary / bounded context.
- **Codebase:** Monorepo.

## Monorepo Layout

```text
aws_cockroach/
├── Readme.md                ← you are here (project overview)
├── Architecture.png         ← canonical architecture diagram
└── apps/
    ├── agent_app/           ← Incident Pilot AI Agent — the Main Backend (Agent platform)
    │   ├── README.md        ← agent app overview & navigation
    │   └── backend_run/     ← FastAPI runtime + bounded contexts
    │       └── README.md    ← detailed V1 architecture & design (source of truth)
    └── target_app/          ← Production Application (Demo Target) — observed by the Agent
```

The **Agent platform** (`agent_app`) and the **Demo Target** (`target_app`) are separate apps. The Agent reads the target's logs, metrics, and health read-only via the MCP tool layer + AWS SDK.

## Where to Start

- Detailed V1 design (request path, agent loop, MCP, memory/vector, failure handling, module layout, Mermaid diagrams): [`apps/agent_app/backend_run/README.md`](apps/agent_app/backend_run/README.md)
- Agent app overview: [`apps/agent_app/README.md`](apps/agent_app/README.md)

> Status: **design only.** No production code is implemented yet — the READMEs are the source of truth for the implementation phase.
