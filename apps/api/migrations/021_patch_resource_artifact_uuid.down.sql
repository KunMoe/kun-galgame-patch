ALTER TABLE patch_resource_file_history DROP COLUMN IF EXISTS old_artifact_uuid;
DROP INDEX IF EXISTS idx_patch_resource_artifact_uuid_unique;
ALTER TABLE patch_resource DROP COLUMN IF EXISTS artifact_uuid;
