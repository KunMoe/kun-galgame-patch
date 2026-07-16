-- 024_trust_disposition_applied
--
-- Trust & Safety enforcement (Phase 2) idempotency ledger. The trust dispatch
-- worker may re-deliver a callback (retry / at-least-once); disposition_id is
-- the dedup key, so a replayed callback is a no-op. Matters most for warn_user
-- (hide/remove are already idempotent set-status / delete).
--
-- Unlike kungal's migration 055, moyu does NOT need to ADD a status column to
-- its content tables: patch_comment.status (0=visible / 1=hidden) and
-- patch_resource.status (0=enabled / 1=disabled / 2=moderation-hidden) already
-- exist, so a `hide` disposition reuses them.
CREATE TABLE IF NOT EXISTS trust_disposition_applied (
    disposition_id bigint      PRIMARY KEY,
    action         smallint    NOT NULL,
    applied_at     timestamptz NOT NULL DEFAULT now()
);
