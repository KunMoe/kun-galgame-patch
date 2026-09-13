-- 038: patch.id IS the catalog work id, so the column that used to hold it is
-- a second copy of the primary key. Runs with the deploy that makes the
-- identity real -- cmd/align-patch-ids has to have renumbered the table first.
--
-- The column was never a cache of something cheap to recompute: it was derived
-- locally by a rule that was wrong for `wiki-<n>` rows (it read n as a catalog
-- work id when n is this site's own gid, which catalog stores as a `curated`
-- ref), and 63 of 10,957 rows pointed at a different game's work. Nothing
-- refreshed it either -- SetCatalogWorkID had zero callers. Deleting it removes
-- the whole class.
--
-- The id sequence goes out of reach on purpose. Page ids are minted by catalog
-- now and every insert names one; a row that fell back to the sequence would
-- take a small integer that catalog will later hand to a different work, which
-- is the exact collision this whole change exists to end.

BEGIN;

DROP INDEX IF EXISTS idx_patch_catalog_work;

ALTER TABLE patch DROP COLUMN IF EXISTS catalog_work_id;

SELECT setval(pg_get_serial_sequence('patch', 'id'), 2000000000, true);

COMMIT;
