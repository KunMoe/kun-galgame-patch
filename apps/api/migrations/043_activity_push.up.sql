-- 043: push this site's public activity into NextMoe community's following
-- feed (infra wave 13, contract docs/community/01 "Activities and the
-- following feed").
--
-- A transactional outbox: triggers enqueue an activity key in the same
-- transaction as the write that changed it, and internal/community/activitypush
-- drains the queue, reads each key's CURRENT state and pushes it (content or a
-- tombstone). Pushing from the request goroutine would lose every write whose
-- process died before the call returned.
--
-- Keys (the pusher parses them back):
--   patch_resource:<resource id>                             verb publish
--   patch_resource_edit:<resource id>:<actor>:<YYYYMMDD>     verb edit, one per
--     resource, editor and Beijing day -- the day community groups by -- so a
--     resource edited five times in a day is one edit, not five.
-- Likes are not pushed: no page shows who liked a resource.
--
-- activity_push_queue.notify is true only for a resource's INSERT and for a
-- resource coming back to status 0 (enabled again / restored from a moderation
-- hide). Community notifies only a key it has never seen whose occurred_at is
-- within 24 hours, so a restore of something already pushed notifies nobody.
-- A conflicting enqueue ORs notify and moves enqueued_at; the drainer deletes a
-- queue row only when enqueued_at is still the value it claimed.
--
-- activity_push_sent is what community last accepted per key. A tombstone
-- needs the actor after the row is gone, and a key community never accepted
-- gets no tombstone: community counts a tombstone as "seen", and the later
-- restore would then never notify.
--
-- Existing rows: every resource and every (resource, editor, day) of
-- patch_resource_revision is enqueued once below with notify=false (the
-- backfill), about 10k keys.

CREATE TABLE IF NOT EXISTS activity_push_queue (
    key         varchar(128) PRIMARY KEY,
    notify      boolean NOT NULL DEFAULT false,
    enqueued_at timestamptz NOT NULL DEFAULT clock_timestamp()
);

CREATE INDEX IF NOT EXISTS idx_activity_push_queue_enqueued
    ON activity_push_queue (enqueued_at);

CREATE TABLE IF NOT EXISTS activity_push_sent (
    key      varchar(128) PRIMARY KEY,
    actor_id int NOT NULL,
    revision bigint NOT NULL,
    removed  boolean NOT NULL,
    sent_at  timestamptz NOT NULL DEFAULT now()
);

CREATE OR REPLACE FUNCTION activity_edit_key(resource_id int, actor_id int, at timestamptz)
RETURNS text LANGUAGE sql STABLE AS $$
    SELECT 'patch_resource_edit:' || resource_id || ':' || actor_id || ':'
        || to_char(at AT TIME ZONE 'Asia/Shanghai', 'YYYYMMDD')
$$;

CREATE OR REPLACE FUNCTION activity_push_enqueue(k text, n boolean)
RETURNS void LANGUAGE sql AS $$
    INSERT INTO activity_push_queue (key, notify) VALUES (k, n)
    ON CONFLICT (key) DO UPDATE
    SET notify = activity_push_queue.notify OR EXCLUDED.notify,
        enqueued_at = clock_timestamp()
$$;

-- A resource's title, visibility and work carry into its edit items, so a
-- change to those re-enqueues the edits too.
CREATE OR REPLACE FUNCTION activity_push_resource()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        PERFORM activity_push_enqueue('patch_resource:' || OLD.id, false);
        RETURN NULL;
    END IF;
    IF TG_OP = 'INSERT' THEN
        PERFORM activity_push_enqueue('patch_resource:' || NEW.id, true);
        RETURN NULL;
    END IF;
    PERFORM activity_push_enqueue('patch_resource:' || NEW.id, OLD.status <> 0 AND NEW.status = 0);
    INSERT INTO activity_push_queue (key)
    SELECT DISTINCT activity_edit_key(resource_id, actor_id, created_at)
    FROM patch_resource_revision
    WHERE resource_id = NEW.id AND actor_id > 0
    ON CONFLICT (key) DO UPDATE SET enqueued_at = clock_timestamp();
    RETURN NULL;
END
$$;

DROP TRIGGER IF EXISTS trg_activity_push_resource ON patch_resource;
CREATE TRIGGER trg_activity_push_resource
    AFTER INSERT OR DELETE ON patch_resource
    FOR EACH ROW EXECUTE FUNCTION activity_push_resource();

-- download and like_count move on every download and like; only the columns
-- an activity is built from re-enqueue.
DROP TRIGGER IF EXISTS trg_activity_push_resource_update ON patch_resource;
CREATE TRIGGER trg_activity_push_resource_update
    AFTER UPDATE ON patch_resource
    FOR EACH ROW
    WHEN (OLD.status IS DISTINCT FROM NEW.status
       OR OLD.name IS DISTINCT FROM NEW.name
       OR OLD.galgame_id IS DISTINCT FROM NEW.galgame_id
       OR OLD.user_id IS DISTINCT FROM NEW.user_id
       OR OLD.type IS DISTINCT FROM NEW.type
       OR OLD.language IS DISTINCT FROM NEW.language
       OR OLD.platform IS DISTINCT FROM NEW.platform
       OR OLD.size IS DISTINCT FROM NEW.size)
    EXECUTE FUNCTION activity_push_resource();

CREATE OR REPLACE FUNCTION activity_push_revision()
RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    r patch_resource_revision%ROWTYPE;
BEGIN
    IF TG_OP = 'DELETE' THEN r := OLD; ELSE r := NEW; END IF;
    IF r.actor_id > 0 THEN
        PERFORM activity_push_enqueue(activity_edit_key(r.resource_id, r.actor_id, r.created_at), false);
    END IF;
    RETURN NULL;
END
$$;

DROP TRIGGER IF EXISTS trg_activity_push_revision ON patch_resource_revision;
CREATE TRIGGER trg_activity_push_revision
    AFTER INSERT OR DELETE ON patch_resource_revision
    FOR EACH ROW EXECUTE FUNCTION activity_push_revision();

-- The claim-event cron unpublishes a page catalog hid, and the changes cron
-- rewrites its content_limit: both change every activity on the page. What
-- catalog changes without touching this row (a rename, a new banner) is left
-- to the pusher's daily reconcile.
CREATE OR REPLACE FUNCTION activity_push_patch()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    INSERT INTO activity_push_queue (key)
    SELECT 'patch_resource:' || id FROM patch_resource WHERE galgame_id = NEW.id
    UNION
    SELECT activity_edit_key(v.resource_id, v.actor_id, v.created_at)
    FROM patch_resource_revision v
    JOIN patch_resource r ON r.id = v.resource_id
    WHERE r.galgame_id = NEW.id AND v.actor_id > 0
    ON CONFLICT (key) DO UPDATE SET enqueued_at = clock_timestamp();
    RETURN NULL;
END
$$;

DROP TRIGGER IF EXISTS trg_activity_push_patch ON patch;
CREATE TRIGGER trg_activity_push_patch
    AFTER UPDATE ON patch
    FOR EACH ROW
    WHEN (OLD.published IS DISTINCT FROM NEW.published
       OR OLD.content_limit IS DISTINCT FROM NEW.content_limit)
    EXECUTE FUNCTION activity_push_patch();

INSERT INTO activity_push_queue (key)
SELECT 'patch_resource:' || id FROM patch_resource
UNION
SELECT activity_edit_key(resource_id, actor_id, created_at)
FROM patch_resource_revision
WHERE actor_id > 0
ON CONFLICT (key) DO NOTHING;
