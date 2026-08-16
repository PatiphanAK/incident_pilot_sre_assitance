"""Incident agent graph — wires the nodes together.

    START → memory_check → analyze → decide ─┬→ run_runbook → persist_memory → END
                                             └→ escalate ───→ persist_memory → END

``decide`` routes on confidence x blast radius: a known incident (matching
runbook + prior memory) with low/medium blast radius runs the runbook,
anything else escalates to a human.

Ports are injected at build time (dependency injection), so the same
graph runs against the real CockroachDB adapters or the in-memory fakes.
"""

from __future__ import annotations

from langgraph.graph import END, START, StateGraph

from agent.nodes.analyze_node import create_analyze_node
from agent.nodes.decide_node import create_decide_node
from agent.nodes.escalate_node import create_escalate_node
from agent.nodes.memory_check_node import create_memory_check_node
from agent.nodes.persist_memory_node import create_persist_memory_node
from agent.nodes.run_runbook_node import create_run_runbook_node
from agent.state import AgentState
from domain.ports.llm_port import LLMPort
from domain.ports.memory_port import MemoryPort
from domain.ports.runbook_port import RunbookPort


def _route_after_decide(state: AgentState) -> str:
    """Conditional-edge router keyed on ``state["decision"]``."""
    return state.get("decision") or "escalate"


def build_graph(llm: LLMPort, memory: MemoryPort, runbook: RunbookPort, top_k: int = 3):
    """Compile the incident-analysis graph with the given port adapters."""
    graph = StateGraph(AgentState)
    graph.add_node("memory_check", create_memory_check_node(memory, top_k=top_k))
    graph.add_node("analyze", create_analyze_node(llm))
    graph.add_node("decide", create_decide_node(runbook))
    graph.add_node("run_runbook", create_run_runbook_node(runbook))
    graph.add_node("escalate", create_escalate_node(llm))
    graph.add_node("persist_memory", create_persist_memory_node(memory))

    graph.add_edge(START, "memory_check")
    graph.add_edge("memory_check", "analyze")
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
