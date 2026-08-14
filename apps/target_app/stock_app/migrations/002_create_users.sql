-- 002_create_users
-- Creates the users table inside the target_app database. Run AFTER 001_create_database.
-- Safe to run again: IF NOT EXISTS makes it a no-op when the table already exists.
-- Table names are fully qualified (target_app.public.<table>), so this file works
-- no matter which database of the cluster you are connected to.

CREATE TABLE IF NOT EXISTS target_app.public.users (
	id         STRING PRIMARY KEY,
	username   STRING NOT NULL,
	email      STRING NOT NULL UNIQUE,
	created_at TIMESTAMPTZ NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL
);
