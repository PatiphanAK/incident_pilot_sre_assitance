-- 003_create_runbooks.sql
--
-- Runbook knowledge base for the agentic app: one row per known incident
-- pattern, with the remediation steps (JSONB) and a coarse blast-radius
-- tag the decide node uses to gate automated execution.
--
-- Run this while connected to the database the app uses, i.e. the value of
-- COCKROARCH_DB_NAME in .env (defaultdb today). The adapter references the
-- table without a database qualifier, so the database selected in the
-- session is where the table will live.
--
-- Safe to run twice (IF NOT EXISTS + NOT EXISTS guards on the seeds).

CREATE TABLE IF NOT EXISTS runbooks (
    id UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    incident_pattern STRING NOT NULL,
    steps JSONB NOT NULL,
    blast_radius STRING NOT NULL,
    created_at TIMESTAMPTZ DEFAULT now()
);

-- Demo seed rows. The NOT EXISTS guard makes re-running this file a no-op
-- instead of duplicating patterns (the table has no unique constraint, so
-- this is the idempotency mechanism).
INSERT INTO runbooks (incident_pattern, steps, blast_radius)
SELECT 'connection pool exhaustion', '[
    "Restart the payments-api deployment (kubectl rollout restart deploy/payments-api)",
    "Watch pool utilization until it settles below 80%",
    "If it recurs within 24h, raise max pool size and add a retry budget"
]'::JSONB, 'low'
WHERE NOT EXISTS (SELECT 1 FROM runbooks WHERE incident_pattern = 'connection pool exhaustion');

INSERT INTO runbooks (incident_pattern, steps, blast_radius)
SELECT 'disk usage high', '[
    "Identify the node above 85% disk usage",
    "Compact / archive old logs and temporary files",
    "Add a warning alert at 75% so this is caught earlier next time"
]'::JSONB, 'medium'
WHERE NOT EXISTS (SELECT 1 FROM runbooks WHERE incident_pattern = 'disk usage high');

-- High blast radius on purpose: proves the decide node escalates instead
-- of running automation even when a runbook is known.
INSERT INTO runbooks (incident_pattern, steps, blast_radius)
SELECT 'primary region outage', '[
    "Declare SEV-1 and page the on-call SRE lead",
    "Verify multi-region failover status in the console",
    "No automated remediation — human decision required"
]'::JSONB, 'high'
WHERE NOT EXISTS (SELECT 1 FROM runbooks WHERE incident_pattern = 'primary region outage');
