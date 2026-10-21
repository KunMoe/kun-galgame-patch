-- Forgets which pages name a deleted work; they page in lists again and fall
-- out at hydrate time, as before 051.
BEGIN;

ALTER TABLE patch DROP COLUMN IF EXISTS catalog_deleted_reason;
ALTER TABLE patch DROP COLUMN IF EXISTS catalog_deleted_at;

COMMIT;
