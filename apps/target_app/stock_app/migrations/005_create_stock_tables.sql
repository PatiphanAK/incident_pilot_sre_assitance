-- 005_create_stock_tables
-- Creates the products and audit_logs tables inside the stock_db database.
-- Run AFTER 004_create_stock_db.
-- Safe to run again: IF NOT EXISTS makes it a no-op when the tables already exist.
-- Table names are fully qualified (stock_db.public.<table>), so this file works
-- no matter which database of the cluster you are connected to.
--
-- stock_db is its own database: it holds no reference to target_app (users) or to
-- order_db. Services stay decoupled and talk only by id, never by cross-DB foreign
-- key. This is deliberate — see order_db's order_items.product_id (no REFERENCES).

-- 2. Products
CREATE TABLE IF NOT EXISTS stock_db.public.products (
    id         UUID PRIMARY KEY DEFAULT (gen_random_uuid()),
    name       VARCHAR(255) NOT NULL,
    quantity   INT NOT NULL DEFAULT 0,
    price      DECIMAL(10, 2) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT (now()),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT (now())
);

-- 3. Audit Log (keeps history for the Stock side)
CREATE TABLE IF NOT EXISTS stock_db.public.audit_logs (
    id           UUID PRIMARY KEY DEFAULT (gen_random_uuid()),
    action       VARCHAR(100) NOT NULL,
    entity_type  VARCHAR(50) NOT NULL,
    entity_id    UUID NOT NULL,
    performed_by UUID,
    details      JSONB,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT (now())
);
