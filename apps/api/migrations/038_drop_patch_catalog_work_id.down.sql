BEGIN;

ALTER TABLE patch ADD COLUMN IF NOT EXISTS catalog_work_id BIGINT;

UPDATE patch SET catalog_work_id = id WHERE catalog_work_id IS NULL;

CREATE INDEX IF NOT EXISTS idx_patch_catalog_work
  ON patch (catalog_work_id) WHERE catalog_work_id IS NOT NULL;

SELECT setval(pg_get_serial_sequence('patch', 'id'), COALESCE((SELECT max(id) FROM patch WHERE id < 1000000000), 1), true);

COMMIT;
