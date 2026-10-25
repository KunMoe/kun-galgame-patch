-- 056: a cache of one hosted zip's file list, read from that zip's own central
-- directory over HTTP Range (ported from letmoe 054, 2026-10-13), so a
-- resource page can show what is inside its file and preview a text, image,
-- audio or RPG Maker save entry without downloading the archive.
--
-- Keyed by artifact uuid, the same varchar(36) patch_resource stores. The bytes
-- behind a uuid never change, and a replaced or normalized file is a new uuid,
-- so a uuid is read once. There is no foreign key to patch_resource: replacing
-- a file leaves the old row, and the sweeper's orphan DELETE reclaims it.
--
-- No existing rows change. The table starts empty; the minute sweeper fills it
-- from the hosted resources that already exist.

CREATE TABLE IF NOT EXISTS artifact_listing (
    artifact_uuid varchar(36) PRIMARY KEY,
    state         text        NOT NULL CHECK (state IN ('ready', 'unsupported', 'failed')),
    entries       jsonb       NOT NULL DEFAULT '[]'::jsonb,
    file_count    integer     NOT NULL DEFAULT 0,
    dir_count     integer     NOT NULL DEFAULT 0,
    total_bytes   bigint      NOT NULL DEFAULT 0,
    error         text        NOT NULL DEFAULT '',
    created       timestamptz NOT NULL DEFAULT now(),
    updated       timestamptz NOT NULL DEFAULT now()
);
