"""Core domain models for incident memory.

These are plain dataclasses — no infrastructure types, so the domain
stays importable from anywhere (nodes, tests, adapters) without
dragging in drivers or SDKs.
"""

from __future__ import annotations

from dataclasses import dataclass


@dataclass(frozen=True)
class IncidentMemory:
    """One recalled piece of long-term operational memory.

    ``distance`` is the L2 (euclidean) distance from the query embedding,
    as returned by the vector store. Lower is more similar; it is only
    meaningful relative to other results from the same search.
    """

    summary: str
    resolution: str | None
    distance: float
