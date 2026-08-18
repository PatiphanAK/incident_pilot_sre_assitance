"""Mark every test under ``tests/unit`` as a unit test.

The whole point of this directory is "fakes only — no network, no DB",
so ``pytest -m unit`` selects it without each module repeating
``pytestmark``. Note: conftest hooks are session-global, so the marker
is applied only to items whose file actually lives under this directory.
"""

from __future__ import annotations

from pathlib import Path

import pytest

_UNIT_DIR = Path(__file__).resolve().parent


def pytest_collection_modifyitems(items: list[pytest.Item]) -> None:
    for item in items:
        if _UNIT_DIR in Path(item.fspath).resolve().parents:
            item.add_marker(pytest.mark.unit)
