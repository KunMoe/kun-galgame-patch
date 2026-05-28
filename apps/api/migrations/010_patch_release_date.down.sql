DROP INDEX IF EXISTS idx_patch_release_date;
ALTER TABLE patch DROP COLUMN IF EXISTS release_date;
