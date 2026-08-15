# Database migrations (stock_app)

Plain SQL files that set up the database for this app. They are meant to be run
by hand — with DBeaver or the `cockroach sql` CLI — **not** by the application.
The app expects the schema to already exist.

Each service gets its own database, and the services stay decoupled — they talk by
`id`, never by a cross-database foreign key. This app uses the database
**`target_app`** (users) and **`stock_db`** (products + audit log).

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

## Rules

1. Run the files in number order (`001`, `002`, ...).
2. Never edit a file that has already been run somewhere — add a NEW file with
   the next number instead (e.g. `004_add_orders.sql`).
3. Every file must be safe to run twice (we use `IF NOT EXISTS`).
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

Use the `DATABASE_URL` from `.env`:

```bash
cockroach sql --url "$DATABASE_URL" -f migrations/001_create_database.sql
cockroach sql --url "$DATABASE_URL" -f migrations/002_create_users.sql
cockroach sql --url "$DATABASE_URL" -f migrations/003_add_password_hash.sql
cockroach sql --url "$DATABASE_URL" -f migrations/004_create_stock_db.sql
cockroach sql --url "$DATABASE_URL" -f migrations/005_create_stock_tables.sql
cockroach sql --url "$DATABASE_URL" -f migrations/006_create_order_db.sql
cockroach sql --url "$DATABASE_URL" -f migrations/007_create_order_tables.sql
```

### Option C — local Docker CockroachDB

```bash
docker exec -i <container-name> cockroach sql --insecure < migrations/001_create_database.sql
docker exec -i <container-name> cockroach sql --insecure < migrations/002_create_users.sql
docker exec -i <container-name> cockroach sql --insecure < migrations/003_add_password_hash.sql
docker exec -i <container-name> cockroach sql --insecure < migrations/004_create_stock_db.sql
docker exec -i <container-name> cockroach sql --insecure < migrations/005_create_stock_tables.sql
docker exec -i <container-name> cockroach sql --insecure < migrations/006_create_order_db.sql
docker exec -i <container-name> cockroach sql --insecure < migrations/007_create_order_tables.sql
```

## After running the migrations

Point the app's `DATABASE_URL` (in `.env`) at the new database — the URL must
end with `/target_app`:

```bash
# CockroachCloud example
DATABASE_URL='postgresql://user:pass@cluster-1234.xz.aws-region-1.cockroachlabs.cloud:26257/target_app?sslmode=verify-full&sslrootcert=$HOME/.postgresql/root.crt&options=--cluster=cluster-1234'

# local Docker example
DATABASE_URL='postgresql://root@localhost:26257/target_app?sslmode=disable'
```

## Adding a new migration (developers)

Create the next numbered file (e.g. `008_...`), write idempotent SQL
(`IF NOT EXISTS`, fully qualified names), and run it in every environment
(local Docker, CockroachCloud dev/prod) in order.
