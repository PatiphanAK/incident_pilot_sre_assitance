-- 003_add_password_hash
-- Adds the password_hash column (bcrypt) to users. Run AFTER 002_create_users.
-- Safe to run again: IF NOT EXISTS makes it a no-op when the column already exists.
-- Existing rows get '' (empty) — bcrypt comparison against '' can never succeed,
-- so users created before this migration simply cannot log in until they are
-- given a password.

ALTER TABLE target_app.public.users
    ADD COLUMN IF NOT EXISTS password_hash STRING NOT NULL DEFAULT '';
