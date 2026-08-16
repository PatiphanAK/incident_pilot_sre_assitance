"""memory_check node — recall similar past incidents before reasoning."""

from __future__ import annotations

from agent.state import AgentState
from domain.ports.memory_port import MemoryPort


def create_memory_check_node(memory: MemoryPort, top_k: int = 3):
    """Return a node that stores the ``top_k`` most similar past
    incidents in ``state["past_incidents"]``.
    """

    def memory_check(state: AgentState) -> dict:
        past = memory.find_similar(state["incident"], top_k=top_k)
        return {"past_incidents": past}

    return memory_check
