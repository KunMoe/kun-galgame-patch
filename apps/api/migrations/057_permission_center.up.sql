-- 057: the permission center (2026-10-31). Until now every staff power on moyu
-- was a hard-coded role list (middleware.ModeratorRoles / SuperAdminRoles); a
-- power could not be lent to one more role or one person without a deploy.
--
-- The compiled catalog and per-role baseline live in pkg/perm. These tables
-- store ONLY deviations from that baseline, one row per (role|user,
-- permission): 'grant' adds a key the baseline lacks, 'revoke' removes one it
-- has. Empty tables mean every role and user holds exactly the baseline.
--
-- ren always holds the whole catalog and is never stored here; the write path
-- refuses rows that target ren or a user holding it.
--
-- permission_change records every replacement of a role's or a user's override
-- set, before and after, with the reason given, so a power that appeared or
-- vanished can be traced. Its newest id per target is also the version a write
-- must name, so two admins editing at once cannot silently overwrite each other.
--
-- New tables only; no existing rows are touched.

CREATE TABLE IF NOT EXISTS role_permission_override (
    role       text NOT NULL,
    permission text NOT NULL,
    effect     text NOT NULL CHECK (effect IN ('grant', 'revoke')),
    updated_by integer NOT NULL DEFAULT 0,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (role, permission)
);

CREATE TABLE IF NOT EXISTS user_permission_override (
    user_id    integer NOT NULL,
    permission text NOT NULL,
    effect     text NOT NULL CHECK (effect IN ('grant', 'revoke')),
    updated_by integer NOT NULL DEFAULT 0,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, permission)
);

CREATE TABLE IF NOT EXISTS permission_change (
    id             serial PRIMARY KEY,
    actor_id       integer NOT NULL,
    target_role    text,
    target_user_id integer,
    before         jsonb NOT NULL DEFAULT '[]',
    after          jsonb NOT NULL DEFAULT '[]',
    reason         text NOT NULL DEFAULT '',
    created        timestamptz NOT NULL DEFAULT now(),
    CHECK ((target_role IS NULL) <> (target_user_id IS NULL))
);

CREATE INDEX IF NOT EXISTS permission_change_created_idx ON permission_change (created DESC, id DESC);
CREATE INDEX IF NOT EXISTS permission_change_role_idx ON permission_change (target_role, id DESC) WHERE target_role IS NOT NULL;
CREATE INDEX IF NOT EXISTS permission_change_user_idx ON permission_change (target_user_id, id DESC) WHERE target_user_id IS NOT NULL;
