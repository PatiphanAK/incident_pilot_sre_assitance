"""Ports — the contracts the agent depends on.

Adapters implement these; the agent (and anything in ``domain/``) only
ever sees the port, never a concrete driver or SDK.
"""

from domain.ports.llm_port import LLMPort
from domain.ports.memory_port import MemoryPort

__all__ = ["LLMPort", "MemoryPort"]
