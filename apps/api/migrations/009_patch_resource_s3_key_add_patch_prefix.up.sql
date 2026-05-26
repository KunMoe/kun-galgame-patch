-- Data fix-up: prepend the missing "patch/" segment to legacy patch_resource
-- rows' s3_key.
--
-- Background: the D10 upload path (apps/api/internal/common/upload/service.go
-- buildPatchResourceKey) generates keys shaped as
--     patch/{galgameID}/{random64}/{filename}
-- and the B2 bucket objects are actually stored at those keys. Legacy data
-- import (predating D10) populated patch_resource.s3_key by reverse-deriving
-- it from the legacy content URL and only stripped the host portion
-- (https://oss.moyu.moe/) — it did NOT strip the leading "/patch/" segment,
-- so DB rows ended up shaped as "{galgameID}/{random}/{filename}" with the
-- bucket-root path component missing.
--
-- This bites at download time: S3Client.PublicURL composes
--     publicURL + "/" + s3_key
-- → "https://oss.moyu.moe/{galgameID}/..." which 404s, because the actual
-- bucket key is "patch/{galgameID}/...".
--
-- Fix: prepend "patch/" to every storage='s3' row missing it. Idempotent —
-- the WHERE clause (NOT LIKE 'patch/%') guarantees re-runs are no-ops.
-- Audit table patch_resource_file_history.old_s3_key is treated the same
-- way so historical audit records align with current key shape; rows that
-- were already correct are skipped.
--
-- After this runs:
--   - PatchService.GetResourceDownloadInfo dynamically rebuilds the download
--     URL via S3Client.PublicURL(s3_key) every read, so the fix takes effect
--     immediately without code redeploy / cache invalidation.
--   - The CreateResource s3_key prefix check (HasPrefix "patch/{gid}/")
--     continues to enforce correctness for all new writes.
--
-- Note on syntax: executed via cmd/migrate (db.Exec on the whole file),
-- which speaks the standard postgres wire protocol — NO psql meta-commands
-- like \gset / \echo / \set are available. Counts + post-condition checks
-- live inside DO blocks (plpgsql) which Exec accepts.

-- Wrap as a DO block so we can RAISE NOTICE the row count for the deploy log.
DO $$
DECLARE
  resource_fixed int;
  history_fixed int;
  still_missing int;
BEGIN
  WITH updated AS (
    UPDATE patch_resource
    SET s3_key = 'patch/' || s3_key
    WHERE storage = 's3'
      AND s3_key <> ''
      AND s3_key NOT LIKE 'patch/%'
    RETURNING 1
  )
  SELECT count(*) INTO resource_fixed FROM updated;
  RAISE NOTICE '009: fixed % patch_resource rows', resource_fixed;

  WITH updated AS (
    UPDATE patch_resource_file_history
    SET old_s3_key = 'patch/' || old_s3_key
    WHERE old_storage = 's3'
      AND old_s3_key <> ''
      AND old_s3_key NOT LIKE 'patch/%'
    RETURNING 1
  )
  SELECT count(*) INTO history_fixed FROM updated;
  RAISE NOTICE '009: fixed % patch_resource_file_history rows', history_fixed;

  -- Post-condition guard: zero storage='s3' rows with non-empty s3_key
  -- should still be missing the prefix. Fail loudly if so — the implicit
  -- transaction around db.Exec rolls back and cmd/migrate exits non-zero.
  SELECT count(*) INTO still_missing FROM patch_resource
   WHERE storage='s3' AND s3_key <> '' AND s3_key NOT LIKE 'patch/%';
  IF still_missing > 0 THEN
    RAISE EXCEPTION '009 post-condition failed: % patch_resource rows still missing patch/ prefix', still_missing;
  END IF;
END $$;
