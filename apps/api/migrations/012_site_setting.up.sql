-- Durable, audited source of truth for site-wide admin toggles (comment-verify,
-- creator-only, ...), replacing the previous Redis key/value approach.
--
-- Why move off Redis for THIS data:
--   * persistence — a Redis without AOF/RDB (or a flush) silently resets the
--     toggles to default; a "审核开关自己关掉" is a real operational hazard.
--   * single source of truth + backups — everything else lives in Postgres;
--     keeping these in Redis means pg_dump doesn't capture them.
--   * auditability — updated_by / updated_at record who changed what, when.
--   * cross-site safety — the old keys ("admin:enable_*") had no site prefix and
--     moyu shares one Redis with kungal, so they could collide. A per-site DB
--     table can't.
--
-- These flags are read only on write paths (publish / comment create) — low
-- frequency — so setting.Service reads this table directly (PK lookup); no cache
-- layer is needed. value is text today (bool "true"/"false"); widen to jsonb if
-- a non-bool setting is ever added.
--
-- updated_by has NO FK to "user" on purpose: settings must outlive any user
-- (including the admin user-purge flow), and it's an audit breadcrumb, not a
-- relation.
CREATE TABLE IF NOT EXISTS site_setting (
    key        varchar(100) PRIMARY KEY,
    value      text        NOT NULL DEFAULT '',
    updated_by integer,
    updated_at timestamptz NOT NULL DEFAULT now()
);
