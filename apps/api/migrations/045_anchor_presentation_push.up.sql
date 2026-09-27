-- 045: tell NextMoe community which page each of this site's comment walls is
-- on (infra wave 13 D5, contract docs/community/01 "Community's own posts in
-- the feed").
--
-- Community projects the comments on moyu's walls into the following feed
-- itself, but it knows a wall only by its anchor: 1/<patch id> for a game page,
-- 2/<resource id> for a resource page. Without a presentation (title, URL,
-- work, content limit) a wall's comments produce no feed item.
--
-- The same transactional outbox as 043: triggers enqueue an anchor in the
-- transaction that may have changed its page, and internal/community/
-- activitypush drains the queue, reads each page's CURRENT state and pushes it
-- (or a tombstone). There is no backfill flag: a presentation never notifies.
--
-- anchor_presentation_sent is what community last accepted per anchor. An
-- anchor community never accepted gets no tombstone.
--
-- A game page renders for any work catalog renders, patch row or not, and a
-- comment on one does not create the row, so the triggers cannot see every
-- wall. The comment write path and the pusher's wall sweep (at start and in the
-- daily reconcile) enqueue the rest; production had 2 such walls of 1,982.
--
-- Existing rows: every patch and every resource is enqueued once below, about
-- 21k anchors.

CREATE TABLE IF NOT EXISTS anchor_presentation_queue (
    anchor_kind smallint NOT NULL,
    anchor_id   varchar(128) NOT NULL,
    enqueued_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (anchor_kind, anchor_id)
);

CREATE INDEX IF NOT EXISTS idx_anchor_presentation_queue_enqueued
    ON anchor_presentation_queue (enqueued_at);

CREATE TABLE IF NOT EXISTS anchor_presentation_sent (
    anchor_kind smallint NOT NULL,
    anchor_id   varchar(128) NOT NULL,
    revision    bigint NOT NULL,
    removed     boolean NOT NULL,
    sent_at     timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (anchor_kind, anchor_id)
);

CREATE OR REPLACE FUNCTION anchor_presentation_enqueue(kind smallint, id text)
RETURNS void LANGUAGE sql AS $$
    INSERT INTO anchor_presentation_queue (anchor_kind, anchor_id) VALUES (kind, id)
    ON CONFLICT (anchor_kind, anchor_id) DO UPDATE SET enqueued_at = clock_timestamp()
$$;

-- A game page's title, cover and rating come from catalog; the patch row only
-- signals that catalog's verdict moved (the claim-event cron unpublishes a page
-- catalog hid, the changes cron rewrites content_limit). Both also reach every
-- resource page of the work. A rename is left to the daily reconcile.
CREATE OR REPLACE FUNCTION anchor_presentation_patch()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        PERFORM anchor_presentation_enqueue(1::smallint, OLD.id::text);
        RETURN NULL;
    END IF;
    PERFORM anchor_presentation_enqueue(1::smallint, NEW.id::text);
    IF TG_OP = 'UPDATE' THEN
        INSERT INTO anchor_presentation_queue (anchor_kind, anchor_id)
        SELECT 2, id::text FROM patch_resource WHERE galgame_id = NEW.id
        ON CONFLICT (anchor_kind, anchor_id) DO UPDATE SET enqueued_at = clock_timestamp();
    END IF;
    RETURN NULL;
END
$$;

DROP TRIGGER IF EXISTS trg_anchor_presentation_patch ON patch;
CREATE TRIGGER trg_anchor_presentation_patch
    AFTER INSERT OR DELETE ON patch
    FOR EACH ROW EXECUTE FUNCTION anchor_presentation_patch();

DROP TRIGGER IF EXISTS trg_anchor_presentation_patch_update ON patch;
CREATE TRIGGER trg_anchor_presentation_patch_update
    AFTER UPDATE ON patch
    FOR EACH ROW
    WHEN (OLD.published IS DISTINCT FROM NEW.published
       OR OLD.content_limit IS DISTINCT FROM NEW.content_limit)
    EXECUTE FUNCTION anchor_presentation_patch();

CREATE OR REPLACE FUNCTION anchor_presentation_resource()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        PERFORM anchor_presentation_enqueue(2::smallint, OLD.id::text);
    ELSE
        PERFORM anchor_presentation_enqueue(2::smallint, NEW.id::text);
    END IF;
    RETURN NULL;
END
$$;

DROP TRIGGER IF EXISTS trg_anchor_presentation_resource ON patch_resource;
CREATE TRIGGER trg_anchor_presentation_resource
    AFTER INSERT OR DELETE ON patch_resource
    FOR EACH ROW EXECUTE FUNCTION anchor_presentation_resource();

-- Status 2 (hidden) takes the page down; a download or a like touches nothing
-- the presentation carries.
DROP TRIGGER IF EXISTS trg_anchor_presentation_resource_update ON patch_resource;
CREATE TRIGGER trg_anchor_presentation_resource_update
    AFTER UPDATE ON patch_resource
    FOR EACH ROW
    WHEN (OLD.status IS DISTINCT FROM NEW.status
       OR OLD.name IS DISTINCT FROM NEW.name
       OR OLD.galgame_id IS DISTINCT FROM NEW.galgame_id)
    EXECUTE FUNCTION anchor_presentation_resource();

INSERT INTO anchor_presentation_queue (anchor_kind, anchor_id)
SELECT 1, id::text FROM patch
UNION ALL
SELECT 2, id::text FROM patch_resource
ON CONFLICT (anchor_kind, anchor_id) DO NOTHING;
