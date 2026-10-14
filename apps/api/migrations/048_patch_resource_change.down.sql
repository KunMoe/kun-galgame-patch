-- Drops the feed's whole history: a reader's cursor then points into nothing,
-- and it must take a full scan of the face again.
DROP TRIGGER IF EXISTS trg_patch_resource_record_change ON patch_resource;
DROP FUNCTION IF EXISTS patch_resource_record_change();
DROP TABLE IF EXISTS patch_resource_change;
