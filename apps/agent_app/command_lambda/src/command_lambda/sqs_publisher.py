"""boto3 adapter implementing the :class:`CommandPublisher` port.

This is the one module in the runtime that imports boto3 — which the Lambda
runtime provides, so it stays out of the dependency set. The message body is
the exact :meth:`EnqueuedMessage.to_dict`; the :class:`SqsPublisher` is a thin
adapter over ``sqs_client.send_message``.

Message attributes carry ``run_id`` (string) so the worker can filter/inspect
messages without parsing the body, and so CloudWatch metrics/alarms can be
keyed off it without sampling payload.
"""

from __future__ import annotations

import json
import logging
import os
from typing import Any

from .domain import CommandPublisher, EnqueuedMessage

__all__ = ["SqsPublisher", "build_publisher"]

log = logging.getLogger(__name__)


class SqsPublisher(CommandPublisher):
    """Publish :class:`EnqueuedMessage` bodies to a standard SQS queue."""

    def __init__(self, sqs_client: Any, queue_url: str) -> None:
        self._sqs = sqs_client
        self._queue_url = queue_url

    def publish(self, message: EnqueuedMessage) -> None:
        body = json.dumps(message.to_dict(), separators=(",", ":"))
        log.info(
            "enqueuing run run_id=%s session_id=%s severity=%s",
            message.run_id,
            message.session_id,
            message.incident.severity,
        )
        self._sqs.send_message(
            QueueUrl=self._queue_url,
            MessageBody=body,
            MessageAttributes={
                "run_id": {
                    "DataType": "String",
                    "StringValue": message.run_id,
                },
                "source": {
                    "DataType": "String",
                    "StringValue": message.source,
                },
            },
        )


def build_publisher() -> SqsPublisher:
    """Build a :class:`SqsPublisher` from Lambda env (``SQS_QUEUE_URL``/region).

    Raises ``RuntimeError`` if ``SQS_QUEUE_URL`` is unset — fail fast on a
    misconfigured deploy rather than silently returning 202 with no enqueue.
    """
    import boto3  # local import: keep module importable in unit-test envs
    # that don't have boto3 installed (the SQS adapter is only exercised via
    # the local shim / sam local / real AWS).

    queue_url = os.environ.get("SQS_QUEUE_URL")
    if not queue_url:
        raise RuntimeError(
            "SQS_QUEUE_URL is not configured; cannot enqueue run jobs"
        )
    region = os.environ.get("AWS_REGION") or os.environ.get("AWS_DEFAULT_REGION")
    sqs_client = boto3.client("sqs", region_name=region) if region else boto3.client("sqs")
    return SqsPublisher(sqs_client, queue_url)
