"""Domain models, ports, and errors for the command-submission layer.

Pure stdlib. No AWS, no IO, no framework imports. The Lambda handler and the
FastAPI local shim both build on these primitives so the *one* validator
(``validate_command``) and *one* use case (``submit_command``) serve two HTTP
adapters.

Layout follows hexagonal intent without ceremony:

- **Models** are immutable dataclasses carrying a parsed, validated command and
  the SQS message body to be published.
- **Ports** are ``typing.Protocol``\\ s (``CommandPublisher``,
  ``RunIdFactory``) the application layer depends on, so it can be tested with
  fakes and wired to boto3 in production.
- **Errors** are value objects (not raised) carrying structured validation
  failures so the adapter can map them to a ``400`` envelope.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Callable, Mapping, Protocol, Sequence

# --- Constants (the single source of truth for validation rules) -------------

#: Allowed severity levels. ``unknown`` is permitted so callers can submit a
#: command before triage has determined a severity.
SEVERITIES: frozenset[str] = frozenset(
    {"low", "medium", "high", "critical", "unknown"}
)

#: Lowercased severity aliases accepted on input and normalized to canonical form.
SEVERITY_ALIASES: Mapping[str, str] = {
    "warn": "low",
    "warning": "low",
    "sev1": "critical",
    "sev2": "high",
    "sev3": "medium",
    "sev4": "low",
    "sev5": "low",
}

#: Top-level keys allowed in a command body. Unknown keys are rejected (strict).
ALLOWED_TOP_KEYS: frozenset[str] = frozenset({"incident", "session_id"})

#: Keys allowed inside the ``incident`` object.
ALLOWED_INCIDENT_KEYS: frozenset[str] = frozenset(
    {"description", "severity", "service", "region", "signals"}
)

#: ``incident.description`` must be non-empty after strip and <= this many chars.
MAX_DESCRIPTION_LEN: int = 4096

#: Each ``incident.signal`` must be non-empty and <= this many chars.
MAX_SIGNAL_LEN: int = 256

#: Cap on the number of signals to bound message size.
MAX_SIGNALS: int = 64

#: Max length for ``incident.service`` / ``incident.region`` / ``session_id``.
MAX_FIELD_LEN: int = 256

#: Identifier of the producer, stamped into every SQS message.
SOURCE: str = "command_lambda"


# --- Errors -------------------------------------------------------------------

@dataclass(frozen=True)
class FieldError:
    """A single validation failure scoped to a JSON Pointer-ish field path.

    ``path`` uses dot/bracket notation relative to the request body, e.g.
    ``"incident.description"`` or ``"incident.signals[2]"``.
    """

    path: str
    message: str


@dataclass(frozen=True)
class ValidationError:
    """Structured outcome of a failed :func:`validate_command`.

    Carried (not raised) so the adapter can translate it to a ``400`` envelope
    with a precise ``fields`` list. Aggregate all field errors in one pass so a
    client gets the complete picture in a single round-trip.
    """

    fields: tuple[FieldError, ...]

    @property
    def code(self) -> str:
        return "INVALID_COMMAND"


# --- Models -------------------------------------------------------------------

@dataclass(frozen=True)
class Incident:
    """A validated incident payload (normalized, trimmed)."""

    description: str
    severity: str            # canonical, lowercased, one of SEVERITIES
    service: str | None      # None when absent
    region: str | None
    signals: tuple[str, ...] # de-duplicated, order-preserved, each non-empty


@dataclass(frozen=True)
class ParsedCommand:
    """Output of :func:`validate_command` — a validated command ready to submit."""

    incident: Incident
    session_id: str | None   # None when absent -> worker creates a session


@dataclass(frozen=True)
class EnqueuedMessage:
    """The exact SQS message body to publish.

    Built by :func:`submit_command` and asserted verbatim by tests / the
    FakePublisher. This is the **SQS message contract** the future ECS worker
    consumes; see ``README.md``.
    """

    run_id: str
    session_id: str | None
    incident: Incident
    submitted_at: str        # ISO-8601 UTC
    source: str

    def to_dict(self) -> dict:
        """Serialize to the JSON-serializable contract dict (dict order stable)."""
        return {
            "run_id": self.run_id,
            "session_id": self.session_id,
            "incident": {
                "description": self.incident.description,
                "severity": self.incident.severity,
                "service": self.incident.service,
                "region": self.incident.region,
                "signals": list(self.incident.signals),
            },
            "submitted_at": self.submitted_at,
            "source": self.source,
        }


@dataclass(frozen=True)
class SubmissionResult:
    """Outcome of :func:`submit_command` — the data for a ``202`` envelope."""

    run_id: str
    status: str = "queued"
    submitted_at: str = ""

    def to_dict(self) -> dict:
        return {
            "run_id": self.run_id,
            "status": self.status,
            "submitted_at": self.submitted_at,
        }


# --- Ports --------------------------------------------------------------------

class CommandPublisher(Protocol):
    """Port: publish an :class:`EnqueuedMessage` to the async execution queue.

    The SQS adapter (:mod:`command_lambda.sqs_publisher`) implements this; tests
    use :class:`~command_lambda.tests.conftest.FakePublisher`. Raising any
    exception signals submission failure -> the handler returns ``500``.
    """

    def publish(self, message: EnqueuedMessage) -> None: ...


RunIdFactory = Callable[[], str]
"""Port: produce a fresh run id. Defaults to a uuid4 factory; the adapter may
override it with a client-supplied ``X-Idempotency-Key`` for dedup."""


UtcNow = Callable[[], str]
"""Port: produce an ISO-8601 UTC timestamp string. Injected so tests are
deterministic and the Lambda never calls wall-clock directly."""

__all__ = [
    "SEVERITIES",
    "SEVERITY_ALIASES",
    "ALLOWED_TOP_KEYS",
    "ALLOWED_INCIDENT_KEYS",
    "MAX_DESCRIPTION_LEN",
    "MAX_SIGNAL_LEN",
    "MAX_SIGNALS",
    "MAX_FIELD_LEN",
    "SOURCE",
    "FieldError",
    "ValidationError",
    "Incident",
    "ParsedCommand",
    "EnqueuedMessage",
    "SubmissionResult",
    "CommandPublisher",
    "RunIdFactory",
    "UtcNow",
]

# Re-exported for convenience so callers can import `Sequence` from the domain.
_ = Sequence  # noqa: F841  (keeps the import meaningful for type-checkers)
