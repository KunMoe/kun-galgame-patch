-- 015_blog: independent, DB-backed blog feature (admin-managed).
--
-- Distinct from /about (about_post, migration 014): about posts are seeded from
-- .mdx files and read-only at runtime; blog posts are created/edited/deleted via
-- the admin backend (/admin/blog) and live entirely in this table.
--
-- Images go through the central image_service: `banner_image_hash` is the
-- image_service content hash (the API derives the CDN URL from it), and inline
-- images in `content` are image_service CDN URLs the editor inserts after
-- uploading via POST /upload/image-service. No image bytes live here.
CREATE TABLE IF NOT EXISTS blog (
    id                BIGSERIAL    PRIMARY KEY,
    title             VARCHAR(255) NOT NULL DEFAULT '',
    summary           TEXT         NOT NULL DEFAULT '',   -- short description for cards / SEO
    content           TEXT         NOT NULL DEFAULT '',   -- markdown body (image_service URLs embedded)
    banner_image_hash CHAR(64)     NOT NULL DEFAULT '',   -- image_service hash; '' = no banner
    status            SMALLINT     NOT NULL DEFAULT 0,    -- 0 = draft, 1 = published
    pin               BOOLEAN      NOT NULL DEFAULT false,
    view              INTEGER      NOT NULL DEFAULT 0,
    user_id           INTEGER      NOT NULL DEFAULT 0,    -- author (admin/moderator)
    created           TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated           TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Public list = status=1 ordered by pin then recency; admin list scans all.
CREATE INDEX IF NOT EXISTS idx_blog_status_pin_created ON blog(status, pin DESC, created DESC);
