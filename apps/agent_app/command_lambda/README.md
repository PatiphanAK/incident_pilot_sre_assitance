# Incident Pilot — Command Lambda

A **thin, DB-free command-submission layer**: API Gateway → Lambda → SQS.
The Lambda receives an incident command, validates it, generates a `run_id`,
enqueues a job to SQS, and returns `202` in seconds. The 5–20 min agent loop
runs later on the ECS Fargate worker (`apps/agent_app/backend_run`).

> This is the load-bearing decision from [`backend_run/README.md`](../backend_run/README.md):
> **Lambda is not the Orchestrator.** Run statuses are written by the *worker*,
> not the Lambda. So this Lambda does not create `agent_runs` — the SQS message
> it publishes carries everything the future worker needs to bootstrap the run
> row (idempotent on `run_id`).
>
> It lives as a **sibling to `backend_run/`** because `backend_run/README.md`
> explicitly scopes the Lambda *out* of `backend_run/`, and `agent_app/README.md`
> lists "API Gateway + Main Backend Lambda (command layer)" as part of the
> platform not yet scaffolded. This module implements it.
>
> **Status: implemented & locally verified.** Unit tests pass (`uv run pytest`,
> 43 tests) and the FastAPI shim is exercised end-to-end with `curl`. The SAM
> build / `sam local` / `sam deploy` paths require Docker / an AWS account and
> are left for the operator to run.

## Stack

- **AWS SAM** (deploy + `sam local start-api` demo of the full API GW→Lambda path, no AWS account).
- **Python 3.13, arm64, 256 MB, 10 s timeout** Lambda.
- **stdlib-only runtime** (boto3 comes from the Lambda runtime) — zero dependency packaging, fast cold starts.
- **Standard SQS queue** (not FIFO) + a DLQ for poison messages.
- **Least-privilege IAM**: the Lambda role allows only `sqs:SendMessage` to the
  specific queue + CloudWatch Logs.

## Layout

```text
command_lambda/
├── README.md                         # this file
├── pyproject.toml                    # uv: dev deps only (runtime = stdlib + boto3 from Lambda)
├── samconfig.toml                    # sam deploy defaults
├── template.yaml                     # SAM: API Gateway + Lambda + SQS + DLQ
├── .env.example                      # SQS_QUEUE_URL, SQS_QUEUE_ARN, AWS_REGION
├── Makefile                          # dev / test / build / local / deploy
├── events/post_runs.json             # sample API GW proxy event for `sam local invoke`
├── src/command_lambda/               # CodeUri for SAM
│   ├── __init__.py
│   ├── domain.py                     # Command models (dataclasses) + CommandPublisher Protocol + errors
│   ├── validate.py                   # validate_command(raw) → ParsedCommand | ValidationError  (SHARED core, stdlib)
│   ├── submit.py                     # submit_command(parsed, publisher, run_id_factory, now) → SubmissionResult
│   ├── sqs_publisher.py              # boto3 adapter implementing CommandPublisher
│   ├── run_id.py                     # uuid4 factory; honors optional X-Idempotency-Key
│   ├── apigw.py                      # parse API Gateway proxy event → raw dict + headers
│   ├── handler.py                    # lambda_handler: parse → validate → submit → 202/400/500
│   └── response.py                   # HTTP envelopes (202, 400, 500)
├── local_dev.py                      # FastAPI shim reusing validate+submit (no Docker needed)
└── tests/
    ├── conftest.py                   # FakePublisher fixture
    ├── test_validate.py
    ├── test_submit.py
    └── test_handler.py
```

### Hexagonal, flattened for a thin layer

`domain.py` = models/ports/errors (no AWS/IO imports); `validate.py` +
`submit.py` = application use cases; `sqs_publisher.py` / `run_id.py` /
`apigw.py` / `handler.py` = infrastructure/adapters. The repo convention is
`domain/application/infrastructure/` *subfolders*, but for a ~8-file thin layer
that's ceremony without benefit — flat naming preserves the same intent and
testability (trivial to restructure into subfolders later).

`validate_command()` is the **single source of truth**, reused by both the
Lambda `handler.py` and the FastAPI `local_dev.py` shim — one validator, two
HTTP adapters.

## HTTP contract

```text
POST /runs
  202 { "run_id": "...", "status": "queued", "submitted_at": "iso8601" }
  400 { "error": { "code": "INVALID_COMMAND", "fields": [{"path":"...","message":"..."}] } }
  500 { "error": { "code": "SUBMISSION_FAILED", "message": "submission failed" } }
```

- **Strict validation** (security principle): unknown fields are rejected,
  `incident.description` is required and length-capped, `severity` is an enum
  (with aliases), signals are count/length-capped and de-duplicated.
- **Fail closed**: if the SQS send fails, the Lambda returns `500` — never a
  lying `202` for an un-enqueued run. No internal details leak in the error
  envelope.

### Validation rules (exact)

Enforced by `validate_command()` in `validate.py`; all field problems in one
pass so a client sees the complete picture in a single round-trip.

| Field                         | Rule                                                                                  |
| ----------------------------- | ------------------------------------------------------------------------------------- |
| top-level keys                | only `incident` (required), `session_id` (optional); unknown keys rejected           |
| `incident.description`        | required, string, non-empty after strip, ≤ 4096 chars                                 |
| `incident.severity`           | optional, one of `low medium high critical unknown` (default `unknown`); aliases below |
| `incident.service`            | optional string/null, ≤ 256 chars; blank → `null`                                     |
| `incident.region`             | optional string/null, ≤ 256 chars; blank → `null`                                     |
| `incident.signals`            | optional array of strings; ≤ 64 entries; each non-empty, ≤ 256 chars; de-duped, order preserved |
| `session_id`                  | optional string/null, ≤ 256 chars; blank → `null`                                     |

`severity` aliases (lowercased before lookup, then normalized to canonical):

```text
warn | warning -> low      sev1 -> critical      sev2 -> high
sev3 -> medium             sev4 | sev5 -> low
```

`X-Idempotency-Key` (optional header): must match `^[A-Za-z0-9._\-:]{1,128}$`.
A valid key **overrides** the server `run_id` (client-controlled dedup). An
absent or invalid key is ignored — the server generates a uuid4 `run_id`
instead (invalid keys do **not** cause a `400`; idempotency is a client
convenience, not a correctness requirement on the server side).

### Example request

```bash
curl -X POST localhost:8000/runs \
  -H 'Content-Type: application/json' \
  -H 'X-Idempotency-Key: client-run-001' \
  -d '{
        "incident": {
          "description": "high 5xx rate on checkout-api in us-east-1",
          "severity": "high",
          "service": "checkout-api",
          "region": "us-east-1",
          "signals": ["5xx-spike", "latency-p99", "5xx-spike"]
        },
        "session_id": "sess-demo"
      }'
# => 202 {"run_id":"client-run-001","status":"queued","submitted_at":"2026-08-06T14:00:00Z"}
```

## SQS message contract (interface with the future ECS worker)

Published by `sqs_publisher.py`, consumed later by `backend_run`'s worker:

```json
{
  "run_id": "uuid4",
  "session_id": null,
  "incident": {
    "description": "string (required)",
    "severity": "low|medium|high|critical|unknown",
    "service": "string|null",
    "region": "string|null",
    "signals": ["string"]
  },
  "submitted_at": "iso8601",
  "source": "command_lambda"
}
```

- `run_id` is a server-generated uuid4 (CockroachDB-friendly random UUID,
  avoids hot spots). An optional client `X-Idempotency-Key` header overrides it
  for client-controlled dedup. `run_id` is the **dedup key** the worker uses
  for idempotent SQS redelivery.
- `session_id` is optional — if absent, the worker creates a session. This
  keeps the Lambda DB-free while preserving the spec's session concept.
- SQS message attribute `run_id` (string) is attached so the worker can
  filter/inspect without parsing the body.

## IAM

The Lambda execution role allows only:

- `sqs:SendMessage` on `RunQueue` (scoped by `SQSSendMessagePolicy` to the specific queue),
- `logs:CreateLogGroup`/`CreateLogStream`/`PutLogEvents` on its own log group.

Nothing else — no DB, no other SQS actions, no read access to other services.

## Local development

The runtime code is stdlib-only; dev deps (pytest, fastapi, boto3) are
installed by `uv` into a local venv. `make <target>` wraps the common flows
(see `Makefile`: `dev`, `test`, `build`, `local`, `invoke`, `deploy`).

```bash
cd apps/agent_app/command_lambda
uv sync                         # install dev deps

# 1) unit tests (no network, no Docker)
uv run pytest -q
make test

# 2) FastAPI shim (no Docker, no AWS) — reuses the exact validate+submit path.
#    Zero-config: with SQS_QUEUE_URL unset, an in-memory recorder logs each
#    enqueued message body (so you can eyeball the SQS contract). GET /health/live
#    reports which publisher is active.
uv run fastapi dev local_dev.py          # serves on :8000
curl localhost:8000/health/live          # => {"status":"UP","publisher":"_RecordingPublisher"}
curl -X POST localhost:8000/runs -H 'Content-Type: application/json' \
  -d '{"incident":{"description":"high 5xx on checkout"}}'
# => 202 + run_id; the shim logs the enqueued message body.
# To verify a live send: export SQS_QUEUE_URL (and AWS_REGION) pointing at a
# real or LocalStack queue, restart the shim, and re-run the curl.

# 3) SAM local API (needs Docker) — full API Gateway → Lambda path
sam build && sam local start-api        # serves on :3000
curl -X POST localhost:3000/runs -H 'Content-Type: application/json' \
  -d '{"incident":{"description":"high 5xx on checkout","severity":"high"}}'

# 3b) single invoke with the sample event (events/post_runs.json)
sam build && sam local invoke CommandFunction --event events/post_runs.json
```

## Deploy

```bash
sam deploy --guided
# then, with the deployed ApiUrl from `sam deploy` Outputs:
curl -X POST <ApiUrl> -H 'Content-Type: application/json' \
  -d '{"incident":{"description":"high 5xx on checkout"}}'
# => 202

# confirm the message landed:
aws sqs receive-message --queue-url <RunQueueUrl> --max-number-of-messages 5
```

`samconfig.toml` holds deploy defaults (stack name, region, capabilities).
`s3_bucket` must be set to a bucket you own before a non-guided deploy.

## What this Lambda does *not* do (by design)

- **No DB** — it does not create `agent_runs`. The worker creates the run row
  on first pickup (idempotent on `run_id`).
- **No status reads** — `GET /runs/{id}/status` would need a DB read and is
  deferred to keep the Lambda thin.
- **No agent orchestration** — that is the ECS worker (the `backend_run` runtime).
  This Lambda defines the SQS message contract that worker will consume.

## Auth (demo only)

API Gateway auth is `NONE` (`ApiKeyRequired: false`) for the demo, exposed via
the `CorsOrigin` parameter (default `*`). **Lock down** (Cognito user pool /
IAM / Lambda authorizer + a restrictive CORS origin) before any production use.

## Verification

1. `uv run pytest` — valid command → 202 + run_id; missing/empty `description`
   → 400; unknown field → 400; bad `severity` → 400; oversized `signals` → 400;
   SQS failure → 500; `X-Idempotency-Key` honored; `FakePublisher` asserts the
   exact message body shape.
2. `uv run fastapi dev local_dev.py` → curl `POST /runs` → 202 + run_id; shim
   logs the enqueued message.
3. `sam build && sam local start-api` → same curl against `:3000/runs` → 202.
4. `sam deploy --guided` → curl the deployed `ApiUrl` → 202; confirm the
   message via `aws sqs receive-message`.

## Source of truth

- [`../backend_run/README.md`](../backend_run/README.md) — V1 architecture & design (why the Lambda is thin).
- [`../README.md`](../README.md) — agent app overview & navigation.
- [`../../../README.md`](../../../README.md) — monorepo overview.
