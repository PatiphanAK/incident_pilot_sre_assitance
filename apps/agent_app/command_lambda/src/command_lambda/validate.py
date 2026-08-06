"""Validate an incident command body — the single source of truth.

Hand-rolled, stdlib only (no pydantic) so the Lambda runtime carries **zero
dependencies**: fast cold starts, no wheel/build-container friction. Both the
Lambda ``handler`` and the FastAPI ``local_dev`` shim call :func:`validate_command`,
so there is exactly one validator behind two HTTP adapters.

Strict (security principle):

- reject unknown top-level or ``incident`` keys,
- require non-empty ``incident.description`` within a length cap,
- coerce ``severity`` to a canonical enum (with aliases), else reject,
- cap signal count and per-signal length; de-duplicate preserving order,
- trim strings; ``service``/``region`` default to ``None`` when absent/blank.

All field problems are collected in one pass and returned together so a client
sees the complete picture in one round-trip.
"""

from __future__ import annotations

from .domain import (
    ALLOWED_INCIDENT_KEYS,
    ALLOWED_TOP_KEYS,
    MAX_DESCRIPTION_LEN,
    MAX_FIELD_LEN,
    MAX_SIGNALS,
    MAX_SIGNAL_LEN,
    SEVERITIES,
    SEVERITY_ALIASES,
    FieldError,
    Incident,
    ParsedCommand,
    ValidationError,
)

__all__ = ["validate_command"]


def validate_command(raw: object) -> ParsedCommand | ValidationError:
    """Validate ``raw`` (the parsed JSON body) into a :class:`ParsedCommand`.

    Returns :class:`ValidationError` (carried, not raised) on any problem so the
    adapter can map it to a ``400`` envelope. ``raw`` must be a ``dict``;
    non-dict input is rejected as a top-level ``INVALID_COMMAND``.
    """
    errors: list[FieldError] = []

    if not isinstance(raw, dict):
        return ValidationError(
            (FieldError("", "request body must be a JSON object"),)
        )

    # --- top-level keys: reject unknown --------------------------------------
    for key in raw:
        if key not in ALLOWED_TOP_KEYS:
            errors.append(
                FieldError(key, f"unknown field; allowed: {sorted(ALLOWED_TOP_KEYS)}")
            )

    incident_raw = raw.get("incident")
    if not isinstance(incident_raw, dict):
        # incident is required and must be an object; emit one precise error and
        # return — nothing downstream is parseable.
        errors.append(
            FieldError("incident", "must be a JSON object and is required")
        )
        return ValidationError(tuple(errors))

    # --- incident keys: reject unknown ---------------------------------------
    for key in incident_raw:
        if key not in ALLOWED_INCIDENT_KEYS:
            errors.append(
                FieldError(
                    f"incident.{key}",
                    f"unknown field; allowed: {sorted(ALLOWED_INCIDENT_KEYS)}",
                )
            )

    # --- description (required, non-empty, capped) --------------------------
    description_raw = incident_raw.get("description")
    description = _clean_str(description_raw)
    if description is None:
        errors.append(
            FieldError("incident.description", "is required and must be a string")
        )
    elif description == "":
        errors.append(
            FieldError("incident.description", "must not be empty")
        )
    elif len(description) > MAX_DESCRIPTION_LEN:
        errors.append(
            FieldError(
                "incident.description",
                f"must be at most {MAX_DESCRIPTION_LEN} characters",
            )
        )

    # --- severity (optional, defaults to "unknown"; enum + aliases) ----------
    severity = _normalize_severity(incident_raw.get("severity"), errors)

    # --- service / region (optional, capped) ---------------------------------
    service = _optional_field(incident_raw.get("service"), "incident.service", errors)
    region = _optional_field(incident_raw.get("region"), "incident.region", errors)

    # --- signals (optional list, capped count + length, de-duped) ------------
    signals = _normalize_signals(incident_raw.get("signals"), errors)

    # --- session_id (optional, capped) — validated before the error gate -----
    # so a bad-typed session_id is reported, not silently dropped.
    session_id = _optional_field(raw.get("session_id"), "session_id", errors)

    if errors:
        return ValidationError(tuple(errors))

    # All fields validated; build the immutable model.
    return ParsedCommand(
        incident=Incident(
            description=description,  # type: ignore[arg-type]  # non-empty str when no errors
            severity=severity,
            service=service,
            region=region,
            signals=signals,
        ),
        session_id=session_id,
    )


# --- helpers ------------------------------------------------------------------

def _clean_str(value: object) -> str | None:
    """Return stripped string, or None if value is not a string."""
    if not isinstance(value, str):
        return None
    return value.strip()


def _optional_field(value: object, path: str, errors: list[FieldError]) -> str | None:
    """Coerce an optional capped string field to ``None`` (absent/blank) or a
    trimmed string. Non-string, non-None input is rejected; oversize is rejected."""
    if value is None:
        return None
    if not isinstance(value, str):
        errors.append(FieldError(path, "must be a string or null"))
        return None
    cleaned = value.strip()
    if cleaned == "":
        return None
    if len(cleaned) > MAX_FIELD_LEN:
        errors.append(
            FieldError(path, f"must be at most {MAX_FIELD_LEN} characters")
        )
        return None
    return cleaned


def _normalize_severity(value: object, errors: list[FieldError]) -> str:
    """Return canonical severity. Absent/None -> ``"unknown"``. Accepts aliases."""
    if value is None:
        return "unknown"
    if not isinstance(value, str):
        errors.append(FieldError("incident.severity", "must be a string or null"))
        return "unknown"
    norm = value.strip().lower()
    if norm == "":
        return "unknown"
    if norm in SEVERITIES:
        return norm
    if norm in SEVERITY_ALIASES:
        return SEVERITY_ALIASES[norm]
    errors.append(
        FieldError(
            "incident.severity",
            f"must be one of {sorted(SEVERITIES)} (or an alias), got {norm!r}",
        )
    )
    return "unknown"


def _normalize_signals(value: object, errors: list[FieldError]) -> tuple[str, ...]:
    """Coerce ``signals`` to a de-duplicated tuple of non-empty trimmed strings.

    ``None`` -> empty tuple. Non-list -> error + empty tuple. Oversize count or
    per-item length/emptiness/typed errors are aggregated.
    """
    if value is None:
        return ()
    if not isinstance(value, list):
        errors.append(FieldError("incident.signals", "must be an array of strings"))
        return ()

    if len(value) > MAX_SIGNALS:
        errors.append(
            FieldError(
                "incident.signals",
                f"must contain at most {MAX_SIGNALS} entries",
            )
        )

    out: list[str] = []
    seen: set[str] = set()
    for i, item in enumerate(value):
        path = f"incident.signals[{i}]"
        if not isinstance(item, str):
            errors.append(FieldError(path, "must be a string"))
            continue
        cleaned = item.strip()
        if cleaned == "":
            errors.append(FieldError(path, "must not be empty"))
            continue
        if len(cleaned) > MAX_SIGNAL_LEN:
            errors.append(
                FieldError(path, f"must be at most {MAX_SIGNAL_LEN} characters")
            )
            continue
        if cleaned in seen:
            continue  # de-duplicate, preserve first occurrence's order
        seen.add(cleaned)
        out.append(cleaned)
    return tuple(out)
