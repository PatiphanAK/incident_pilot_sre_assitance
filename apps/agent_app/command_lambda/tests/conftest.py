"""Shared test fixtures.

The :class:`FakePublisher` is the test double for :class:`CommandPublisher`.
It records every :class:`EnqueuedMessage` it would have sent and can be
configured to raise, so ``submit_command`` and ``lambda_handler`` can be
exercised without boto3, network, or an AWS account.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Sequence

import pytest

from command_lambda.domain import CommandPublisher, EnqueuedMessage


@dataclass
class FakePublisher:
    """Captures published messages instead of sending them to SQS."""

    published: list[EnqueuedMessage] = field(default_factory=list)
    fail_with: BaseException | None = None

    def publish(self, message: EnqueuedMessage) -> None:
        if self.fail_with is not None:
            raise self.fail_with
        self.published.append(message)

    @property
    def last(self) -> EnqueuedMessage:
        return self.published[-1]

    def bodies(self) -> list[dict]:
        return [m.to_dict() for m in self.published]


@pytest.fixture
def fake_publisher() -> FakePublisher:
    return FakePublisher()


@pytest.fixture
def fixed_clock() -> callable:
    """A UtcNow that always returns the same timestamp for determinism."""
    def _now() -> str:
        return "2026-08-06T14:00:00Z"
    return _now


@pytest.fixture
def fixed_run_id() -> callable:
    """A RunIdFactory that always returns the same id for determinism."""
    def _factory() -> str:
        return "run-1234567890abcdef"
    return _factory


def _gw_event(body: object | None, headers: dict | None = None) -> dict:
    """Build a minimal API Gateway REST/HTTP API proxy event."""
    import json
    return {
        "resource": "/runs",
        "path": "/runs",
        "httpMethod": "POST",
        "headers": headers or {},
        "body": None if body is None else json.dumps(body),
        "isBase64Encoded": False,
    }


@pytest.fixture
def gw_event():
    return _gw_event
