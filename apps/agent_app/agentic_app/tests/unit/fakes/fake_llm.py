"""Canned-response FakeLLM for unit testing the analyze node and graph.

Records every prompt it receives so tests can assert what context the
nodes actually sent to the model.
"""

from __future__ import annotations

from domain.ports.llm_port import LLMPort


class FakeLLM(LLMPort):
    def __init__(self, response: str = "Synthetic analysis: restart the service.") -> None:
        self.response = response
        self.prompts: list[str] = []

    def generate(self, prompt: str) -> str:
        self.prompts.append(prompt)
        return self.response
