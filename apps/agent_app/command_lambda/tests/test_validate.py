"""Unit tests for :func:`command_lambda.validate.validate_command`.

Covers the happy path and every rejection case: missing/empty description,
unknown fields, bad severity, oversized signals, type errors, and aliases.
"""

from __future__ import annotations

import pytest

from command_lambda.domain import (
    MAX_DESCRIPTION_LEN,
    MAX_SIGNALS,
    ValidationError,
)
from command_lambda.validate import validate_command


def _paths(error: ValidationError) -> list[str]:
    return [fe.path for fe in error.fields]


# --- happy path ---------------------------------------------------------------

def test_valid_minimal_command_returns_parsed_command():
    result = validate_command({"incident": {"description": "high 5xx on checkout"}})
    assert not hasattr(result, "fields")  # not a ValidationError
    assert result.incident.description == "high 5xx on checkout"
    assert result.incident.severity == "unknown"  # default
    assert result.incident.service is None
    assert result.incident.region is None
    assert result.incident.signals == ()
    assert result.session_id is None


def test_valid_full_command_normalizes_and_dedups_signals():
    result = validate_command(
        {
            "incident": {
                "description": "  db cpu saturated  ",
                "severity": "HIGH",
                "service": "checkout-api",
                "region": "us-east-1",
                "signals": ["  latency-p99  ", "latency-p99", "5xx-spike", "5xx-spike"],
            },
            "session_id": "sess-1",
        }
    )
    assert result.incident.description == "db cpu saturated"
    assert result.incident.severity == "high"  # lowercased
    assert result.incident.service == "checkout-api"
    assert result.incident.region == "us-east-1"
    # de-duplicated, order preserved, trimmed
    assert result.incident.signals == ("latency-p99", "5xx-spike")
    assert result.session_id == "sess-1"


@pytest.mark.parametrize(
    "alias,expected",
    [
        ("warning", "low"),
        ("SEV2", "high"),
        ("sev1", "critical"),
        ("sev4", "low"),
    ],
)
def test_severity_aliases_are_normalized(alias, expected):
    result = validate_command(
        {"incident": {"description": "x", "severity": alias}}
    )
    assert result.incident.severity == expected


# --- rejections (400) --------------------------------------------------------

def test_non_object_body_is_rejected():
    result = validate_command("not an object")
    assert isinstance(result, ValidationError)
    assert _paths(result) == [""]


def test_missing_incident_is_rejected():
    result = validate_command({"session_id": "s"})
    assert isinstance(result, ValidationError)
    assert "incident" in _paths(result)


def test_missing_description_is_rejected():
    result = validate_command({"incident": {"severity": "low"}})
    assert isinstance(result, ValidationError)
    assert "incident.description" in _paths(result)


def test_empty_description_is_rejected():
    result = validate_command({"incident": {"description": "   "}})
    assert isinstance(result, ValidationError)
    assert "incident.description" in _paths(result)


def test_oversized_description_is_rejected():
    result = validate_command(
        {"incident": {"description": "x" * (MAX_DESCRIPTION_LEN + 1)}}
    )
    assert isinstance(result, ValidationError)
    assert "incident.description" in _paths(result)


def test_unknown_top_level_field_is_rejected():
    result = validate_command(
        {"incident": {"description": "x"}, "runbook": "auto-scale"}
    )
    assert isinstance(result, ValidationError)
    assert "runbook" in _paths(result)


def test_unknown_incident_field_is_rejected():
    result = validate_command(
        {"incident": {"description": "x", "oncall": "alice"}}
    )
    assert isinstance(result, ValidationError)
    assert "incident.oncall" in _paths(result)


def test_bad_severity_is_rejected():
    result = validate_command(
        {"incident": {"description": "x", "severity": "on-fire"}}
    )
    assert isinstance(result, ValidationError)
    assert "incident.severity" in _paths(result)


def test_severity_must_be_string():
    result = validate_command(
        {"incident": {"description": "x", "severity": 3}}
    )
    assert isinstance(result, ValidationError)
    assert "incident.severity" in _paths(result)


def test_oversized_signals_list_is_rejected():
    result = validate_command(
        {"incident": {"description": "x", "signals": [f"s{i}" for i in range(MAX_SIGNALS + 1)]}}
    )
    assert isinstance(result, ValidationError)
    assert "incident.signals" in _paths(result)


def test_non_string_signal_is_rejected():
    result = validate_command(
        {"incident": {"description": "x", "signals": ["ok", 42]}}
    )
    assert isinstance(result, ValidationError)
    assert "incident.signals[1]" in _paths(result)


def test_signals_must_be_a_list():
    result = validate_command(
        {"incident": {"description": "x", "signals": "latency"}}
    )
    assert isinstance(result, ValidationError)
    assert "incident.signals" in _paths(result)


def test_service_must_be_string_or_null():
    result = validate_command(
        {"incident": {"description": "x", "service": 123}}
    )
    assert isinstance(result, ValidationError)
    assert "incident.service" in _paths(result)


def test_blank_service_becomes_none():
    result = validate_command(
        {"incident": {"description": "x", "service": "   "}}
    )
    assert not isinstance(result, ValidationError)
    assert result.incident.service is None


def test_multiple_errors_aggregated():
    result = validate_command(
        {"incident": {"description": "", "severity": "lava", "bogus": 1}, "extra": 2}
    )
    assert isinstance(result, ValidationError)
    paths = set(_paths(result))
    assert {
        "incident.description",
        "incident.severity",
        "incident.bogus",
        "extra",
    } <= paths


def test_empty_dict_body_is_rejected():
    result = validate_command({})
    assert isinstance(result, ValidationError)
    assert "incident" in _paths(result)


def test_session_id_must_be_string_or_null():
    result = validate_command(
        {"incident": {"description": "x"}, "session_id": 55}
    )
    assert isinstance(result, ValidationError)
    assert "session_id" in _paths(result)


def test_session_id_blank_becomes_none():
    result = validate_command(
        {"incident": {"description": "x"}, "session_id": "  "}
    )
    assert not isinstance(result, ValidationError)
    assert result.session_id is None
