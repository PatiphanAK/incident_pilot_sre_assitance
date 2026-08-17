-- 900_legacy_migrate_quantity  (LEGACY CLUSTERS ONLY — not part of the 0*.sql loop)
--
-- Moves the legacy per-product `quantity` (the column the pre-008 products
-- table embedded) into the inventory table created by 009_add_inventory.sql,
-- then drops the obsolete column. Run this file AFTER 009, on clusters that
-- still have the pre-008 products table (a `quantity` column).
--
--   * legacy (001-007) cluster: preserves the current stock levels, then drops
--     products.quantity.
--   * 001-008 (fresh) cluster: DO NOT RUN — the quantity column does not exist
--     and the INSERT will fail. It is deliberately named so that it does NOT
--     match the `migrations/0*.sql` glob used by the standard run loop, so a
--     fresh cluster never runs it.
--
-- Run it once: a second run fails (the column is already dropped), which is
-- the expected signal that the migration is done.
--
-- Table names are fully qualified (stock_db.public.<table>), so this file
-- works no matter which database of the cluster you are connected to.

SET autocommit_before_ddl = false;

-- Standalone statement (own implicit transaction): required form for schema_locked.
ALTER TABLE stock_db.public.products SET (schema_locked = false);

BEGIN;

-- Move the legacy stock levels into inventory. Existing inventory rows
-- (created by the app after 009) win — ON CONFLICT DO NOTHING.
INSERT INTO stock_db.public.inventory (product_id, quantity, updated_at)
SELECT id, quantity, updated_at FROM stock_db.public.products
ON CONFLICT (product_id) DO NOTHING;

-- Drop the now-obsolete column.
ALTER TABLE stock_db.public.products DROP COLUMN IF EXISTS quantity;

COMMIT;

-- Standalone statement (own implicit transaction): restore the 26.x default.
ALTER TABLE stock_db.public.products SET (schema_locked = true);
