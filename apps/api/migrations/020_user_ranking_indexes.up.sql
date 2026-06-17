-- 020: Index user_id on the three tables the user ranking / profile aggregates
-- count over.
--
-- GET /ranking/user and GET /user/:id compute per-user totals with
--   COUNT(*) FROM patch          WHERE user_id = ?
--   COUNT(*) FROM patch_resource WHERE user_id = ?
--   COUNT(*) FROM patch_comment  WHERE user_id = ?
-- Postgres does NOT auto-create an index on an FK referencing column, so until
-- now each of these counts was a full table scan. Sorting the ranking by
-- resource_count / comment_count evaluates the count for EVERY user (31k+) to
-- build the sort key, i.e. tens of thousands of full scans per request — the
-- reported slowness. These btree indexes turn each per-user count into an index
-- range scan.
--
-- Idempotent (IF NOT EXISTS) per repo convention.
CREATE INDEX IF NOT EXISTS idx_patch_user_id ON public.patch (user_id);
CREATE INDEX IF NOT EXISTS idx_patch_resource_user_id ON public.patch_resource (user_id);
CREATE INDEX IF NOT EXISTS idx_patch_comment_user_id ON public.patch_comment (user_id);
