-- MOYU-PR7 (M5): prevent two patch_resource rows from claiming the same S3
-- upload. The D10 upload flow has no upload_session table (the s3_key is the
-- only handle), so DB-level uniqueness is the cheapest correct enforcement of
-- the "single-use" invariant. Partial index because:
--   - storage != 's3' rows use Content (external link) instead of S3Key
--   - legacy / future rows may keep S3Key='' as a sentinel
-- so the unique constraint only applies to rows that actually hold an S3 key.
--
-- Prerequisite for prod: if any existing patch_resource has duplicate s3_key
-- under (storage='s3' AND s3_key<>''), this CREATE INDEX fails. Audit first:
--
--   SELECT s3_key, count(*) FROM patch_resource
--     WHERE storage='s3' AND s3_key<>''
--     GROUP BY s3_key HAVING count(*)>1;
--
-- Test env (this project's current target) has no such duplicates.

CREATE UNIQUE INDEX IF NOT EXISTS idx_patch_resource_s3_key_unique
  ON patch_resource(s3_key)
  WHERE storage = 's3' AND s3_key <> '';
