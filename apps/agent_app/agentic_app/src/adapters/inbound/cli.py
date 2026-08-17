"""CLI entrypoint — run the incident agent on a single incident report.

This is the only place the concrete adapters are wired together
(dependency injection lives here, not in the agent or domain layers)::

    cd apps/agent_app/agentic_app
    uv run agentic-app "checkout latency spiked on the payments service"
"""

from __future__ import annotations

import sys
from pathlib import Path

from dotenv import load_dotenv

from adapters.outbound.llm.openai_compatible_llm import OpenAICompatibleLLM
from adapters.outbound.memory.cockroachdb_adapter import memory_from_env
from adapters.outbound.observability.cloudwatch_adapter import observability_from_env
from adapters.outbound.runbook.cockroachdb_runbook_adapter import runbook_from_env
from agent.graph import build_graph


def _load_env() -> None:
    load_dotenv(Path(__file__).resolve().parents[3] / ".env")


def _print_live_telemetry(result: dict) -> None:
    """Print the CloudWatch evidence the observe node gathered (if any)."""
    metrics = result.get("metric_observations") or []
    logs = result.get("raw_logs") or []
    if not metrics and not logs:
        return
    print("Live telemetry (CloudWatch, recent window):")
    for obs in metrics:
        dims = obs.get("dimensions", {})
        dim_str = "/".join(f"{k}={v}" for k, v in dims.items())
        if obs.get("latest") is None:
            print(f"  - {obs['metric']} ({dim_str}): no data")
        else:
            print(f"  - {obs['metric']} ({dim_str}): latest={obs['latest']}")
    if logs:
        print(f"  - recent log lines: {len(logs)}")
    print()


def main(argv: list[str] | None = None) -> int:
    _load_env()
    args = sys.argv[1:] if argv is None else argv
    if not args:
        print('usage: agentic-app "<incident report>"', file=sys.stderr)
        return 2

    incident = " ".join(args)
    graph = build_graph(
        llm=OpenAICompatibleLLM(),
        memory=memory_from_env(),
        runbook=runbook_from_env(),
        observability=observability_from_env(),
    )

    print(f"Analyzing incident: {incident}\n")
    # CLI input is free text with no incident_type, so decide() finds no
    # runbook match and the graph always escalates — the decision support
    # (analysis + memory) is still produced below.
    result = graph.invoke({"incident": incident})

    past = result.get("past_incidents") or []
    if past:
        print("Relevant past incidents:")
        for mem in past:
            print(f"  - [{mem.distance:.4f}] {mem.summary}")
            if mem.resolution:
                print(f"      resolution: {mem.resolution}")
        print()
    _print_live_telemetry(result)
    print("Analysis:")
    print(result["analysis"])
    if result.get("decision") == "run_runbook":
        execution = result["execution"]
        print(f"\nDecision: run_runbook ({execution.status})")
        for i, step in enumerate(execution.action, start=1):
            print(f"  {i}. {step}")
    else:
        print("\nDecision: escalate to human operator")
        print(result["escalation"])
    print("\n[memory] incident persisted for future recall.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
