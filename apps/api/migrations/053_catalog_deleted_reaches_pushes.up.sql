-- 053: a work catalog deleted (051) leaves the community feeds and the face's
-- change feed when it is stamped, and comes back when the stamp is cleared.
--
-- 051's stamp hid the page and its resources from this site's own reads, but
-- three things outside them only listen to triggers on patch. The activity push
-- (043) and the anchor presentation push (045) re-read a page only when
-- published or content_limit changes, so a deleted work's resources stayed in
-- the community following feed and its walls kept their presentation until the
-- next daily reconcile, and a restore waited for it too. The face's change feed
-- (048) only logs patch_resource writes, so letmoe was never told that the
-- resources it merges into its packs stopped being public.
--
-- The two patch-update triggers now also fire on catalog_deleted_at, and a new
-- trigger writes a hidden row for each public resource of a page when the stamp
-- is set and a restored row when it is cleared, the same events 048 writes when
-- a resource's own status moves. No rows are touched by this migration.

BEGIN;

DROP TRIGGER IF EXISTS trg_activity_push_patch ON patch;
CREATE TRIGGER trg_activity_push_patch
    AFTER UPDATE ON patch
    FOR EACH ROW
    WHEN (OLD.published IS DISTINCT FROM NEW.published
       OR OLD.content_limit IS DISTINCT FROM NEW.content_limit
       OR OLD.catalog_deleted_at IS DISTINCT FROM NEW.catalog_deleted_at)
    EXECUTE FUNCTION activity_push_patch();

DROP TRIGGER IF EXISTS trg_anchor_presentation_patch_update ON patch;
CREATE TRIGGER trg_anchor_presentation_patch_update
    AFTER UPDATE ON patch
    FOR EACH ROW
    WHEN (OLD.published IS DISTINCT FROM NEW.published
       OR OLD.content_limit IS DISTINCT FROM NEW.content_limit
       OR OLD.catalog_deleted_at IS DISTINCT FROM NEW.catalog_deleted_at)
    EXECUTE FUNCTION anchor_presentation_patch();

CREATE OR REPLACE FUNCTION patch_catalog_deleted_record_change()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    INSERT INTO patch_resource_change (resource_id, patch_id, storage, kind)
    SELECT r.id, r.galgame_id, r.storage,
           CASE WHEN NEW.catalog_deleted_at IS NULL THEN 'restored' ELSE 'hidden' END
    FROM patch_resource r
    WHERE r.galgame_id = NEW.id AND r.status = 0;
    RETURN NULL;
END
$$;

DROP TRIGGER IF EXISTS trg_patch_catalog_deleted_change ON patch;
CREATE TRIGGER trg_patch_catalog_deleted_change
    AFTER UPDATE ON patch
    FOR EACH ROW
    WHEN ((OLD.catalog_deleted_at IS NULL) IS DISTINCT FROM (NEW.catalog_deleted_at IS NULL))
    EXECUTE FUNCTION patch_catalog_deleted_record_change();

COMMIT;
