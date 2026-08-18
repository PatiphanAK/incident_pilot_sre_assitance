-- 009_add_inventory
-- Catches stock_db up to the 008 shape, for clusters that still have the
-- pre-008 layout (products with an embedded `quantity` column + audit_logs,
-- no sku/description, no inventory table).
--
-- What it does:
--   1. Adds the `sku` and `description` columns the app expects (008 shape).
--   2. Backfills unique sku values ('migrated-<id>') for legacy rows.
--   3. Creates the UNIQUE index on sku — with the same index name 008's inline
--      UNIQUE produces (products_sku_key). The app relies on this constraint:
--      duplicate-sku inserts must fail with SQLSTATE 23505 so the API returns
--      409 "sku already exists" instead of a 500.
--   4. Adds 008's price CHECK constraint.
--   5. Creates the `inventory` table (008 shape): one row per product, FK keeps
--      it consistent (delete a product -> its stock row goes with it).
--
-- Legacy stock levels (the old products.quantity values) are moved by
-- 900_legacy_migrate_quantity.sql — run that file on legacy clusters too.
--
-- Safe to run on BOTH shapes, and safe to run twice (every statement is
-- idempotent, so a partially-applied run simply re-runs to completion):
--   * 001-008 (fresh) cluster: every statement is a no-op (IF NOT EXISTS /
--     guarded UPDATE), so 009 can simply be the last file in the numbered
--     sequence in every environment.
--   * legacy (001-007) cluster: brings stock_db up to the 008 shape (except
--     the quantity migration, which is 900's job).
-- Table names are fully qualified (stock_db.public.<table>), so this file
-- works no matter which database of the cluster you are connected to.
--
-- What this file does NOT touch:
--   * stock_db.public.audit_logs (legacy, no application code references it).
--   * stock_db.public.products.quantity (moved/dropped by 900 on legacy).
--   * target_app and order_db entirely.
--
-- Statement layout (two CockroachDB 26.x constraints drive it):
--   * schema_locked can only be set/reset on its own, in its own (implicit)
--     transaction — so the unlock/re-lock pair sits OUTSIDE the DDL
--     transaction. New 26.x tables are schema-locked and refuse changes while
--     locked; on older/unlocked tables both statements are harmless no-ops.
--   * ADD COLUMN ... NOT NULL DEFAULT backfills existing rows in a background
--     job, so the column is unusable inside the transaction that added it.
--     The sku backfill UPDATE and the unique index therefore run as separate
--     statements after that transaction commits; the `cockroach sql` CLI waits
--     for the backfill job between statements. If the file is interrupted,
--     re-run it: every step is a no-op when already applied.
-- The DDL itself runs in ONE transaction, so the schema change cannot be left
-- half-applied. autocommit_before_ddl is turned off so the DDL stays in that
-- transaction instead of auto-committing early.

SET autocommit_before_ddl = false;

-- Standalone (own implicit transaction): required form for schema_locked.
ALTER TABLE stock_db.public.products SET (schema_locked = false);

BEGIN;
-- 1. Columns the app expects. No-ops on a 008 cluster (both exist).
ALTER TABLE stock_db.public.products
    ADD COLUMN IF NOT EXISTS sku STRING NOT NULL DEFAULT '';
ALTER TABLE stock_db.public.products
    ADD COLUMN IF NOT EXISTS description STRING;

-- 4. Parity with 008's CHECK constraint. No-op on a 008 cluster.
ALTER TABLE stock_db.public.products
    ADD CONSTRAINT IF NOT EXISTS products_price_non_negative CHECK (price >= 0);

-- 5. The inventory table (008 shape).
CREATE TABLE IF NOT EXISTS stock_db.public.inventory (
    product_id  UUID PRIMARY KEY REFERENCES stock_db.public.products(id) ON DELETE CASCADE,
    quantity    INT NOT NULL DEFAULT 0,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT (now()),
    CONSTRAINT inventory_quantity_non_negative CHECK (quantity >= 0)
);
COMMIT;

-- 2. Legacy rows all got sku = '' from the DEFAULT above, which would violate
-- the UNIQUE constraint. Give them unique values first. The guard keys off the
-- legacy `quantity` column (still present here — 900 drops it afterwards):
--   * legacy cluster: quantity exists -> backfill every empty sku (even a
--     single legacy row).
--   * 008 cluster: quantity never existed -> no-op, so a manual sku = '' row
--     (the API itself rejects empty skus) is never rewritten.
-- Runs after the transaction above committed (and its backfill job finished),
-- because the column is unusable while it is being backfilled.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM stock_db.information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'products' AND column_name = 'quantity'
    ) THEN
        UPDATE stock_db.public.products
        SET sku = 'migrated-' || id::STRING
        WHERE sku = '';
    END IF;
END $$;

-- 3. Same index name the inline UNIQUE in 008 creates; no-op on a 008 cluster.
-- Created after the backfill so the unique check runs on the final values.
-- (Index names are unqualified; the index lands in the table's schema.)
CREATE UNIQUE INDEX IF NOT EXISTS products_sku_key
    ON stock_db.public.products (sku);

-- Standalone (own implicit transaction): restore the 26.x default.
ALTER TABLE stock_db.public.products SET (schema_locked = true);
