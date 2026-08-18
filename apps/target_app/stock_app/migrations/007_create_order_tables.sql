-- 007_create_order_tables
-- Creates the orders, order_items and audit_logs tables inside the order_db
-- database. Run AFTER 006_create_order_db.
-- Safe to run again: IF NOT EXISTS makes it a no-op when the tables already exist.
-- Table names are fully qualified (order_db.public.<table>), so this file works no
-- matter which database of the cluster you are connected to.
--
-- order_db is its own database and holds NO reference to stock_db or target_app:
--   * order_items.product_id keeps the product's id BY VALUE only (no cross-DB FK).
--   * order_db.orders.user_id  keeps the user's id   BY VALUE only (no cross-DB FK).
-- The services stay decoupled and talk only by id. The order -> stock reservation
-- happens at the application layer (in-process), never through the database. This is
-- the "prepared to split" stance: each module owns its data, no cross-DB FKs.

-- 4. Orders
CREATE TABLE IF NOT EXISTS order_db.public.orders (
    id          UUID PRIMARY KEY DEFAULT (gen_random_uuid()),
    user_id     UUID NOT NULL,
    status      VARCHAR(50) NOT NULL DEFAULT 'PENDING',
    total_price DECIMAL(10, 2) NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT (now())
);

-- 5. Order Items
-- order_id references orders within order_db (allowed: same database).
-- product_id is kept by value only: deliberately NOT a reference to stock_db, so the
-- order service stays decoupled from the stock service.
CREATE TABLE IF NOT EXISTS order_db.public.order_items (
    id                UUID PRIMARY KEY DEFAULT (gen_random_uuid()),
    order_id          UUID NOT NULL REFERENCES order_db.public.orders(id),
    product_id        UUID NOT NULL,
    quantity          INT NOT NULL,
    price_at_purchase DECIMAL(10, 2) NOT NULL
);

-- 6. Audit Log (keeps history for the Order side)
CREATE TABLE IF NOT EXISTS order_db.public.audit_logs (
    id           UUID PRIMARY KEY DEFAULT (gen_random_uuid()),
    action       VARCHAR(100) NOT NULL,
    entity_type  VARCHAR(50) NOT NULL,
    entity_id    UUID NOT NULL,
    performed_by UUID,
    details      JSONB,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT (now())
);
