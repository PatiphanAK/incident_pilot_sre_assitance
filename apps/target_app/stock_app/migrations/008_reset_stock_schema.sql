-- 008_reset_stock_schema
-- Resets the business schema (stock_db + order_db) to the finalized ER design:
-- a minimal Product + Inventory model plus the order tables (007 shape).
-- Run AFTER 007_create_order_tables.
--
-- Safe to run twice: IF EXISTS guards mean a missing table is not an error.
-- NOTE: re-running re-drops and re-creates the NEW tables (products,
-- inventory, orders, order_items), erasing any rows added since the previous
-- run. This is a reset migration — that is intended behaviour, not a bug.
--
-- The audit_logs tables (stock_db and order_db) are dropped but deliberately
-- NOT recreated: no application code references them, and the MVP stays minimal.
--
-- Table names are fully qualified (stock_db.public.<table> /
-- order_db.public.<table>), so this file works no matter which database of the
-- cluster you are connected to.
--
-- What this file does NOT touch:
--   * target_app.public.users (authentication) — kept intact. Nothing dropped
--     here references users, and nothing references users, so dropping the
--     business tables cannot affect authentication data.
--   * The databases themselves (target_app, stock_db, order_db) — only tables
--     are dropped/created, never databases.
--
-- Foreign-key dependencies are handled explicitly by drop order:
--   * inventory.product_id  -> products.id   (inventory dropped before products)
--   * order_items.order_id  -> orders.id     (order_items dropped before orders)
-- The whole reset runs in ONE transaction, so a failure cannot leave a
-- half-reset schema behind (CockroachDB DDL is transactional). The client's
-- autocommit_before_ddl setting would otherwise auto-commit before the first
-- DDL and silently break that atomicity, so it is turned off for this session.

SET autocommit_before_ddl = false;

BEGIN;

-- 1. Drop obsolete business tables (FK children before their parents).
-- stock_db: the pre-ER products table (quantity embedded, no sku/description)
-- and its audit log (no application code references it).
DROP TABLE IF EXISTS stock_db.public.inventory;   -- only present after a previous run of THIS file
DROP TABLE IF EXISTS stock_db.public.audit_logs;
DROP TABLE IF EXISTS stock_db.public.products;

-- order_db: the pre-reset order model (recreated in section 4 below).
-- order_items is dropped first because of its FK to orders.
DROP TABLE IF EXISTS order_db.public.order_items;
DROP TABLE IF EXISTS order_db.public.audit_logs;
DROP TABLE IF EXISTS order_db.public.orders;

-- 2. Products (catalog + pricing).
CREATE TABLE stock_db.public.products (
    id          UUID PRIMARY KEY DEFAULT (gen_random_uuid()),
    sku         STRING NOT NULL UNIQUE,
    name        STRING NOT NULL,
    description STRING,
    price       DECIMAL(10, 2) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT (now()),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT (now()),
    CONSTRAINT products_price_non_negative CHECK (price >= 0)
);

-- 3. Inventory (stock levels), one row per product:
-- the PRIMARY KEY on product_id enforces the 1:1 cardinality, and the FK keeps
-- it consistent with products (delete a product -> its stock row goes with it).
CREATE TABLE stock_db.public.inventory (
    product_id  UUID PRIMARY KEY REFERENCES stock_db.public.products(id) ON DELETE CASCADE,
    quantity    INT NOT NULL DEFAULT 0,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT (now()),
    CONSTRAINT inventory_quantity_non_negative CHECK (quantity >= 0)
);

-- 4. Orders (order_db) — same shape as 007_create_order_tables. order_db is its
-- own database and keeps NO reference to stock_db or target_app:
--   * order_items.product_id keeps the product's id BY VALUE only (no cross-DB FK).
--   * orders.user_id keeps the user's id BY VALUE only (no cross-DB FK).
CREATE TABLE order_db.public.orders (
    id          UUID PRIMARY KEY DEFAULT (gen_random_uuid()),
    user_id     UUID NOT NULL,
    status      VARCHAR(50) NOT NULL DEFAULT 'PENDING',
    total_price DECIMAL(10, 2) NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT (now())
);

-- order_id references orders within order_db (allowed: same database).
CREATE TABLE order_db.public.order_items (
    id                UUID PRIMARY KEY DEFAULT (gen_random_uuid()),
    order_id          UUID NOT NULL REFERENCES order_db.public.orders(id),
    product_id        UUID NOT NULL,
    quantity          INT NOT NULL,
    price_at_purchase DECIMAL(10, 2) NOT NULL
);

COMMIT;
