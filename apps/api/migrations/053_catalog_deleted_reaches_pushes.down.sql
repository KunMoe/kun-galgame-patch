-- Back to 051's reach: the stamp hides a work from this site's reads only, and
-- the community feeds and letmoe catch up at the next reconcile or never.
BEGIN;

SET LOCAL lock_timeout = '5s';

DROP TRIGGER IF EXISTS trg_patch_catalog_deleted_change ON patch;
DROP FUNCTION IF EXISTS patch_catalog_deleted_record_change();

DROP TRIGGER IF EXISTS trg_activity_push_patch ON patch;
CREATE TRIGGER trg_activity_push_patch
    AFTER UPDATE ON patch
    FOR EACH ROW
    WHEN (OLD.published IS DISTINCT FROM NEW.published
       OR OLD.content_limit IS DISTINCT FROM NEW.content_limit)
    EXECUTE FUNCTION activity_push_patch();

DROP TRIGGER IF EXISTS trg_anchor_presentation_patch_update ON patch;
CREATE TRIGGER trg_anchor_presentation_patch_update
    AFTER UPDATE ON patch
    FOR EACH ROW
    WHEN (OLD.published IS DISTINCT FROM NEW.published
       OR OLD.content_limit IS DISTINCT FROM NEW.content_limit)
    EXECUTE FUNCTION anchor_presentation_patch();

COMMIT;
