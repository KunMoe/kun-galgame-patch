-- MOYU-PR5 (M3): patch_resource file replacement audit trail.
--
-- Purpose: patch_resource is mutated in place when a user re-uploads or
-- swaps the file/link for a resource. Without an audit row, "this download
-- broke" complaints can't be traced back to *what changed* / *who changed
-- it* / *why*. This table records a snapshot of the OLD file fields before
-- each substantive change. Pure metadata edits (note/name/code/...) do NOT
-- write a row — only Storage / S3Key / Content changes count.
--
-- CASCADE on delete: when the resource is deleted, its history goes with it.
-- Deliberate per the moyu upgrade plan §4.3.3 — "delete = forget" semantics
-- avoid leaking past file paths of removed resources.

CREATE TABLE IF NOT EXISTS patch_resource_file_history (
    id              BIGSERIAL PRIMARY KEY,
    resource_id     INT NOT NULL REFERENCES patch_resource(id) ON DELETE CASCADE,
    old_storage     VARCHAR(16) NOT NULL,                   -- 's3' / 'mega' / 'onedrive' / ...
    old_s3_key      VARCHAR(2048) NOT NULL DEFAULT '',      -- valid when old_storage='s3'
    old_blake3      VARCHAR(128) NOT NULL DEFAULT '',
    old_size        VARCHAR(107) NOT NULL DEFAULT '',       -- mirrors patch_resource.size (display string, not bytes)
    old_content     TEXT NOT NULL DEFAULT '',               -- original external URL / link text
    reason          VARCHAR(500) NOT NULL DEFAULT '',       -- operator-supplied "why replaced"; optional
    actor_id        INT NOT NULL,
    actor_role      INT NOT NULL DEFAULT 0,                 -- snapshot role: 3=admin, 2=moderator, 1=user, 0=unknown
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_prfh_resource ON patch_resource_file_history(resource_id, created_at DESC);
