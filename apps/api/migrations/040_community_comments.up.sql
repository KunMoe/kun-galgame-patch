-- The comment walls move onto the NextMoe community primitive (kun_community).
--
-- Additive and safe to deploy before the cutover: until the API is pointed at
-- community the table sits empty and nothing reads it.
--
-- patch_comment and user_patch_comment_like_relation are FROZEN, not dropped.
-- They are the import's source of truth and the rollback site; a later
-- deploy-then-drop migration retires them once the cutover has held.
--
-- No local like mirror: the community read faces carry reaction_count and
-- viewer_reacted, so the count and the viewer's "我赞过" flag are answered
-- upstream. The 653 pre-cutover likes are carried over by the infra-side
-- importer as community reactions, not copied into this database.

-- patch_comment_community_map: old patch_comment id -> its migrated community
-- (thread, post). Every link minted before the cutover points at
-- #comment-<old id>, and LocateComment resolves through here.
--
-- ALSO created by the infra import tool with the IDENTICAL
-- CREATE TABLE IF NOT EXISTS DDL, so adopting it here is idempotent whichever
-- side runs first. NOT back-filled here: the import populates it, and it runs
-- BEFORE this site is deployed.
CREATE TABLE IF NOT EXISTS patch_comment_community_map (
  old_comment_id int    PRIMARY KEY,
  thread_id      bigint NOT NULL,
  post_id        bigint NOT NULL,
  galgame_id     int    NOT NULL,
  resource_id    int
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_patch_comment_map_post
  ON patch_comment_community_map (post_id);
