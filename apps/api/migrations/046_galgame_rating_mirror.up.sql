-- 046: galgame_rating, moyu's copy of NextMoe's central ratings.
--
-- A rating is NextMoe catalog's, one per person per work across every NextMoe
-- site (infra spec 2.44+, owner rulings 2026-10-11). moyu writes it with the
-- reader's token through /v2/me/work-records and keeps this table equal to
-- /v2/ratings/changes, which the rating mirror cron drains by id. moyu had no
-- ratings of its own, so nothing is imported: the first drain from an empty
-- cursor is the whole population (4,749 rows on 2026-10-13), almost all written
-- on other sites.
--
-- No foreign keys, deliberately:
--   * work_id names a catalog work, which is the page id (iron rule 3) whether
--     or not a patch row exists. letmoe kept only ratings of works it already
--     had and lost 88 for good, because a work arrived after its ratings and
--     the feed never sent them again; the forum's RESTRICT key would wedge the
--     mirror on the first such row.
--   * user_id is the central user id; most authors have never signed in here,
--     so the local user table does not have them.
--
-- Nothing reads this table to push anything: no trigger, no activity, no
-- notification. client_id is the OAuth client the rating was first written
-- from, and is the only way to tell moyu's ratings apart -- moyu's client is
-- bound to catalog site kungal like the forum's, so site cannot.
--
-- created_at/updated_at are catalog's RFC 3339 seconds, stored as given and
-- never set locally, so the mirror's IS DISTINCT FROM sees an unchanged row as
-- unchanged.

CREATE TABLE IF NOT EXISTS galgame_rating (
    id                bigint PRIMARY KEY,
    work_id           bigint NOT NULL,
    user_id           int NOT NULL,
    overall           smallint NOT NULL,
    art               smallint,
    story             smallint,
    music             smallint,
    "character"       smallint,
    route             smallint,
    "system"          smallint,
    voice             smallint,
    replay_value      smallint,
    recommend         varchar(16) NOT NULL,
    spoiler_level     varchar(16) NOT NULL,
    short_summary     text NOT NULL DEFAULT '',
    author_state      varchar(16),
    author_completion varchar(16),
    client_id         varchar(64) NOT NULL DEFAULT '',
    site              varchar(64) NOT NULL DEFAULT '',
    created_at        timestamptz NOT NULL,
    updated_at        timestamptz NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_galgame_rating_user_work
    ON galgame_rating (user_id, work_id);

CREATE INDEX IF NOT EXISTS idx_galgame_rating_work_created
    ON galgame_rating (work_id, created_at DESC, id DESC);
