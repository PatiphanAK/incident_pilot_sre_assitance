-- 006_create_order_db
-- Creates the order_db database for the order service. Run AFTER 005_create_stock_tables.
-- Safe to run again: IF NOT EXISTS makes it a no-op when the database already exists.
-- You can run it while connected to any database in the cluster (e.g. "defaultdb").

CREATE DATABASE IF NOT EXISTS order_db;
