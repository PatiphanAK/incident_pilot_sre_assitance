"""HTTP response envelopes for the command Lambda.

Returns API Gateway proxy response dicts (``statusCode``/``headers``/``body``).
Bodies are JSON strings. No internal details leak in error envelopes — the
``500`` carries only a stable code and a generic message; structured validation
failures carry precise field paths (safe to disclose — they describe the input).
"""

from __future__ import annotations

import json
from typing import Any, Mapping, Sequence

from .domain import SubmissionResult, ValidationError

__all__ = ["accepted", "invalid", "failed", "_DEFAULT_HEADERS"]

_HEADERS: Mapping[str, str] = {
    "Content-Type": "application/json",
}


def accepted(result: SubmissionResult, cors_origin: str = "*") -> dict[str, Any]:
    """202 Accepted — the command validated and was enqueued to SQS."""
    return _envelope(202, result.to_dict(), cors_origin)


def invalid(
    error: ValidationError, cors_origin: str = "*"
) -> dict[str, Any]:
    """400 Bad Request — structured validation failure with per-field detail."""
    fields = [
        {"path": fe.path, "message": fe.message} for fe in error.fields
    ]
    return _envelope(
        400,
        {"error": {"code": error.code, "fields": fields}},
        cors_origin,
    )


def failed(
    message: str = "submission failed",
    cors_origin: str = "*",
) -> dict[str, Any]:
    """500 Internal Server Error — SQS send failed; fail closed (no 202)."""
    return _envelope(
        500,
        {"error": {"code": "SUBMISSION_FAILED", "message": message}},
        cors_origin,
    )


def _envelope(
    status: int, body: Mapping[str, Any], cors_origin: str
) -> dict[str, Any]:
    headers = dict(_HEADERS)
    if cors_origin:
        headers["Access-Control-Allow-Origin"] = cors_origin
    return {
        "statusCode": status,
        "headers": headers,
        "body": json.dumps(body),
    }
