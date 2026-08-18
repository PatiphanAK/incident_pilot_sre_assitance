-- 004_create_stock_db
-- Creates the stock_db database for the stock service. Run AFTER 003_add_password_hash.
-- Safe to run again: IF NOT EXISTS makes it a no-op when the database already exists.
-- You can run it while connected to any database in the cluster (e.g. "defaultdb").

CREATE DATABASE IF NOT EXISTS stock_db;
