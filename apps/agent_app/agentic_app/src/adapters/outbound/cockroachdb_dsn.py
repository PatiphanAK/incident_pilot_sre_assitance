"""Shared conninfo builder for the CockroachDB outbound adapters.

Both the memory adapter and the runbook adapter connect with the same
``COCKROARCH_*`` environment (typo prefix kept on purpose — it is
consistent everywhere), so the DSN assembly lives here once.
"""

from __future__ import annotations

import os


def build_dsn() -> str:
    """Build a psycopg conninfo string from the ``COCKROARCH_*`` env vars."""
    user = os.environ.get("COCKROARCH_USER")
    password = os.environ.get("COCKROARCH_DB_PASS")
    host = os.environ.get("COCKROARCH_DB_HOST")
    if not (user and password and host):
        raise RuntimeError(
            "Incomplete CockroachDB credentials in the environment; "
            "COCKROARCH_USER, COCKROARCH_DB_PASS and COCKROARCH_DB_HOST are required."
        )
    port = os.environ.get("COCKROARCH_DB_PORT", "26257")
    dbname = os.environ.get("COCKROARCH_DB_NAME", "defaultdb")
    sslmode = os.environ.get("COCKROARCH_DB_SSLMODE", "require")
    params = [f"sslmode={sslmode}"]
    sslrootcert = os.environ.get("COCKROARCH_DB_SSLROOTCERT")
    if sslrootcert:
        params.append(f"sslrootcert={sslrootcert}")
    if "://" in host:  # allow a full DSN to be provided directly
        return host + ("?" if "?" not in host else "&") + "&".join(params)
    return f"postgresql://{user}:{password}@{host}:{port}/{dbname}?{'&'.join(params)}"
