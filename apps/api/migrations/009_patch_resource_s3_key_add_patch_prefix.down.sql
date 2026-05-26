-- This is a data fix-up migration, not a schema change. The "down" is a
-- no-op by design.
--
-- A literal reversal ("strip leading 'patch/' from every s3_key") is unsafe
-- because new rows written AFTER the up-migration are also shaped with the
-- "patch/" prefix (that's the canonical D10 format — see
-- apps/api/internal/common/upload/service.go buildPatchResourceKey). A
-- blanket strip would corrupt them too, leaving the DB in a worse state
-- than before the up ran.
--
-- If you genuinely need to roll back (e.g. a coordinated revert of a
-- breaking change in the upload pipeline), do it manually with a snapshot:
--   1. pg_dump the patch_resource + patch_resource_file_history tables
--      BEFORE running the up migration
--   2. restore from that snapshot on rollback
-- The application itself doesn't care about the prefix at write time — the
-- canonical shape comes from buildPatchResourceKey, not from anything this
-- migration touched.

DO $$ BEGIN
  RAISE NOTICE '009 down: no-op (see file comment for rationale)';
END $$;
