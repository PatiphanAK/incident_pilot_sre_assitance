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
from agent.graph import build_graph


def _load_env() -> None:
    load_dotenv(Path(__file__).resolve().parents[3] / ".env")


def main(argv: list[str] | None = None) -> int:
    _load_env()
    args = sys.argv[1:] if argv is None else argv
    if not args:
        print('usage: agentic-app "<incident report>"', file=sys.stderr)
        return 2

    incident = " ".join(args)
    graph = build_graph(llm=OpenAICompatibleLLM(), memory=memory_from_env())

    print(f"Analyzing incident: {incident}\n")
    result = graph.invoke({"incident": incident})

    past = result.get("past_incidents") or []
    if past:
        print("Relevant past incidents:")
        for mem in past:
            print(f"  - [{mem.distance:.4f}] {mem.summary}")
            if mem.resolution:
                print(f"      resolution: {mem.resolution}")
        print()
    print("Analysis:")
    print(result["analysis"])
    print("\n[memory] incident persisted for future recall.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
