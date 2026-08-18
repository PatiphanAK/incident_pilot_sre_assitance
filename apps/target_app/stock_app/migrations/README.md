# Database migrations (stock_app)

Plain SQL files that set up the database for this app. They are meant to be run
by hand — with DBeaver or the `cockroach sql` CLI — **not** by the application.
The app expects the schema to already exist.

Each service gets its own database, and the services stay decoupled — they talk by
`id`, never by a cross-database foreign key. This app uses the databases
**`target_app`** (users), **`stock_db`** (products + inventory) and
**`order_db`** (orders + order_items).

## Files

| File | What it does |
|------|--------------|
| `001_create_database.sql` | Creates the `target_app` database |
| `002_create_users.sql`   | Creates the `users` table |
| `003_add_password_hash.sql` | Adds the `password_hash` column (bcrypt) to `users` |
| `004_create_stock_db.sql` | Creates the `stock_db` database |
| `005_create_stock_tables.sql` | Creates `products` and `audit_logs` in `stock_db` |
| `006_create_order_db.sql` | Creates the `order_db` database |
| `007_create_order_tables.sql` | Creates `orders`, `order_items` and `audit_logs` in `order_db` |
| `008_reset_stock_schema.sql` | Resets the business schema: drops the obsolete `stock_db`/`order_db` business tables (never touches `users`), creates the final `products` + `inventory` tables, and recreates `orders` + `order_items` (007 shape; `audit_logs` is dropped, not recreated) |
| `009_add_inventory.sql` | Catches a legacy `stock_db` up to the 008 shape **without dropping data**: adds `sku`/`description` to `products` (backfilling unique skus for legacy rows), creates the UNIQUE index + price CHECK from 008, and creates the `inventory` table. No-op on a fresh 001-008 cluster, so it runs last in the normal loop everywhere |
| `900_legacy_migrate_quantity.sql` | **Legacy clusters only:** moves the old embedded `products.quantity` values into `inventory` (preserving stock levels) and drops the column. Deliberately not matched by the `0*.sql` glob, so a fresh cluster never runs it. Run once, after 009 |

## Rules

1. Run the files in number order (`001`, `002`, ...).
2. Never edit a file that has already been run somewhere — add a NEW file with
   the next number instead (e.g. `004_add_orders.sql`).
3. Every file must be safe to run twice (we use `IF NOT EXISTS`). The one
   exception is `900_legacy_migrate_quantity.sql` — a documented one-shot
   catch-up for legacy clusters (its second run fails on purpose; see its
   header and the "Legacy clusters" section below).
4. Because names are fully qualified (`target_app.public.users`), it does not
   matter which database of the cluster you are connected to when running them.

## How to run

### Option A — DBeaver

1. In DBeaver: **Database → New Connection → CockroachDB**.
2. Fill in host / port / user / password (they are inside the `DATABASE_URL`
   from `.env`, e.g. `postgresql://user:pass@host:26257/...`).
3. Connect (any database, e.g. `defaultdb`, is fine).
4. **SQL Editor → Open SQL script** → choose `migrations/001_create_database.sql`.
5. Run the whole script (**Alt+X** / "Execute script").
6. Repeat for `002_create_users.sql`, then `003_add_password_hash.sql`.

### Option B — `cockroach sql` CLI (CockroachCloud)

Use the `DATABASE_URL` from `.env` (any database works — the SQL is fully
qualified). The `0*.sql` glob runs the numbered sequence in order and
deliberately skips `900_*` (legacy-only, see below).

> ⚠️ The loop includes `008_reset_stock_schema.sql`, which **drops and
> recreates the business tables** (data loss). It is for empty/fresh clusters
> only. If `stock_db` already holds products, skip straight to
> "Legacy clusters" below — do NOT run the loop.

```bash
for f in migrations/0*.sql; do
  cockroach sql --url "$DATABASE_URL" -f "$f"
done
```

### Option C — local Docker CockroachDB

```bash
for f in migrations/0*.sql; do
  docker exec -i <container-name> cockroach sql --insecure < "$f"
done
```

### Legacy clusters (still on the pre-008 `stock_db`)

If `stock_db` still has the old `products` table with an embedded `quantity`
column (no `inventory` table), run **only these two files** — run them in
order, and do **not** run `008` on such a cluster (it drops the business
tables and would erase your products/orders):

```bash
cockroach sql --url "$DATABASE_URL" -f migrations/009_add_inventory.sql
cockroach sql --url "$DATABASE_URL" -f migrations/900_legacy_migrate_quantity.sql
```

`009` adds the `sku`/`description` columns (backfilling unique skus like
`migrated-<id>` for legacy rows), the UNIQUE sku index, and the `inventory`
table; `900` moves your stock levels from `products.quantity` into
`inventory` and drops the obsolete column. Both are safe to run when already
applied, except `900`, which fails on a second run (the column is already
dropped) — that error is the expected "done" signal.

## After running the migrations

Point the app's three URLs (in `.env`, or the `stock-app` Secrets Manager
secret in ECS) at the three databases — same host/user/password, only the path
differs:

```bash
# CockroachCloud example
DATABASE_URL='postgresql://user:pass@cluster-1234.xz.aws-region-1.cockroachlabs.cloud:26257/target_app?sslmode=verify-full&sslrootcert=$HOME/.postgresql/root.crt&options=--cluster=cluster-1234'
STOCK_DATABASE_URL='postgresql://user:pass@cluster-1234.xz.aws-region-1.cockroachlabs.cloud:26257/stock_db?sslmode=verify-full&sslrootcert=$HOME/.postgresql/root.crt&options=--cluster=cluster-1234'
ORDER_DATABASE_URL='postgresql://user:pass@cluster-1234.xz.aws-region-1.cockroachlabs.cloud:26257/order_db?sslmode=verify-full&sslrootcert=$HOME/.postgresql/root.crt&options=--cluster=cluster-1234'

# local Docker example
DATABASE_URL='postgresql://root@localhost:26257/target_app?sslmode=disable'
STOCK_DATABASE_URL='postgresql://root@localhost:26257/stock_db?sslmode=disable'
ORDER_DATABASE_URL='postgresql://root@localhost:26257/order_db?sslmode=disable'
```

`STOCK_DATABASE_URL` / `ORDER_DATABASE_URL` fall back to `DATABASE_URL` when
unset, so a single-database setup still works (all tables in one database). In
the prepared-to-split layout above, leaving them unset makes the stock/order
pools connect to `target_app`, where their tables do not exist — the app now
logs `startup.schema_check_failed` for those databases and `/health` reports
them as down instead of silently 500-ing.

## Adding a new migration (developers)

Create the next numbered file (e.g. `008_...`), write idempotent SQL
(`IF NOT EXISTS`, fully qualified names), and run it in every environment
(local Docker, CockroachCloud dev/prod) in order.
