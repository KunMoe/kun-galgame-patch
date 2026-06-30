-- 023: Add patch.is_stub — distinguish an interaction-materialized stub from a
-- real registration, so the first real publish can ADOPT a stub correctly.
--
-- WHY: a 未收录 galgame's local patch row is now lazily created on the first
-- INTERACTION (favorite/comment/resource via ensureLocalPatch) with the wiki
-- entry creator as a PLACEHOLDER owner (user_id) and no +3 reward. When someone
-- later actually publishes (发布补丁 → createPatchRow), the idempotent dedup used
-- to early-return, so the real publisher inherited the placeholder owner and got
-- no +3 / no contributor / no owner-gating rights. There is no reliable
-- column-only heuristic to tell a stub from a real registration (a comment-stub
-- bumps contribute_count just like a registration does), hence this explicit
-- marker.
--
-- ensureLocalPatch sets is_stub=true; createPatchRow inserts a fresh
-- registration with is_stub=false and, on finding an existing is_stub=true row,
-- adopts it (transfer user_id, clear the flag, register the contributor, +3).
--
-- Idempotent: ADD COLUMN IF NOT EXISTS; the backfill only touches not-yet-marked
-- rows, so a re-run on each deploy is a no-op.

ALTER TABLE patch ADD COLUMN IF NOT EXISTS is_stub boolean NOT NULL DEFAULT false;

-- Backfill legacy stubs (incl. rows left by the old materialize-on-VIEW
-- behavior): no resources AND no contributors ⇒ never registered via
-- CreatePatch/claim (which always make the registrant the first contributor),
-- only viewed/interacted ⇒ a stub that a later publish should be able to adopt.
UPDATE patch SET is_stub = true
WHERE resource_count = 0 AND contribute_count = 0 AND is_stub = false;
