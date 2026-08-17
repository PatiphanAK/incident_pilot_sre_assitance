"""Architectural boundary tests — enforce the hexagonal constraint.

DoD: no adapter or driver imports appear anywhere under ``agent/`` or
``domain/``. These tests fail if that invariant ever breaks.
"""

from __future__ import annotations

import re
from pathlib import Path

import pytest

_SRC = Path(__file__).resolve().parents[2] / "src"

# Module prefixes that must never be imported inside agent/ or domain/.
_FORBIDDEN = (
    "psycopg",  # DB driver
    "openai",  # LLM SDK
    "boto3",  # AWS SDK (CloudWatch)
    "botocore",  # AWS SDK core
    "adapters",  # concrete implementations
    "langchain",  # heavy SDKs; the agent uses langgraph only
)


def _py_files(*dirs: str) -> list[Path]:
    return sorted(p for d in dirs for p in (_SRC / d).rglob("*.py"))


@pytest.mark.parametrize("path", _py_files("agent", "domain"), ids=lambda p: str(p))
def test_agent_and_domain_import_no_drivers_or_adapters(path: Path) -> None:
    source = path.read_text()
    offenders = [
        line.strip()
        for line in source.splitlines()
        if re.match(r"^\s*(?:import|from)\s+", line)
        and any(m in line for m in _FORBIDDEN)
    ]
    assert not offenders, f"{path} imports forbidden modules: {offenders}"


def test_domain_does_not_import_agent() -> None:
    for path in _py_files("domain"):
        source = path.read_text()
        assert "import agent" not in source, f"{path} imports the agent layer"
