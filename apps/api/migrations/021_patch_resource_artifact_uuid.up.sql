-- 021_patch_resource_artifact_uuid
--
-- Add artifact_uuid to patch_resource: the identifier of the file's blob in the
-- centralized artifact service (kun-galgame-infra). New s3-storage resources
-- reference their blob by this uuid and download via the artifact service; legacy
-- rows keep s3_key/content for the old CDN path (dual-read, forward-only — see
-- kun-galgame-infra/docs/artifact/08). Empty string = not (yet) artifact-backed.
ALTER TABLE patch_resource ADD COLUMN IF NOT EXISTS artifact_uuid varchar(36) NOT NULL DEFAULT '';

-- Single-use: an artifact blob may back at most one resource (parallels the
-- legacy partial unique index on s3_key for storage='s3').
CREATE UNIQUE INDEX IF NOT EXISTS idx_patch_resource_artifact_uuid_unique
  ON patch_resource (artifact_uuid) WHERE artifact_uuid <> '';

-- Record the previous artifact uuid on file replacement (audit/recovery parity
-- with old_s3_key; the artifact service also retains the old blob via its
-- soft-delete TTL).
ALTER TABLE patch_resource_file_history ADD COLUMN IF NOT EXISTS old_artifact_uuid varchar(36) NOT NULL DEFAULT '';
