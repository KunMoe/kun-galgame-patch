-- Migration: OAuth Integration + Schema Alignment
-- Date: 2026-04-09
-- Description:
--   1. Convert String[] (text[]) columns to jsonb (16 columns)
--   2. Add denormalized count fields + backfill
--   3. Add OAuth account table
--   4. Add user follower/following count fields
--
-- IMPORTANT: Run this SQL manually BEFORE running `prisma db push`.
-- After this script completes, Prisma schema and database will be in sync.
--
-- Usage:
--   psql -h <host> -U <user> -d <dbname> -f migration.sql
--
-- Always backup the database before running!

BEGIN;

-- ============================================================
-- Part 0: Fill NULL values with empty arrays before type conversion
--
-- Some columns have NULL rows. to_jsonb(NULL) stays NULL, but
-- the Prisma schema marks these fields as required (no `?`).
-- Fill NULLs with '{}' (empty text[]) so they become '[]' jsonb.
-- ============================================================

-- patch table
UPDATE patch SET type     = '{}' WHERE type     IS NULL;
UPDATE patch SET language = '{}' WHERE language IS NULL;
UPDATE patch SET engine   = '{}' WHERE engine   IS NULL;
UPDATE patch SET platform = '{}' WHERE platform IS NULL;

-- patch_resource table
UPDATE patch_resource SET type     = '{}' WHERE type     IS NULL;
UPDATE patch_resource SET language = '{}' WHERE language IS NULL;
UPDATE patch_resource SET platform = '{}' WHERE platform IS NULL;

-- patch_release table
UPDATE patch_release SET platforms = '{}' WHERE platforms IS NULL;
UPDATE patch_release SET languages = '{}' WHERE languages IS NULL;

-- patch_char table
UPDATE patch_char SET roles = '{}' WHERE roles IS NULL;

-- patch_company table
UPDATE patch_company SET primary_language = '{}' WHERE primary_language IS NULL;
UPDATE patch_company SET official_website = '{}' WHERE official_website IS NULL;
UPDATE patch_company SET parent_brand     = '{}' WHERE parent_brand     IS NULL;
UPDATE patch_company SET alias            = '{}' WHERE alias            IS NULL;

-- patch_person table
UPDATE patch_person SET roles = '{}' WHERE roles IS NULL;
UPDATE patch_person SET links = '{}' WHERE links IS NULL;

-- patch_tag table
UPDATE patch_tag SET alias = '{}' WHERE alias IS NULL;

-- ============================================================
-- Part 1: Convert text[] columns to jsonb (zero data loss)
--
-- PostgreSQL cannot auto-cast an existing text[] DEFAULT to jsonb,
-- so we must: DROP DEFAULT → ALTER TYPE → SET new DEFAULT.
-- ============================================================

-- patch table (4 columns)
ALTER TABLE patch
  ALTER COLUMN type DROP DEFAULT,
  ALTER COLUMN type TYPE jsonb USING to_jsonb(type),
  ALTER COLUMN type SET DEFAULT '[]'::jsonb;

ALTER TABLE patch
  ALTER COLUMN language DROP DEFAULT,
  ALTER COLUMN language TYPE jsonb USING to_jsonb(language),
  ALTER COLUMN language SET DEFAULT '[]'::jsonb;

ALTER TABLE patch
  ALTER COLUMN engine DROP DEFAULT,
  ALTER COLUMN engine TYPE jsonb USING to_jsonb(engine),
  ALTER COLUMN engine SET DEFAULT '[]'::jsonb;

ALTER TABLE patch
  ALTER COLUMN platform DROP DEFAULT,
  ALTER COLUMN platform TYPE jsonb USING to_jsonb(platform),
  ALTER COLUMN platform SET DEFAULT '[]'::jsonb;

-- patch_resource table (3 columns)
ALTER TABLE patch_resource
  ALTER COLUMN type DROP DEFAULT,
  ALTER COLUMN type TYPE jsonb USING to_jsonb(type),
  ALTER COLUMN type SET DEFAULT '[]'::jsonb;

ALTER TABLE patch_resource
  ALTER COLUMN language DROP DEFAULT,
  ALTER COLUMN language TYPE jsonb USING to_jsonb(language),
  ALTER COLUMN language SET DEFAULT '[]'::jsonb;

ALTER TABLE patch_resource
  ALTER COLUMN platform DROP DEFAULT,
  ALTER COLUMN platform TYPE jsonb USING to_jsonb(platform),
  ALTER COLUMN platform SET DEFAULT '[]'::jsonb;

-- patch_release table (2 columns)
ALTER TABLE patch_release
  ALTER COLUMN platforms DROP DEFAULT,
  ALTER COLUMN platforms TYPE jsonb USING to_jsonb(platforms),
  ALTER COLUMN platforms SET DEFAULT '[]'::jsonb;

ALTER TABLE patch_release
  ALTER COLUMN languages DROP DEFAULT,
  ALTER COLUMN languages TYPE jsonb USING to_jsonb(languages),
  ALTER COLUMN languages SET DEFAULT '[]'::jsonb;

-- patch_char table (1 column)
ALTER TABLE patch_char
  ALTER COLUMN roles DROP DEFAULT,
  ALTER COLUMN roles TYPE jsonb USING to_jsonb(roles),
  ALTER COLUMN roles SET DEFAULT '[]'::jsonb;

-- patch_company table (4 columns)
ALTER TABLE patch_company
  ALTER COLUMN primary_language DROP DEFAULT,
  ALTER COLUMN primary_language TYPE jsonb USING to_jsonb(primary_language),
  ALTER COLUMN primary_language SET DEFAULT '[]'::jsonb;

ALTER TABLE patch_company
  ALTER COLUMN official_website DROP DEFAULT,
  ALTER COLUMN official_website TYPE jsonb USING to_jsonb(official_website),
  ALTER COLUMN official_website SET DEFAULT '[]'::jsonb;

ALTER TABLE patch_company
  ALTER COLUMN parent_brand DROP DEFAULT,
  ALTER COLUMN parent_brand TYPE jsonb USING to_jsonb(parent_brand),
  ALTER COLUMN parent_brand SET DEFAULT '[]'::jsonb;

ALTER TABLE patch_company
  ALTER COLUMN alias DROP DEFAULT,
  ALTER COLUMN alias TYPE jsonb USING to_jsonb(alias),
  ALTER COLUMN alias SET DEFAULT '[]'::jsonb;

-- patch_person table (2 columns)
ALTER TABLE patch_person
  ALTER COLUMN roles DROP DEFAULT,
  ALTER COLUMN roles TYPE jsonb USING to_jsonb(roles),
  ALTER COLUMN roles SET DEFAULT '[]'::jsonb;

ALTER TABLE patch_person
  ALTER COLUMN links DROP DEFAULT,
  ALTER COLUMN links TYPE jsonb USING to_jsonb(links),
  ALTER COLUMN links SET DEFAULT '[]'::jsonb;

-- patch_tag table (1 column)
ALTER TABLE patch_tag
  ALTER COLUMN alias DROP DEFAULT,
  ALTER COLUMN alias TYPE jsonb USING to_jsonb(alias),
  ALTER COLUMN alias SET DEFAULT '[]'::jsonb;

-- ============================================================
-- Part 2: Add denormalized count fields
-- ============================================================

-- user table: follower/following counts
ALTER TABLE "user"
  ADD COLUMN IF NOT EXISTS follower_count  integer NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS following_count integer NOT NULL DEFAULT 0;

-- patch table: favorite/contribute/comment/resource counts
ALTER TABLE patch
  ADD COLUMN IF NOT EXISTS favorite_count   integer NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS contribute_count integer NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS comment_count    integer NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS resource_count   integer NOT NULL DEFAULT 0;

-- patch_comment table: like count
ALTER TABLE patch_comment
  ADD COLUMN IF NOT EXISTS like_count integer NOT NULL DEFAULT 0;

-- patch_resource table: like count
ALTER TABLE patch_resource
  ADD COLUMN IF NOT EXISTS like_count integer NOT NULL DEFAULT 0;

-- ============================================================
-- Part 3: Backfill count fields from existing data
-- ============================================================

-- Patch counts
UPDATE patch SET favorite_count = sub.cnt
  FROM (SELECT patch_id, COUNT(*)::int AS cnt FROM user_patch_favorite_relation GROUP BY patch_id) sub
  WHERE patch.id = sub.patch_id;

UPDATE patch SET contribute_count = sub.cnt
  FROM (SELECT patch_id, COUNT(*)::int AS cnt FROM user_patch_contribute_relation GROUP BY patch_id) sub
  WHERE patch.id = sub.patch_id;

UPDATE patch SET comment_count = sub.cnt
  FROM (SELECT patch_id, COUNT(*)::int AS cnt FROM patch_comment GROUP BY patch_id) sub
  WHERE patch.id = sub.patch_id;

UPDATE patch SET resource_count = sub.cnt
  FROM (SELECT patch_id, COUNT(*)::int AS cnt FROM patch_resource GROUP BY patch_id) sub
  WHERE patch.id = sub.patch_id;

-- Comment like counts
UPDATE patch_comment SET like_count = sub.cnt
  FROM (SELECT comment_id, COUNT(*)::int AS cnt FROM user_patch_comment_like_relation GROUP BY comment_id) sub
  WHERE patch_comment.id = sub.comment_id;

-- Resource like counts
UPDATE patch_resource SET like_count = sub.cnt
  FROM (SELECT resource_id, COUNT(*)::int AS cnt FROM user_patch_resource_like_relation GROUP BY resource_id) sub
  WHERE patch_resource.id = sub.resource_id;

-- User follow counts
UPDATE "user" SET follower_count = sub.cnt
  FROM (SELECT following_id, COUNT(*)::int AS cnt FROM user_follow_relation GROUP BY following_id) sub
  WHERE "user".id = sub.following_id;

UPDATE "user" SET following_count = sub.cnt
  FROM (SELECT follower_id, COUNT(*)::int AS cnt FROM user_follow_relation GROUP BY follower_id) sub
  WHERE "user".id = sub.follower_id;

-- ============================================================
-- Part 4: Create OAuth account table
-- ============================================================

CREATE TABLE IF NOT EXISTS oauth_account (
  id       SERIAL       PRIMARY KEY,
  user_id  integer      NOT NULL,
  provider varchar(50)  NOT NULL DEFAULT 'kun-oauth',
  sub      varchar(255) NOT NULL,
  created  timestamptz  NOT NULL DEFAULT now(),
  updated  timestamptz  NOT NULL DEFAULT now(),

  CONSTRAINT oauth_account_sub_key UNIQUE (sub),
  CONSTRAINT oauth_account_user_id_fkey
    FOREIGN KEY (user_id) REFERENCES "user"(id) ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE INDEX IF NOT EXISTS oauth_account_user_id_idx ON oauth_account (user_id);

COMMIT;

-- ============================================================
-- Verification queries (run manually after migration)
-- ============================================================
-- Check jsonb column types:
--   SELECT column_name, data_type FROM information_schema.columns
--   WHERE table_name = 'patch' AND column_name IN ('type','language','engine','platform');
--
-- Check count fields exist:
--   SELECT column_name FROM information_schema.columns
--   WHERE table_name = 'patch' AND column_name LIKE '%_count';
--
-- Check oauth_account table:
--   SELECT * FROM information_schema.tables WHERE table_name = 'oauth_account';
--
-- Spot-check backfill accuracy:
--   SELECT p.id, p.comment_count, (SELECT COUNT(*) FROM patch_comment WHERE patch_id = p.id) AS actual
--   FROM patch p ORDER BY p.id LIMIT 10;
