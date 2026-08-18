"""Embedding adapter — OpenAI-compatible ``/embeddings`` endpoint.

The provider is selected by environment, defaulting to the same
provider that powers the primary LLM (DashScope/Qwen today):

- ``QWEN_API_KEY`` / ``QWEN_BASE_URL``  (primary, matches ``.env``)
- ``OPENAI_API_KEY`` / ``OPENAI_BASE_URL`` (generic fallback)
- ``EMBEDDING_MODEL``  (default: ``text-embedding-v4``)
- ``EMBEDDING_DIMENSIONS`` (default: ``1536``, matching ``VECTOR(1536)``)

Nothing here is hardcoded into the memory adapter: it only ever calls
:func:`embed`, so swapping the provider is a config change, not a code
change.
"""

from __future__ import annotations

import os
import threading

from openai import OpenAI

_CLIENT: OpenAI | None = None
_lock = threading.Lock()

_DEFAULT_MODEL = "text-embedding-v4"
_DEFAULT_DIMENSIONS = 1536


def _get_client() -> OpenAI:
    """Lazily build the OpenAI-compatible client from the environment."""
    global _CLIENT
    if _CLIENT is None:
        with _lock:
            if _CLIENT is None:
                api_key = os.environ.get("QWEN_API_KEY") or os.environ.get("OPENAI_API_KEY")
                if not api_key:
                    raise RuntimeError(
                        "No API key configured for the embedding provider; "
                        "set QWEN_API_KEY (or OPENAI_API_KEY)."
                    )
                base_url = (
                    os.environ.get("QWEN_BASE_URL")
                    or os.environ.get("OPENAI_BASE_URL")
                    or "https://api.openai.com/v1"
                )
                _CLIENT = OpenAI(api_key=api_key, base_url=base_url)
    return _CLIENT


def _model() -> str:
    return os.environ.get("EMBEDDING_MODEL", _DEFAULT_MODEL)


def _dimensions() -> int:
    return int(os.environ.get("EMBEDDING_DIMENSIONS", _DEFAULT_DIMENSIONS))


def embed(text: str) -> list[float]:
    """Embed ``text`` into a vector of floats.

    The vector length is determined by the configured model/dimensions
    and must match the ``VECTOR(n)`` column of ``observed_incidents``.
    """
    response = _get_client().embeddings.create(
        model=_model(), input=text, dimensions=_dimensions()
    )
    return [float(x) for x in response.data[0].embedding]
