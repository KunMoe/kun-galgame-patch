-- Dropping the columns loses every archive comment read so far; the originals
-- they came from may already be deleted.
ALTER TABLE patch_resource_normalize DROP COLUMN IF EXISTS source_comment;
ALTER TABLE patch_resource DROP COLUMN IF EXISTS archive_comment;
