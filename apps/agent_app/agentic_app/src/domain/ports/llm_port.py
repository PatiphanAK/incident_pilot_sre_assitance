"""LLMPort — the foundation-model boundary.

Nodes call this port; the concrete provider (Qwen/DashScope today,
Bedrock/Claude later) is an adapter and stays swappable.
"""

from __future__ import annotations

from abc import ABC, abstractmethod


class LLMPort(ABC):
    """Minimal text-in / text-out LLM contract the agent needs."""

    @abstractmethod
    def generate(self, prompt: str) -> str:
        """Run a single completion and return the model's text response."""
