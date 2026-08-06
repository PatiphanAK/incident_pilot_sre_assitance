"""Submit a validated command — the application use case that enqueues a job.

Pure stdlib + the :class:`CommandPublisher` port. It does not touch AWS, the
filesystem, or wall-clock directly — those are injected (publisher, run-id
factory, utc-now clock) so the use case is fully deterministic under test and
the same code path runs behind both the Lambda handler and the FastAPI shim.

Contract with the future ECS worker: the :class:`EnqueuedMessage` published
here is the SQS message body the worker will consume. The Lambda is DB-free; the
worker creates the ``agent_runs`` row on first pickup (idempotent on ``run_id``).
"""

from __future__ import annotations

from datetime import datetime, timezone

from .domain import (
    CommandPublisher,
    EnqueuedMessage,
    ParsedCommand,
    RunIdFactory,
    SubmissionResult,
    UtcNow,
)

__all__ = ["submit_command", "utc_now"]


def utc_now() -> str:
    """Default UTC clock: ISO-8601 with ``Z`` suffix (e.g. ``2026-08-06T14:00:00Z``).

    Kept here (not in ``run_id.py``) because the timestamp is part of the
    submission contract, not the id. Injected into :func:`submit_command` so
    tests can freeze time.
    """
    return datetime.now(timezone.utc).replace(microsecond=0).isoformat().replace(
        "+00:00", "Z"
    )


def submit_command(
    parsed: ParsedCommand,
    publisher: CommandPublisher,
    run_id_factory: RunIdFactory,
    now: UtcNow = utc_now,
) -> SubmissionResult:
    """Build the :class:`EnqueuedMessage`, publish it, and return the 202 data.

    Raises propagate from ``publisher.publish`` (e.g. SQS ``ClientError``) — the
    handler catches them and maps to ``500``. We **fail closed**: a successful
    return guarantees the message was accepted by SQS; a failed publish never
    returns ``202``.
    """
    run_id = run_id_factory()
    submitted_at = now()

    message = EnqueuedMessage(
        run_id=run_id,
        session_id=parsed.session_id,
        incident=parsed.incident,
        submitted_at=submitted_at,
        source="command_lambda",
    )
    publisher.publish(message)

    return SubmissionResult(
        run_id=run_id,
        status="queued",
        submitted_at=submitted_at,
    )
