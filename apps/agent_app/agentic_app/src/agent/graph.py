"""Incident agent graph — wires the nodes together.

    START → memory_check → analyze → persist_memory → END

Ports are injected at build time (dependency injection), so the same
graph runs against the real CockroachDB adapter or the in-memory fake.
"""

from __future__ import annotations

from langgraph.graph import END, START, StateGraph

from agent.nodes.analyze_node import create_analyze_node
from agent.nodes.memory_check_node import create_memory_check_node
from agent.nodes.persist_memory_node import create_persist_memory_node
from agent.state import AgentState
from domain.ports.llm_port import LLMPort
from domain.ports.memory_port import MemoryPort


def build_graph(llm: LLMPort, memory: MemoryPort, top_k: int = 3):
    """Compile the incident-analysis graph with the given port adapters."""
    graph = StateGraph(AgentState)
    graph.add_node("memory_check", create_memory_check_node(memory, top_k=top_k))
    graph.add_node("analyze", create_analyze_node(llm))
    graph.add_node("persist_memory", create_persist_memory_node(memory))

    graph.add_edge(START, "memory_check")
    graph.add_edge("memory_check", "analyze")
    graph.add_edge("analyze", "persist_memory")
    graph.add_edge("persist_memory", END)
    return graph.compile()
