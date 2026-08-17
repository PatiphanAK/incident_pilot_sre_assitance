"""Incident agent graph — wires the nodes together.

    START → memory_check → observe → analyze → decide ─┬→ run_runbook → persist_memory → END
                                                      └→ escalate ───────────────────────→ persist_memory → END

``decide`` routes on confidence x blast radius: a known incident (matching
runbook + prior memory) with low/medium blast radius runs the runbook,
anything else escalates to a human.

``observe`` gathers the target app's live CloudWatch telemetry (logs +
metrics) so ``analyze`` reasons over real evidence, not just the alert text.
It is a no-op (empty evidence) when no ``observability`` port is supplied, so
every existing caller keeps working unchanged.

Ports are injected at build time (dependency injection), so the same
graph runs against the real adapters or the in-memory fakes.
"""

from __future__ import annotations

from langgraph.graph import END, START, StateGraph

from agent.nodes.analyze_node import create_analyze_node
from agent.nodes.decide_node import create_decide_node
from agent.nodes.escalate_node import create_escalate_node
from agent.nodes.memory_check_node import create_memory_check_node
from agent.nodes.observe_node import create_observe_node
from agent.nodes.persist_memory_node import create_persist_memory_node
from agent.nodes.run_runbook_node import create_run_runbook_node
from agent.state import AgentState
from domain.ports.llm_port import LLMPort
from domain.ports.memory_port import MemoryPort
from domain.ports.observability_port import ObservabilityPort
from domain.ports.runbook_port import RunbookPort


def _route_after_decide(state: AgentState) -> str:
    """Conditional-edge router keyed on ``state["decision"]``."""
    return state.get("decision") or "escalate"


def build_graph(
    llm: LLMPort,
    memory: MemoryPort,
    runbook: RunbookPort,
    observability: ObservabilityPort | None = None,
    top_k: int = 3,
):
    """Compile the incident-analysis graph with the given port adapters.

    ``observability`` is optional; when omitted the ``observe`` node stores
    empty evidence and the graph behaves as before.
    """
    graph = StateGraph(AgentState)
    graph.add_node("memory_check", create_memory_check_node(memory, top_k=top_k))
    graph.add_node("observe", create_observe_node(observability))
    graph.add_node("analyze", create_analyze_node(llm))
    graph.add_node("decide", create_decide_node(runbook))
    graph.add_node("run_runbook", create_run_runbook_node(runbook))
    graph.add_node("escalate", create_escalate_node(llm))
    graph.add_node("persist_memory", create_persist_memory_node(memory))

    graph.add_edge(START, "memory_check")
    graph.add_edge("memory_check", "observe")
    graph.add_edge("observe", "analyze")
    graph.add_edge("analyze", "decide")
    graph.add_conditional_edges(
        "decide",
        _route_after_decide,
        {"run_runbook": "run_runbook", "escalate": "escalate"},
    )
    graph.add_edge("run_runbook", "persist_memory")
    graph.add_edge("escalate", "persist_memory")
    graph.add_edge("persist_memory", END)
    return graph.compile()
