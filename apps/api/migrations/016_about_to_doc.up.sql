-- 016_about_to_doc: unify /about + /blog into one `doc` feature.
--
-- about_post (migration 014) already has the doc structure: category (its
-- `directory`), slug, markdown content, tree-able layout. Promote it to `doc`
-- and add the admin/blog capabilities: publish status, image_service banner
-- hash, view counter, author user_id. The standalone `blog` table (migration
-- 015) is redundant once docs are admin-managed — it was empty, so drop it.
--
-- One-shot (renames aren't idempotent) — tracked by _migrations, runs once.

ALTER TABLE about_post RENAME COLUMN directory TO category;
ALTER TABLE about_post ADD COLUMN IF NOT EXISTS status SMALLINT NOT NULL DEFAULT 1; -- existing about posts = published
ALTER TABLE about_post ADD COLUMN IF NOT EXISTS banner_image_hash CHAR(64) NOT NULL DEFAULT '';
ALTER TABLE about_post ADD COLUMN IF NOT EXISTS view INTEGER NOT NULL DEFAULT 0;
ALTER TABLE about_post ADD COLUMN IF NOT EXISTS user_id INTEGER NOT NULL DEFAULT 0;

-- Seed the author from the legacy frontmatter author_uid.
UPDATE about_post SET user_id = author_uid WHERE user_id = 0 AND author_uid > 0;

ALTER TABLE about_post RENAME TO doc;

DROP INDEX IF EXISTS idx_about_post_directory;
DROP INDEX IF EXISTS idx_about_post_date;
CREATE INDEX IF NOT EXISTS idx_doc_category ON doc(category);
CREATE INDEX IF NOT EXISTS idx_doc_status_date ON doc(status, date DESC);

DROP TABLE IF EXISTS blog;
