-- 047: repacks, the derived copy of a hosted resource's file that letmoe
-- splices into its merged base-game + patch downloads (letmoe spec 2026-10-14,
-- infra composite downloads, docs/artifact/11).
--
-- letmoe's resource forge downloads the original through
-- /v2/moyu/resources/{id}/file, rebuilds the same tree as a deterministic STORE
-- zip without the archive password, and uploads it to moyu through the bot
-- lane. It lives here, under the original resource, because a patch belongs to
-- its author (owner's ruling): the repack has no page, counters or attribution
-- of its own. One per resource.
--
-- letmoe composites pin the repack's artifact, and infra deletes every
-- composite pinning an artifact when moyu deletes it. So the artifact must be
-- deleted whenever the repack stops being valid: the resource is deleted, its
-- file is replaced, or the repack is withdrawn or superseded. Triggers catch
-- those, not the Go write paths, because a resource also goes by FK cascade
-- when its page or its user is deleted, which no Go code sees. The artifact
-- delete is a network call, so the triggers only queue it in
-- patch_resource_repack_purge, and the API drains that queue.
--
-- A disabled or moderation-hidden resource keeps its repack: both are
-- reversible, /file answers 404 meanwhile, and letmoe takes its own composite
-- down until the resource comes back.
--
-- New tables only; no existing rows are touched.

CREATE TABLE IF NOT EXISTS patch_resource_repack (
    resource_id          integer PRIMARY KEY REFERENCES patch_resource(id) ON DELETE CASCADE,
    artifact_uuid        varchar(36) NOT NULL UNIQUE,
    source_artifact_uuid varchar(36) NOT NULL,
    blake3               varchar(64) NOT NULL,
    bytes                bigint NOT NULL,
    file_count           integer NOT NULL,
    source_blake3        varchar(64) NOT NULL,
    created_at           timestamptz NOT NULL DEFAULT now()
);

-- resource_id carries no FK: the resource is usually gone by the time its
-- repack's artifact is purged.
CREATE TABLE IF NOT EXISTS patch_resource_repack_purge (
    artifact_uuid   varchar(36) PRIMARY KEY,
    resource_id     integer NOT NULL,
    attempts        integer NOT NULL DEFAULT 0,
    last_error      text NOT NULL DEFAULT '',
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_patch_resource_repack_purge_due
    ON patch_resource_repack_purge (next_attempt_at);

CREATE OR REPLACE FUNCTION patch_resource_repack_queue_purge()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' OR OLD.artifact_uuid IS DISTINCT FROM NEW.artifact_uuid THEN
        INSERT INTO patch_resource_repack_purge (artifact_uuid, resource_id)
        VALUES (OLD.artifact_uuid, OLD.resource_id)
        ON CONFLICT (artifact_uuid) DO NOTHING;
    END IF;
    RETURN NULL;
END;
$$;

DROP TRIGGER IF EXISTS trg_patch_resource_repack_queue_purge ON patch_resource_repack;
CREATE TRIGGER trg_patch_resource_repack_queue_purge
    AFTER DELETE OR UPDATE OF artifact_uuid ON patch_resource_repack
    FOR EACH ROW EXECUTE FUNCTION patch_resource_repack_queue_purge();

-- The WHEN clause is load-bearing: GORM's Save writes every column, so an edit
-- that only renames a resource still lists artifact_uuid in its SET.
CREATE OR REPLACE FUNCTION patch_resource_drop_stale_repack()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    DELETE FROM patch_resource_repack WHERE resource_id = NEW.id;
    RETURN NULL;
END;
$$;

DROP TRIGGER IF EXISTS trg_patch_resource_drop_stale_repack ON patch_resource;
CREATE TRIGGER trg_patch_resource_drop_stale_repack
    AFTER UPDATE OF artifact_uuid ON patch_resource
    FOR EACH ROW WHEN (OLD.artifact_uuid IS DISTINCT FROM NEW.artifact_uuid)
    EXECUTE FUNCTION patch_resource_drop_stale_repack();
