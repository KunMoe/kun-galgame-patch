-- Comment moderation status for the admin "评论需要审核" (comment-verify) toggle.
--
--   0 = approved / visible (default — all EXISTING comments stay visible, so
--       turning the toggle on/off never retroactively hides past comments).
--   1 = pending review — set when a comment is created while the toggle is ON.
--       Hidden from every public read (per-patch list, global /comment list,
--       home recent-comments) until an admin approves it via
--       PUT /admin/comment/:id/approve, which flips it to 0 and ONLY THEN
--       increments patch.comment_count + awards the author→owner moemoepoint
--       (so pending / rejected comments never inflate counts or farm points).
ALTER TABLE patch_comment ADD COLUMN IF NOT EXISTS status integer NOT NULL DEFAULT 0;

-- Partial index for the admin review queue (status <> 0). Tiny: only pending
-- rows are indexed; the hot status=0 read path is unaffected.
CREATE INDEX IF NOT EXISTS idx_patch_comment_pending ON patch_comment (status) WHERE status <> 0;
