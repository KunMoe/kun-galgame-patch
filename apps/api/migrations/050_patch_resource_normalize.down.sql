-- Dropping the jobs forgets which files are forge products and what each swap
-- overwrote (the original's uuid, password and size), so a revert is no longer
-- possible. The swapped files stay swapped.
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

DROP TABLE IF EXISTS patch_resource_normalize;
