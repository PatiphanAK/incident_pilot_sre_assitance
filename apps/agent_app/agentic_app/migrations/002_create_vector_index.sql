-- 002_create_vector_index.sql
--
-- Vector index on observed_incidents.embedding. This is what lets
-- `ORDER BY embedding <-> $1 LIMIT k` (the search in
-- adapters/outbound/memory/cockroachdb_adapter.py) use the index instead of
-- scanning the whole table.
--
-- Euclidean (L2) distance is the only metric the index supports today, which
-- is exactly what the `<->` operator in the adapter computes.
--
-- Safe to run twice (IF NOT EXISTS). Requires CockroachDB >= 25.2.

CREATE VECTOR INDEX IF NOT EXISTS observed_incidents_embedding
    ON observed_incidents (embedding);
