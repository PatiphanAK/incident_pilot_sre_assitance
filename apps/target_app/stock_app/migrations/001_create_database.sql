-- 001_create_database
-- Creates the target_app database. Run this file FIRST, before any other migration.
-- Safe to run again: IF NOT EXISTS makes it a no-op when the database already exists.
-- You can run it while connected to any database in the cluster (e.g. "defaultdb").

CREATE DATABASE IF NOT EXISTS target_app;
