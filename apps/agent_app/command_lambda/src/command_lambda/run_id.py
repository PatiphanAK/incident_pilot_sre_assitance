"""run_id generation — the dedup key for the future ECS worker.

Defaults to a server-generated uuid4 (random UUID — CockroachDB-friendly,
avoids hot spots on sequential keys). An optional client-supplied
``X-Idempotency-Key`` header overrides it, enabling client-controlled dedup
without the Lambda touching a database.
"""

from __future__ import annotations

import re
from uuid import uuid4

from .domain import RunIdFactory

__all__ = ["new_run_id", "resolve_run_id_factory", "IDEMPOTENCY_KEY_RE"]

#: A client idempotency key must be a non-empty token of these chars and length.
#: We constrain it so a hostile or fat-fingered header can't become a weird
#: run_id / SQS message attribute. uuid-like, slug-like, and short hashes pass.
IDEMPOTENCY_KEY_RE = re.compile(r"^[A-Za-z0-9._\-:]{1,128}$")


def new_run_id() -> str:
    """Default factory: a fresh uuid4 as a canonical hex string."""
    return str(uuid4())


def resolve_run_id_factory(idempotency_key: str | None) -> RunIdFactory:
    """Return a run-id factory honoring an optional ``X-Idempotency-Key``.

    - No key (or empty/invalid) -> the uuid4 factory (server-generated).
    - Valid key -> a factory that always returns that key (client-controlled dedup).

    Invalid keys are ignored (treated as absent) rather than failing the request,
    because idempotency is a *client convenience*, not a correctness requirement
    on the server side — and a 400 on a bad optional header would surprise clients.
    """
    if idempotency_key and IDEMPOTENCY_KEY_RE.match(idempotency_key):
        key = idempotency_key
        return lambda: key
    return new_run_id
