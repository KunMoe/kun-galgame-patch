-- Reverse 016: doc → about_post (drop added columns, restore directory name)
-- and recreate the empty blog table (migration 015 shape).
ALTER TABLE doc RENAME TO about_post;
DROP INDEX IF EXISTS idx_doc_category;
DROP INDEX IF EXISTS idx_doc_status_date;
ALTER TABLE about_post DROP COLUMN IF EXISTS status;
ALTER TABLE about_post DROP COLUMN IF EXISTS banner_image_hash;
ALTER TABLE about_post DROP COLUMN IF EXISTS view;
ALTER TABLE about_post DROP COLUMN IF EXISTS user_id;
ALTER TABLE about_post RENAME COLUMN category TO directory;
CREATE INDEX IF NOT EXISTS idx_about_post_directory ON about_post(directory);
CREATE INDEX IF NOT EXISTS idx_about_post_date ON about_post(date DESC);

CREATE TABLE IF NOT EXISTS blog (
    id                BIGSERIAL    PRIMARY KEY,
    title             VARCHAR(255) NOT NULL DEFAULT '',
    summary           TEXT         NOT NULL DEFAULT '',
    content           TEXT         NOT NULL DEFAULT '',
    banner_image_hash CHAR(64)     NOT NULL DEFAULT '',
    status            SMALLINT     NOT NULL DEFAULT 0,
    pin               BOOLEAN      NOT NULL DEFAULT false,
    view              INTEGER      NOT NULL DEFAULT 0,
    user_id           INTEGER      NOT NULL DEFAULT 0,
    created           TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated           TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_blog_status_pin_created ON blog(status, pin DESC, created DESC);
