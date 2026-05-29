DROP INDEX IF EXISTS idx_patch_comment_pending;
ALTER TABLE patch_comment DROP COLUMN IF EXISTS status;
