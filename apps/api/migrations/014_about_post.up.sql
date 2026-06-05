-- 014_about_post: move the /about articles from on-disk .mdx files into the DB.
--
-- Until now /about posts were static .mdx files baked into the api image and
-- re-read from disk per request (internal/about/service). This table makes the
-- DB the source of truth: the about service now reads from here, and a one-time
-- seeder (cmd/migrate-about-posts) imports the existing .mdx files (idempotent
-- upsert by slug — re-run it to publish edits).
--
-- `content` holds the raw markdown body; HTML + TOC are rendered on read so the
-- output stays in lockstep with the markdown package. `date` mirrors the
-- frontmatter date string verbatim (ISO → lexical sort == chronological) to
-- preserve the existing ordering and wire shape. `slug` is "<directory>/<name>"
-- (forward slashes); `directory` is its top-level segment, kept for grouping.
CREATE TABLE IF NOT EXISTS about_post (
    id              BIGSERIAL    PRIMARY KEY,
    slug            VARCHAR(255) NOT NULL UNIQUE,
    directory       VARCHAR(64)  NOT NULL DEFAULT '',
    title           VARCHAR(255) NOT NULL DEFAULT '',
    banner          VARCHAR(512) NOT NULL DEFAULT '',
    description     TEXT         NOT NULL DEFAULT '',
    date            VARCHAR(32)  NOT NULL DEFAULT '',
    author_uid      INT          NOT NULL DEFAULT 0,
    author_name     VARCHAR(255) NOT NULL DEFAULT '',
    author_avatar   VARCHAR(512) NOT NULL DEFAULT '',
    author_homepage VARCHAR(512) NOT NULL DEFAULT '',
    pin             BOOLEAN      NOT NULL DEFAULT false,
    content         TEXT         NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_about_post_directory ON about_post(directory);
CREATE INDEX IF NOT EXISTS idx_about_post_date ON about_post(date DESC);
