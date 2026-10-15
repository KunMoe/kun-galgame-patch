-- 050: every hosted resource's file becomes letmoe forge's passwordless zip
-- (owner's ruling, 2026-10-15): the author's archive is extracted with the
-- resource's password and repacked whole, readme and notes included, and the
-- original is deleted. Author, times, history, counters and note stay as they
-- were; only the file is swapped and the password cleared. Uploads and
-- replacements made after this go through the same lane, minutes after they
-- are published.
--
-- patch_resource_normalize holds one job per source file, keyed by that file's
-- artifact_uuid. The queue is not stored: a public hosted resource whose
-- current file has no job and is not a forge product is pending, so a new
-- upload or a replaced file joins it with no write path involved. A job ends
-- `done` (artifact_uuid names the product, and source_password / source_size
-- keep what the swap overwrote; the original is queued for deletion 30 days
-- out, so a revert needs nothing from infra until then) or `failed` (terminal for that file; the author replacing
-- it starts a new job). claimed_at paces the backfill: every claim the forge
-- takes becomes an upload Init on moyu's artifact quota (3,000 a UTC day,
-- shared with every reader's upload).
--
-- Two existing triggers change:
--   047's repack purge no longer queues an artifact that is still some
--   resource's file. Promoting a repack makes it the file, and 047 drops the
--   repack row on the artifact_uuid change; without this its purge would
--   delete the file the swap just installed.
--   048's change feed logs the swap as `normalized`, not `file_replaced`:
--   letmoe rebuilds a merged download on file_replaced, and ~300 of them pin
--   the very repack being promoted.
--
-- New table and trigger bodies only; no existing rows are touched.

CREATE TABLE IF NOT EXISTS patch_resource_normalize (
    source_artifact_uuid varchar(36) PRIMARY KEY,
    resource_id          integer NOT NULL REFERENCES patch_resource(id) ON DELETE CASCADE,
    state                varchar(16) NOT NULL CHECK (state IN ('claimed', 'done', 'failed')),
    attempts             integer NOT NULL DEFAULT 0,
    lease_until          timestamptz,
    claimed_at           timestamptz,
    artifact_uuid        varchar(36) NOT NULL DEFAULT '',
    blake3               varchar(64) NOT NULL DEFAULT '',
    bytes                bigint NOT NULL DEFAULT 0,
    file_count           integer NOT NULL DEFAULT 0,
    has_archives         boolean NOT NULL DEFAULT false,
    source_password      varchar(1007) NOT NULL DEFAULT '',
    source_size          varchar(107) NOT NULL DEFAULT '',
    code                 varchar(64) NOT NULL DEFAULT '',
    message              text NOT NULL DEFAULT '',
    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_patch_resource_normalize_resource
    ON patch_resource_normalize (resource_id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_patch_resource_normalize_product
    ON patch_resource_normalize (artifact_uuid) WHERE artifact_uuid <> '';

CREATE OR REPLACE FUNCTION patch_resource_repack_queue_purge()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF (TG_OP = 'DELETE' OR OLD.artifact_uuid IS DISTINCT FROM NEW.artifact_uuid)
       AND NOT EXISTS (SELECT 1 FROM patch_resource WHERE artifact_uuid = OLD.artifact_uuid) THEN
        INSERT INTO patch_resource_repack_purge (artifact_uuid, resource_id)
        VALUES (OLD.artifact_uuid, OLD.resource_id)
        ON CONFLICT (artifact_uuid) DO NOTHING;
    END IF;
    RETURN NULL;
END;
$$;

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
        k := CASE WHEN EXISTS (
                SELECT 1 FROM patch_resource_normalize
                WHERE source_artifact_uuid = OLD.artifact_uuid AND artifact_uuid = NEW.artifact_uuid
             ) THEN 'normalized' ELSE 'file_replaced' END;
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
