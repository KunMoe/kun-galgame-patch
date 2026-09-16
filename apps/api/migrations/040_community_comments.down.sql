-- Dropping the map strands every pre-cutover #comment-<id> link; it is
-- recoverable only by re-running the import tool, whose ledger this is. Run
-- this only when rolling the whole cutover back to patch_comment.
DROP INDEX IF EXISTS uq_patch_comment_map_post;
DROP TABLE IF EXISTS patch_comment_community_map;
