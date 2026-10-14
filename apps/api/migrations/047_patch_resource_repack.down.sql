-- Dropping the purge queue forgets any repack artifact still waiting to be
-- deleted, and the letmoe composites pinning it keep serving. Drain it first:
-- SELECT count(*) FROM patch_resource_repack_purge must be 0.
DROP TRIGGER IF EXISTS trg_patch_resource_drop_stale_repack ON patch_resource;
DROP FUNCTION IF EXISTS patch_resource_drop_stale_repack();
DROP TRIGGER IF EXISTS trg_patch_resource_repack_queue_purge ON patch_resource_repack;
DROP FUNCTION IF EXISTS patch_resource_repack_queue_purge();
DROP TABLE IF EXISTS patch_resource_repack_purge;
DROP TABLE IF EXISTS patch_resource_repack;
