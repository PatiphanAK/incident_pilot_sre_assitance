"""Local FastAPI shim — exercises the *same* validate+submit path as the Lambda.

No Docker, no AWS account needed. Reuses :func:`validate_command` and
:func:`submit_command` so there is **one validator, two HTTP adapters** (this
and the Lambda handler). By default it uses an in-memory :class:`FakePublisher`
that logs the enqueued message; set ``SQS_QUEUE_URL`` to point at a real or
LocalStack queue to verify a live send.

Run::

    uv run fastapi dev local_dev.py
    curl -X POST localhost:8000/runs -H 'Content-Type: application/json' \
        -d '{"incident":{"description":"high 5xx on checkout"}}'
"""

from __future__ import annotations

import json
import logging
import os

from fastapi import FastAPI, Request
from fastapi.responses import JSONResponse

from command_lambda.domain import CommandPublisher, EnqueuedMessage
from command_lambda.run_id import resolve_run_id_factory
from command_lambda.sqs_publisher import build_publisher
from command_lambda.submit import submit_command
from command_lambda.validate import validate_command

log = logging.getLogger("command_lambda.local_dev")
logging.basicConfig(level=logging.INFO, format="%(levelname)s %(name)s: %(message)s")

app = FastAPI(title="Incident Pilot — Command Lambda (local shim)", version="0.1.0")

# Build the publisher once at import time. If SQS_QUEUE_URL is unset we fall back
# to an in-memory recorder so `fastapi dev` works with zero configuration.
_publisher: CommandPublisher


class _RecordingPublisher:
    """In-memory publisher used when no real SQS queue is configured.

    Logs the message body so a developer can eyeball the contract the future
    ECS worker will consume, without any AWS dependency.
    """

    def __init__(self) -> None:
        self.published: list[EnqueuedMessage] = []

    def publish(self, message: EnqueuedMessage) -> None:
        self.published.append(message)
        log.info("ENQUEUED (in-memory) run_id=%s body=%s",
                 message.run_id, json.dumps(message.to_dict()))


def _init_publisher() -> CommandPublisher:
    if os.environ.get("SQS_QUEUE_URL"):
        try:
            return build_publisher()
        except Exception as exc:  # noqa: BLE001
            log.warning("SQS publisher build failed (%s); using in-memory recorder", exc)
    return _RecordingPublisher()


_publisher = _init_publisher()


@app.get("/health/live", tags=["Health"])
def liveness() -> dict:
    return {"status": "UP", "publisher": type(_publisher).__name__}


@app.post("/runs", status_code=202, tags=["Runs"])
async def submit_run(request: Request) -> JSONResponse:
    raw = await request.json()

    outcome = validate_command(raw)
    if hasattr(outcome, "fields"):  # ValidationError -> 400
        fields = [{"path": fe.path, "message": fe.message} for fe in outcome.fields]
        return JSONResponse(
            status_code=400,
            content={"error": {"code": "INVALID_COMMAND", "fields": fields}},
        )

    idempotency_key = request.headers.get("x-idempotency-key")
    run_id_factory = resolve_run_id_factory(idempotency_key)

    try:
        result = submit_command(outcome, _publisher, run_id_factory)
    except Exception as exc:  # noqa: BLE001
        log.exception("submission failed")
        return JSONResponse(
            status_code=500,
            content={"error": {"code": "SUBMISSION_FAILED", "message": "submission failed"}},
        )

    return JSONResponse(status_code=202, content=result.to_dict())
