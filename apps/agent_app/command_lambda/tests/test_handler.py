"""Unit tests for :func:`command_lambda.handler.lambda_handler`.

Drives the full API Gateway -> Lambda path end-to-end with a fake publisher
(no boto3, no Docker, no AWS): valid -> 202 + run_id; validation failures ->
400; SQS failure -> 500; ``X-Idempotency-Key`` honored.
"""

from __future__ import annotations

import json

import pytest

from command_lambda.handler import lambda_handler
from command_lambda.run_id import new_run_id, resolve_run_id_factory

from conftest import FakePublisher


def _body(resp: dict) -> dict:
    return json.loads(resp["body"])


def test_valid_command_returns_202_with_run_id(
    fake_publisher: FakePublisher, fixed_run_id, fixed_clock, gw_event
):
    # Patch the run-id factory used by the handler by injecting one via the
    # idempotency-key path (valid key -> client-controlled id).
    resp = lambda_handler(
        gw_event(
            {"incident": {"description": "high 5xx on checkout"}},
            headers={"X-Idempotency-Key": "client-key-1"},
        ),
        publisher=fake_publisher,
    )
    assert resp["statusCode"] == 202
    body = _body(resp)
    assert body["status"] == "queued"
    # client-controlled id honored
    assert body["run_id"] == "client-key-1"
    assert "submitted_at" in body

    # exactly one message enqueued, with the exact contract body
    assert len(fake_publisher.published) == 1
    assert fake_publisher.bodies()[0]["run_id"] == "client-key-1"
    assert fake_publisher.bodies()[0]["source"] == "command_lambda"


def test_default_run_id_is_uuid_when_no_idempotency_key(
    fake_publisher: FakePublisher, gw_event
):
    resp = lambda_handler(
        gw_event({"incident": {"description": "db cpu saturated"}}),
        publisher=fake_publisher,
    )
    assert resp["statusCode"] == 202
    rid = _body(resp)["run_id"]
    # uuid4 string, 36 chars incl. hyphens (e.g. 8-4-4-4-12)
    assert len(rid) == 36 and rid.count("-") == 4
    # parseable as a uuid
    import uuid as _uuid
    _uuid.UUID(rid)
    assert fake_publisher.bodies()[0]["run_id"] == rid


def test_missing_description_returns_400(fake_publisher: FakePublisher, gw_event):
    resp = lambda_handler(
        gw_event({"incident": {"severity": "low"}}),
        publisher=fake_publisher,
    )
    assert resp["statusCode"] == 400
    body = _body(resp)
    assert body["error"]["code"] == "INVALID_COMMAND"
    paths = [f["path"] for f in body["error"]["fields"]]
    assert "incident.description" in paths
    # nothing enqueued
    assert fake_publisher.published == []


def test_empty_body_returns_400(fake_publisher: FakePublisher, gw_event):
    resp = lambda_handler(gw_event(None), publisher=fake_publisher)
    assert resp["statusCode"] == 400
    assert _body(resp)["error"]["code"] == "INVALID_COMMAND"


def test_unparseable_body_returns_400(fake_publisher: FakePublisher):
    resp = lambda_handler(
        {"body": "{not json", "isBase64Encoded": False, "headers": {}},
        publisher=fake_publisher,
    )
    assert resp["statusCode"] == 400


def test_unknown_field_returns_400(fake_publisher: FakePublisher, gw_event):
    resp = lambda_handler(
        gw_event({"incident": {"description": "x"}, "runbook": "scale-up"}),
        publisher=fake_publisher,
    )
    assert resp["statusCode"] == 400
    paths = [f["path"] for f in _body(resp)["error"]["fields"]]
    assert "runbook" in paths


def test_bad_severity_returns_400(fake_publisher: FakePublisher, gw_event):
    resp = lambda_handler(
        gw_event({"incident": {"description": "x", "severity": "lava"}}),
        publisher=fake_publisher,
    )
    assert resp["statusCode"] == 400
    paths = [f["path"] for f in _body(resp)["error"]["fields"]]
    assert "incident.severity" in paths


def test_oversized_signals_returns_400(fake_publisher: FakePublisher, gw_event):
    from command_lambda.domain import MAX_SIGNALS
    resp = lambda_handler(
        gw_event(
            {
                "incident": {
                    "description": "x",
                    "signals": [f"s{i}" for i in range(MAX_SIGNALS + 1)],
                }
            }
        ),
        publisher=fake_publisher,
    )
    assert resp["statusCode"] == 400
    paths = [f["path"] for f in _body(resp)["error"]["fields"]]
    assert "incident.signals" in paths


def test_sqs_failure_returns_500(fake_publisher: FakePublisher, gw_event):
    fake_publisher.fail_with = RuntimeError("sqs throttled")
    resp = lambda_handler(
        gw_event({"incident": {"description": "x"}}),
        publisher=fake_publisher,
    )
    assert resp["statusCode"] == 500
    body = _body(resp)
    assert body["error"]["code"] == "SUBMISSION_FAILED"
    # no internals leaked
    assert "throttled" not in body["error"]["message"]
    # nothing recorded as enqueued
    assert fake_publisher.published == []


def test_idempotency_key_honored(fake_publisher: FakePublisher, gw_event):
    resp = lambda_handler(
        gw_event(
            {"incident": {"description": "x"}},
            headers={"X-Idempotency-Key": "client-supplied-id"},
        ),
        publisher=fake_publisher,
    )
    assert resp["statusCode"] == 202
    assert _body(resp)["run_id"] == "client-supplied-id"
    assert fake_publisher.bodies()[0]["run_id"] == "client-supplied-id"


def test_invalid_idempotency_key_falls_back_to_uuid(
    fake_publisher: FakePublisher, gw_event
):
    resp = lambda_handler(
        gw_event(
            {"incident": {"description": "x"}},
            # spaces + overlong chars -> invalid -> ignored, server id used
            headers={"X-Idempotency-Key": "bad key with spaces!"},
        ),
        publisher=fake_publisher,
    )
    assert resp["statusCode"] == 202
    rid = _body(resp)["run_id"]
    assert len(rid) == 36 and rid.count("-") == 4  # uuid fallback


def test_response_has_cors_header(fake_publisher: FakePublisher, gw_event):
    resp = lambda_handler(
        gw_event({"incident": {"description": "x"}}), publisher=fake_publisher
    )
    assert resp["headers"]["Access-Control-Allow-Origin"] == "*"
    assert resp["headers"]["Content-Type"] == "application/json"


def test_base64_body_is_decoded(fake_publisher: FakePublisher):
    import base64
    payload = json.dumps({"incident": {"description": "decoded ok"}})
    encoded = base64.b64encode(payload.encode()).decode()
    resp = lambda_handler(
        {"body": encoded, "isBase64Encoded": True, "headers": {}},
        publisher=fake_publisher,
    )
    assert resp["statusCode"] == 202


def test_resolve_run_id_factory_unit():
    # valid key -> factory returns that key always
    factory = resolve_run_id_factory("abc-123_def.0")
    assert factory() == "abc-123_def.0"
    assert factory() == "abc-123_def.0"  # stable
    # invalid / empty -> uuid factory
    assert resolve_run_id_factory(None).__name__  # callable
    f = resolve_run_id_factory("has spaces")
    assert f() != "has spaces"
    # uuid factory produces distinct ids
    a, b = new_run_id(), new_run_id()
    assert a != b and len(a) == 36 and len(b) == 36
