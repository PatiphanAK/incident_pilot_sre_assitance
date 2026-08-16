"""In-memory FakeMemoryPort for unit testing agent nodes.

Implements :class:`domain.ports.memory_port.MemoryPort` with an
in-memory list and cosine similarity in pure Python — zero network
calls, zero DB connections, no dependency on the embedding provider.

The built-in "embedding" is a deterministic hash-bag of character
trigrams: not semantically smart, but deterministic and monotonic with
lexical overlap, which is all the unit tests need to verify retrieval
ordering, recall, and persistence behavior.
"""

from __future__ import annotations

import hashlib
import math
import re

from domain.models import IncidentMemory
from domain.ports.memory_port import MemoryPort

_DIM = 256


def _fake_embed(text: str) -> list[float]:
    """Deterministic hash-trigram embedding, L2-normalized."""
    vec = [0.0] * _DIM
    for token in re.findall(r"[a-z0-9]+", text.lower()):
        padded = f" {token} "
        for i in range(len(padded) - 2):
            gram = padded[i : i + 3]
            idx = int(hashlib.sha256(gram.encode()).hexdigest(), 16) % _DIM
            vec[idx] += 1.0
    norm = math.sqrt(sum(x * x for x in vec))
    return [x / norm for x in vec] if norm else vec


def _cosine(a: list[float], b: list[float]) -> float:
    dot = sum(x * y for x, y in zip(a, b))
    na = math.sqrt(sum(x * x for x in a))
    nb = math.sqrt(sum(y * y for y in b))
    if na == 0.0 or nb == 0.0:
        return 0.0
    return dot / (na * nb)


class FakeMemoryPort(MemoryPort):
    """In-memory incident memory (cosine similarity, pure Python)."""

    def __init__(self) -> None:
        self._items: list[tuple[str, str | None, list[float]]] = []

    def save_incident(self, summary: str, resolution: str | None = None) -> None:
        self._items.append((summary, resolution, _fake_embed(summary)))

    def find_similar(self, text: str, top_k: int = 3) -> list[IncidentMemory]:
        if not self._items:
            return []
        query = _fake_embed(text)
        ranked = sorted(
            range(len(self._items)),
            key=lambda i: _cosine(query, self._items[i][2]),
            reverse=True,
        )
        return [
            IncidentMemory(
                summary=self._items[i][0],
                resolution=self._items[i][1],
                distance=1.0 - _cosine(query, self._items[i][2]),
            )
            for i in ranked[:top_k]
        ]

    @property
    def summaries(self) -> list[str]:
        """Stored summaries, in insertion order (test introspection)."""
        return [summary for summary, _, _ in self._items]
