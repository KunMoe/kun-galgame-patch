-- 019: one-time cleanup of notification messages whose link target no longer
-- exists (deleted / remapped patches & resources).
--
-- user_message has no FK to patch / patch_resource, so when a patch (or its
-- resources) is deleted/remapped, the patchResourceCreate / patchResourceUpdate
-- notifications pointing at it (`/patch/:id/resource`, `/resource/:id`) are left
-- dangling — clicking one 404s ("补丁更新通知跳转错误"). At time of writing this
-- was ~2,987 create + ~1,359 update rows across 359 missing patches (old ids).
--
-- This deletes ONLY rows whose link is exactly a patch resource-list or a
-- resource detail link AND whose target row is gone — valid notifications and
-- other link shapes (mentions, comments, …) are untouched.
--
-- Recurrence is prevented going forward by the in-tx cleanup added to the patch
-- + admin repositories' DeletePatch / DeleteResource (commit pairing this).
--
-- Irreversible: deleted notifications can't be restored — the down is a no-op.

DELETE FROM user_message
WHERE (
        link ~ '^/patch/[0-9]+/resource$'
        AND NOT EXISTS (
          SELECT 1 FROM patch p
          WHERE p.id = substring(link FROM '^/patch/([0-9]+)/resource$')::int
        )
      )
   OR (
        link ~ '^/resource/[0-9]+$'
        AND NOT EXISTS (
          SELECT 1 FROM patch_resource r
          WHERE r.id = substring(link FROM '^/resource/([0-9]+)$')::int
        )
      );
