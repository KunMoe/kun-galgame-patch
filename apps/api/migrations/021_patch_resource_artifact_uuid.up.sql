-- 021_patch_resource_artifact_uuid
--
-- Add artifact_uuid to patch_resource: the identifier of the file's blob in the
-- centralized artifact service (kun-galgame-infra). New s3-storage resources
-- reference their blob by this uuid and download via the artifact service; legacy
-- rows keep s3_key/content for the old CDN path (dual-read, forward-only — see
-- kun-galgame-infra/docs/artifact/08). Empty string = not (yet) artifact-backed.
ALTER TABLE patch_resource ADD COLUMN IF NOT EXISTS artifact_uuid varchar(36) NOT NULL DEFAULT '';
