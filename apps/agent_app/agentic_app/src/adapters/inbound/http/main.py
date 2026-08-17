"""FastAPI app — HTTP entrypoint hosting the alerts webhook.

Like the CLI, this is one of the two places concrete adapters are wired
together (dependency injection lives here, not in agent or domain)::

    cd apps/agent_app/agentic_app
    uv run uvicorn adapters.inbound.http.main:app --port 8000
"""

from __future__ import annotations

from pathlib import Path

from dotenv import load_dotenv
from fastapi import FastAPI

from adapters.inbound.http.routers.alerts import create_alerts_router
from adapters.outbound.llm.openai_compatible_llm import OpenAICompatibleLLM
from adapters.outbound.memory.cockroachdb_adapter import memory_from_env
from adapters.outbound.observability.cloudwatch_adapter import observability_from_env
from adapters.outbound.runbook.cockroachdb_runbook_adapter import runbook_from_env
from agent.graph import build_graph

load_dotenv(Path(__file__).resolve().parents[4] / ".env")


def create_app() -> FastAPI:
    """Build the app with the real (env-configured) port adapters."""
    app = FastAPI(title="Incident Pilot — agentic app")
    graph = build_graph(
        llm=OpenAICompatibleLLM(),
        memory=memory_from_env(),
        runbook=runbook_from_env(),
        observability=observability_from_env(),
    )
    app.include_router(create_alerts_router(graph))
    return app


app = create_app()
