"""Parse an API Gateway proxy (REST/HTTP API) event into a body dict + headers.

Isolates the API Gateway envelope shape from the handler logic. Handles the
common encodings (base64 vs plain JSON) and lower-cases header keys so the
handler can read ``X-Idempotency-Key`` without case sensitivity.

Never raises on a malformed event — it returns ``None`` for body so the handler
can map an unparseable/empty body to a structured ``400`` rather than a ``500``.
"""

from __future__ import annotations

import base64
import json
from typing import Any, Mapping

__all__ = ["parse_event", "ParsedEvent"]


class ParsedEvent:
    """Minimal view over an API Gateway event: the parsed body + headers."""

    __slots__ = ("body", "headers")

    def __init__(self, body: object | None, headers: Mapping[str, str]):
        self.body = body
        self.headers = headers


def _decode_body(raw_body: Any, is_base64: bool) -> str:
    """Return the body as a string, decoding base64 when API GW flagged it."""
    if raw_body is None:
        return ""
    if is_base64 and isinstance(raw_body, (bytes, str)):
        data = raw_body if isinstance(raw_body, bytes) else raw_body.encode("utf-8")
        return base64.b64decode(data).decode("utf-8", errors="replace")
    if isinstance(raw_body, bytes):
        return raw_body.decode("utf-8", errors="replace")
    return str(raw_body)


def parse_event(event: Mapping[str, Any]) -> ParsedEvent:
    """Parse an API Gateway proxy event.

    Tolerant: an empty or non-object body yields ``body=None`` (the validator
    turns that into a precise ``400``). An unparseable JSON body also yields
    ``body=None`` with no exception escaping.
    """
    raw_body = event.get("body")
    is_base64 = bool(event.get("isBase64Encoded", False))
    body_str = _decode_body(raw_body, is_base64).strip()

    parsed: object | None = None
    if body_str:
        try:
            decoded = json.loads(body_str)
        except (ValueError, TypeError):
            parsed = None
        else:
            parsed = decoded

    # Lowercase the header map once. API GW may provide either a dict or None.
    raw_headers = event.get("headers") or {}
    headers = {
        str(k).lower(): ("" if v is None else str(v))
        for k, v in raw_headers.items()
    }
    return ParsedEvent(body=parsed, headers=headers)
