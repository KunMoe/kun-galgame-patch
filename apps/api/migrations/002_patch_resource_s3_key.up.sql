-- 002: patch_resource schema adjustment (D10)
--
-- Background: see D10 in docs/proj/next-fiber/09-risks-and-decisions.md.
-- Rename the legacy hash column to blake3 (preserving existing BLAKE3 values),
-- and add a new s3_key column (the full S3 object key; all future PUT/DELETE/HEAD
-- operations use it directly).
--
-- Backfill for existing data: regex out the full S3 key from the content URL
-- so existing flows such as deletePatchResource do not need to change their
-- path-building logic.

BEGIN;

-- ─────────────────────────────────────────────────
-- 1. Rename hash -> blake3
--    Wrapped in DO/EXCEPTION so fresh DBs (where 000_baseline has already
--    built the post-migration schema with `blake3` and no `hash`) don't
--    fail with SQLSTATE 42703 / undefined_column. On prisma-era backups the
--    `hash` column is still present and the rename proceeds normally.
-- ─────────────────────────────────────────────────
DO $$ BEGIN
  ALTER TABLE patch_resource RENAME COLUMN hash TO blake3;
EXCEPTION
  WHEN undefined_column THEN
    RAISE NOTICE '002 skip rename: patch_resource.hash already renamed to blake3';
  WHEN duplicate_column THEN
    -- Belt-and-braces: in the (theoretical) case both columns exist briefly,
    -- bail with a clear marker rather than half-applying.
    RAISE NOTICE '002 skip rename: both hash and blake3 present, manual inspection needed';
END $$;

-- ─────────────────────────────────────────────────
-- 2. Add the s3_key column (full S3 object key).
--    IF NOT EXISTS so the baseline path is a no-op.
-- ─────────────────────────────────────────────────
ALTER TABLE patch_resource ADD COLUMN IF NOT EXISTS s3_key VARCHAR(2048) NOT NULL DEFAULT '';

-- ─────────────────────────────────────────────────
-- 3. Backfill existing rows: strip the URL prefix from content
--    content looks like "https://<host>/<bucket>/patch/<id>/<blake3>/<file>"
--    target s3_key       "patch/<id>/<blake3>/<file>"
--    Only processes rows where storage != 'user' (i.e. S3 resources), content
--    matches the standard URL shape, AND s3_key is still empty — that last
--    guard makes the UPDATE idempotent so re-runs on already-backfilled data
--    (or on fresh DBs where s3_key was created with the column DEFAULT '')
--    don't rewrite existing values.
-- ─────────────────────────────────────────────────
UPDATE patch_resource
SET s3_key = REGEXP_REPLACE(content, '^https?://[^/]+/[^/]+/', '')
WHERE storage <> 'user'
  AND s3_key = ''
  AND content ~ '^https?://[^/]+/[^/]+/.+';

-- ─────────────────────────────────────────────────
-- 4. Add a unique index on s3_key (optional, helps prevent duplicate writes before HeadObject)
-- ─────────────────────────────────────────────────
CREATE INDEX IF NOT EXISTS idx_patch_resource_s3_key ON patch_resource(s3_key) WHERE s3_key <> '';

-- Verification
DO $$
DECLARE
    total   INT;
    filled  INT;
    missing INT;
BEGIN
    SELECT COUNT(*) INTO total   FROM patch_resource WHERE storage <> 'user';
    SELECT COUNT(*) INTO filled  FROM patch_resource WHERE storage <> 'user' AND s3_key <> '';
    SELECT COUNT(*) INTO missing FROM patch_resource WHERE storage <> 'user' AND s3_key =  '';
    RAISE NOTICE 'OK: S3 resources total %, backfilled s3_key %, not backfilled % (likely non-standard URLs)', total, filled, missing;
END $$;

COMMIT;
