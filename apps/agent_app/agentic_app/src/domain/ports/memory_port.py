"""MemoryPort — long-term incident memory (RAG over past incidents).

The agent layer (``agent/nodes/*``) may only call this port. Concrete
backends (CockroachDB, an in-memory fake, ...) live in
``adapters/`` or ``tests/`` and are injected at graph build time.
"""

from __future__ import annotations

from abc import ABC, abstractmethod

from domain.models import IncidentMemory


class MemoryPort(ABC):
    """Semantic long-term memory for observed incidents."""

    @abstractmethod
    def find_similar(self, text: str, top_k: int = 3) -> list[IncidentMemory]:
        """Return the ``top_k`` stored incidents most similar to ``text``.

        Similarity is semantic (embedding distance), not keyword based.
        Results are ordered most-similar first. An empty list means
        nothing relevant (or nothing stored yet).
        """

    @abstractmethod
    def save_incident(self, summary: str, resolution: str | None = None) -> None:
        """Persist a newly observed incident for future recall."""
