"""Unit tests for :func:`command_lambda.submit.submit_command`.

Verifies the exact SQS message body shape (the contract with the future ECS
worker), idempotency-key override of the run-id factory, and fail-closed
behavior on publisher error.
"""

from __future__ import annotations

import pytest

from command_lambda.domain import EnqueuedMessage, ParsedCommand, Incident
from command_lambda.submit import submit_command

from conftest import FakePublisher


def _parsed(**incident_overrides) -> ParsedCommand:
    incident = {
        "description": "high 5xx on checkout",
        "severity": "high",
        "service": "checkout-api",
        "region": "us-east-1",
        "signals": ("5xx-spike", "latency-p99"),
    }
    incident.update(incident_overrides)
    return ParsedCommand(
        incident=Incident(**incident),
        session_id=None,
    )


def test_submit_publishes_exact_message_body(fake_publisher, fixed_run_id, fixed_clock):
    result = submit_command(
        _parsed(),
        fake_publisher,
        fixed_run_id,
        now=fixed_clock,
    )
    assert result.run_id == "run-1234567890abcdef"
    assert result.status == "queued"
    assert result.submitted_at == "2026-08-06T14:00:00Z"

    assert len(fake_publisher.published) == 1
    body = fake_publisher.bodies()[0]
    assert body == {
        "run_id": "run-1234567890abcdef",
        "session_id": None,
        "incident": {
            "description": "high 5xx on checkout",
            "severity": "high",
            "service": "checkout-api",
            "region": "us-east-1",
            "signals": ["5xx-spike", "latency-p99"],
        },
        "submitted_at": "2026-08-06T14:00:00Z",
        "source": "command_lambda",
    }


def test_submit_carries_session_id_through(fake_publisher, fixed_run_id, fixed_clock):
    parsed = ParsedCommand(
        incident=Incident(
            description="d",
            severity="low",
            service=None,
            region=None,
            signals=(),
        ),
        session_id="sess-abc",
    )
    submit_command(parsed, fake_publisher, fixed_run_id, now=fixed_clock)
    assert fake_publisher.last.session_id == "sess-abc"
    assert fake_publisher.bodies()[0]["session_id"] == "sess-abc"


def test_submit_propagates_publisher_error(fake_publisher, fixed_run_id, fixed_clock):
    fake_publisher.fail_with = RuntimeError("sqs down")
    with pytest.raises(RuntimeError, match="sqs down"):
        submit_command(_parsed(), fake_publisher, fixed_run_id, now=fixed_clock)
    # fail closed: nothing recorded as enqueued
    assert fake_publisher.published == []


def test_run_id_factory_is_called_once_per_submission(fake_publisher, fixed_clock):
    calls = {"n": 0}

    def factory() -> str:
        calls["n"] += 1
        return "run-xyz"

    submit_command(_parsed(), fake_publisher, factory, now=fixed_clock)
    assert calls["n"] == 1
    assert fake_publisher.last.run_id == "run-xyz"


def test_published_message_is_enqueued_message_instance(fake_publisher, fixed_run_id, fixed_clock):
    submit_command(_parsed(), fake_publisher, fixed_run_id, now=fixed_clock)
    assert isinstance(fake_publisher.last, EnqueuedMessage)
    assert fake_publisher.last.source == "command_lambda"
