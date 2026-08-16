# Database migrations (agentic_app)

Plain SQL files that set up the database for this app. They are meant to be run
by hand — with DBeaver or the `cockroach sql` CLI — **not** by the application.
The app expects the schema to already exist.

This app keeps everything in a single table, `observed_incidents` (long-term
incident memory for RAG), in the database named by `COCKROARCH_DB_NAME` in
`.env` (**`defaultdb`** as of today). Unlike stock_app there is no dedicated
database per service — and because the adapter
(`src/adapters/outbound/memory/cockroachdb_adapter.py`) references the table
**without** a database qualifier, you must run these files while connected to
the database the app actually uses.

## Files

| File | What it does |
|------|--------------|
| `001_create_observed_incidents.sql` | Creates the `observed_incidents` table (`summary`, `resolution`, 1536-dim `embedding`, `created_at`) |
| `002_create_vector_index.sql` | Creates the vector index on `embedding` (required for `ORDER BY embedding <-> …` to be fast) |

## Rules

1. Run the files in number order (`001`, `002`, ...).
2. Never edit a file that has already been run somewhere — add a NEW file with
   the next number instead (e.g. `003_add_incident_tags.sql`).
3. Every file must be safe to run twice (`IF NOT EXISTS`).
4. The DDL is unqualified on purpose, mirroring the app code: make sure your
   session is connected to the database in `COCKROARCH_DB_NAME`
   (`defaultdb`), not `system` or some other database.

## Prerequisites

- The cluster's **CA certificate** for `sslmode=verify-full`
  (`COCKROARCH_DB_SSLROOTCERT` in `.env`, i.e.
  `infra/cockroach-labs-cloud-ca.cert.pem`). Download it from the CockroachDB
  Cloud console: your cluster → **Connect** → **Download CA cert**. If the
  file contains an HTML page instead of a `-----BEGIN CERTIFICATE-----`
  block, it was saved wrong — re-download it.
- CockroachDB **>= 25.2** (vector indexes). CockroachCloud clusters run a
  recent 25.x/26.x, so this is normally already satisfied.

## How to run

Build the connection URL from the `COCKROARCH_*` values in `.env` the same way
the app does (see `_build_dsn()` in the adapter), pointing at the database the
app uses:

```
postgresql://USER:PASS@HOST:26257/defaultdb?sslmode=verify-full&sslrootcert=infra/cockroach-labs-cloud-ca.cert.pem
```

### Option A — `cockroach sql` CLI (CockroachCloud)

```bash
cockroach sql --url "postgresql://USER:PASS@HOST:26257/defaultdb?sslmode=verify-full&sslrootcert=infra/cockroach-labs-cloud-ca.cert.pem" -f migrations/001_create_observed_incidents.sql
cockroach sql --url "postgresql://USER:PASS@HOST:26257/defaultdb?sslmode=verify-full&sslrootcert=infra/cockroach-labs-cloud-ca.cert.pem" -f migrations/002_create_vector_index.sql
```

### Option B — DBeaver

1. In DBeaver: **Database → New Connection → CockroachDB**.
2. Fill in host / port / user / password (from the `COCKROARCH_*` values in
   `.env`), set the TLS root certificate to `infra/cockroach-labs-cloud-ca.cert.pem`
   and connect to the **`defaultdb`** database.
3. **SQL Editor → Open SQL script** → choose `migrations/001_create_observed_incidents.sql`.
4. Run the whole script (**Alt+X** / "Execute script").
5. Repeat for `002_create_vector_index.sql`.

### Option C — local Docker CockroachDB

If you run a local cluster instead (set `COCKROARCH_DB_NAME` / `COCKROARCH_DB_HOST`
in `.env` accordingly and point the CLI at it):

```bash
docker exec -i <container-name> cockroach sql --insecure -d defaultdb < migrations/001_create_observed_incidents.sql
docker exec -i <container-name> cockroach sql --insecure -d defaultdb < migrations/002_create_vector_index.sql
```

## After running the migrations

Verify while connected to `defaultdb`:

```sql
SHOW INDEXES FROM observed_incidents;   -- expect primary key + observed_incidents_embedding
SELECT count(*) FROM observed_incidents;
```

The app's integration tests (`uv run pytest -m integration`) then exercise the
table end to end (they seed and delete their own rows, so they are safe to
re-run).

## Changing the embedding model

`VECTOR(1536)` must match the configured embedding dimensions
(`EMBEDDING_DIMENSIONS`, default `1536` — see
`src/adapters/outbound/embedding/embedder.py`). A vector column cannot be
resized in place; switching to a model with different dimensions means a new
migration that creates a new column (or table), backfills, and drops the old
one.

## Adding a new migration (developers)

Create the next numbered file (e.g. `003_...`), write idempotent SQL
(`IF NOT EXISTS`, unqualified names like the app uses), and run it in every
environment (local Docker if you use one, CockroachCloud dev/prod) in order —
connected to the database the app uses.
