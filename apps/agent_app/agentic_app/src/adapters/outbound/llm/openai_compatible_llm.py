"""Foundation-model adapter — OpenAI-compatible chat endpoint.

Defaults to the provider already configured as the primary LLM
(DashScope/Qwen via ``QWEN_BASE_URL`` in ``.env``); any OpenAI-compatible
endpoint works by setting ``OPENAI_*``/``QWEN_*`` vars instead.
"""

from __future__ import annotations

import os
import threading

from openai import OpenAI

from domain.ports.llm_port import LLMPort

_CLIENT: OpenAI | None = None
_lock = threading.Lock()


def _get_client() -> OpenAI:
    global _CLIENT
    if _CLIENT is None:
        with _lock:
            if _CLIENT is None:
                api_key = os.environ.get("QWEN_API_KEY") or os.environ.get("OPENAI_API_KEY")
                if not api_key:
                    raise RuntimeError(
                        "No API key configured for the LLM provider; "
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
    return os.environ.get("QWEN_MODEL", "qwen-plus")


class OpenAICompatibleLLM(LLMPort):
    """Single-shot chat completion over an OpenAI-compatible API."""

    def generate(self, prompt: str) -> str:
        response = _get_client().chat.completions.create(
            model=_model(),
            messages=[{"role": "user", "content": prompt}],
        )
        return response.choices[0].message.content or ""
