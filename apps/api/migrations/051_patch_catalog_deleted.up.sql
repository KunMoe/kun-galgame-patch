-- 051: remember which pages name a work catalog has deleted.
--
-- Catalog can delete a work since spec 2.62.0 (infra #453). A reviewed deletion
-- request soft-deletes it: the detail face answers 410 ENTITY_DELETED with
-- deleted_reason and deleted_at, /v2/catalog/changes reports the id gone, and a
-- reviewer may restore it later under the same id. Until now a gone id was
-- skipped, so a deleted work's page kept paging in every list and only fell
-- out at hydrate time, and the page itself answered a bare 404.
--
-- The page stays on its number with its resources. The display mirror cron
-- stamps these columns when the feed reports the id gone and the detail face
-- confirms the deletion, and clears them when the id comes back live after a
-- restore. Lists leave a stamped page out; the page says the work was deleted.
--
-- No backfill. Deletions carry a 48-hour cooling-off from approval and the face
-- shipped 2026-10-21, and the cron re-reads the feed's partial tail every tick,
-- so the first ticks after this deploys see every deletion there is.

BEGIN;

ALTER TABLE patch ADD COLUMN IF NOT EXISTS catalog_deleted_at TIMESTAMPTZ;
ALTER TABLE patch ADD COLUMN IF NOT EXISTS catalog_deleted_reason TEXT;

COMMENT ON COLUMN patch.catalog_deleted_at IS
  'When catalog deleted this work (051). NULL = live. Cleared when catalog restores it.';
COMMENT ON COLUMN patch.catalog_deleted_reason IS
  'Catalog deleted_reason (not_a_work, out_of_scope, spam_or_test, other) while catalog_deleted_at is set (051).';

COMMIT;
