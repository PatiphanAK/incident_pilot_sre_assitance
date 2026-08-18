-- 001_create_observed_incidents.sql
--
-- Long-term memory (RAG) for the agentic app: one row per observed
-- incident, with the embedding of its summary for vector search.
--
-- Run this while connected to the database the app uses, i.e. the value of
-- COCKROARCH_DB_NAME in .env (defaultdb today). The adapter references the
-- table without a database qualifier, so the database selected in the
-- session is where the table will live.
--
-- Safe to run twice (IF NOT EXISTS).

CREATE TABLE IF NOT EXISTS observed_incidents (
    id UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    summary STRING NOT NULL,
    resolution STRING,
    embedding VECTOR(1536) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT now()
);
