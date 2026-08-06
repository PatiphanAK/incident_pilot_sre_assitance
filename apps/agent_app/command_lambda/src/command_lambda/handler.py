"""Lambda handler — the API Gateway adapter wiring the whole flow.

``parse -> validate -> submit -> 202 / 400 / 500``. This is the *only* place
that knows about the Lambda runtime envelope and boto3; everything upstream is
pure stdlib. SQS failures fail closed (``500``, never a lying ``202``).

The publisher is built lazily and cached on the module so a warm container
reuses the boto3 client across invocations (faster, fewer connections).
"""

from __future__ import annotations

import logging
from typing import Any, Mapping

from . import response
from .apigw import parse_event
from .run_id import resolve_run_id_factory
from .submit import submit_command
from .validate import validate_command

__all__ = ["lambda_handler"]

log = logging.getLogger()
log.setLevel(logging.INFO)

# Cache the boto3-backed publisher on the module so a warm Lambda reuses the
# SQS client. Built lazily (and only when a real send is needed) so unit tests
# and `sam local invoke` without env can still import this module.
_publisher: Any = None


def _get_publisher() -> Any:
    global _publisher
    if _publisher is None:
        from .sqs_publisher import build_publisher

        _publisher = build_publisher()
    return _publisher


def lambda_handler(
    event: Mapping[str, Any], context: Any = None, *, publisher: Any = None
) -> dict[str, Any]:
    """Handle an API Gateway proxy event for ``POST /runs``.

    The optional ``publisher`` keyword is a seam for tests to inject a fake
    without touching the cached boto3 client. Production invocations leave it
    ``None`` and the real :class:`SqsPublisher` is used.
    """
    parsed_event = parse_event(event)
    cors_origin = "*"

    # 1) Validate. Body may be None (missing/empty/unparseable) -> 400.
    outcome = validate_command(parsed_event.body)
    if hasattr(outcome, "fields"):  # ValidationError
        return response.invalid(outcome, cors_origin)

    parsed_command = outcome  # ParsedCommand

    # 2) Resolve the run-id factory from the optional idempotency header.
    idempotency_key = parsed_event.headers.get("x-idempotency-key")
    run_id_factory = resolve_run_id_factory(idempotency_key)

    # 3) Submit. Fail closed: a publish error becomes 500, never a 202.
    pub = publisher if publisher is not None else _get_publisher()
    try:
        result = submit_command(parsed_command, pub, run_id_factory)
    except Exception as exc:  # noqa: BLE001  (broad: any publisher failure -> 500)
        log.exception("submission failed")
        return response.failed("submission failed", cors_origin)

    # 4) 202 Accepted.
    return response.accepted(result, cors_origin)
