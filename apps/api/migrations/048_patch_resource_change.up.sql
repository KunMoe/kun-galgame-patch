-- 048: an append-only log of what happened to each resource, served as the
-- face's GET /v2/moyu/changes. letmoe merges every eligible moyu patch into its
-- game packs automatically (owner's ruling, 2026-10-14) and follows this feed
-- rather than polling every work.
--
-- One row per visible event, written by a trigger so every write path counts,
-- FK cascades from a page or user delete included:
--   created        a resource was published
--   file_replaced  its file (storage, artifact, s3 key or link) changed
--   updated        anything else a reader sees changed: name, type, language,
--                  platform, size, note, codes, publisher, or the page it is on
--   hidden         it stopped being public (status 0 -> disabled or hidden)
--   restored       it became public again
--   deleted        the row is gone
-- Counters (download, like_count) and edits made while the resource is not
-- public are not events; restored tells the reader to read it again.
--
-- The feed orders by (txid, seq), not by seq alone. A bigserial is drawn when
-- the row is written, not when it commits, so a transaction holding seq N can
-- commit after N+1 is already served, and a seq cursor would step past N for
-- good. Serving only rows whose transaction is older than the oldest one still
-- running (pg_snapshot_xmin) makes every row visible before the cursor passes
-- it; a long writing transaction delays the feed and loses nothing.
--
-- No backfill: the log starts empty, and letmoe takes one full scan of the face
-- before following it.

CREATE TABLE IF NOT EXISTS patch_resource_change (
    seq         bigserial PRIMARY KEY,
    txid        xid8 NOT NULL DEFAULT pg_current_xact_id(),
    resource_id integer NOT NULL,
    patch_id    integer NOT NULL,
    storage     text NOT NULL,
    kind        varchar(16) NOT NULL,
    changed_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_patch_resource_change_feed
    ON patch_resource_change (txid, seq);

CREATE OR REPLACE FUNCTION patch_resource_record_change()
RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    k text;
BEGIN
    IF TG_OP = 'DELETE' THEN
        INSERT INTO patch_resource_change (resource_id, patch_id, storage, kind)
        VALUES (OLD.id, OLD.galgame_id, OLD.storage, 'deleted');
        RETURN NULL;
    END IF;

    IF TG_OP = 'INSERT' THEN
        IF NEW.status = 0 THEN
            k := 'created';
        END IF;
    ELSIF (OLD.status = 0) <> (NEW.status = 0) THEN
        k := CASE WHEN NEW.status = 0 THEN 'restored' ELSE 'hidden' END;
    ELSIF NEW.status <> 0 THEN
        RETURN NULL;
    ELSIF (OLD.storage, OLD.artifact_uuid, OLD.s3_key, OLD.content)
          IS DISTINCT FROM (NEW.storage, NEW.artifact_uuid, NEW.s3_key, NEW.content) THEN
        k := 'file_replaced';
    ELSIF (OLD.galgame_id, OLD.user_id, OLD.name, OLD.model_name, OLD.localization_group_name,
           OLD.size, OLD.code, OLD.password, OLD.note, OLD.type, OLD.language, OLD.platform)
          IS DISTINCT FROM
          (NEW.galgame_id, NEW.user_id, NEW.name, NEW.model_name, NEW.localization_group_name,
           NEW.size, NEW.code, NEW.password, NEW.note, NEW.type, NEW.language, NEW.platform) THEN
        k := 'updated';
    END IF;

    IF k IS NOT NULL THEN
        INSERT INTO patch_resource_change (resource_id, patch_id, storage, kind)
        VALUES (NEW.id, NEW.galgame_id, NEW.storage, k);
    END IF;
    RETURN NULL;
END;
$$;

-- The column list keeps the download and like counters, which are written on
-- every download and like, from firing it at all.
DROP TRIGGER IF EXISTS trg_patch_resource_record_change ON patch_resource;
CREATE TRIGGER trg_patch_resource_record_change
    AFTER INSERT OR DELETE OR UPDATE OF
        status, storage, artifact_uuid, s3_key, content, galgame_id, user_id, name, model_name,
        localization_group_name, size, code, password, note, type, language, platform
    ON patch_resource
    FOR EACH ROW EXECUTE FUNCTION patch_resource_record_change();
