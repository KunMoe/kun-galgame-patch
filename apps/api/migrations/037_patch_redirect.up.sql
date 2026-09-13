-- 037: the page id stops being this site's own number and starts being
-- catalog's work id. Two things have to exist before that can happen: a place
-- to record where every page went, and a schema that lets a page id move at
-- all.
--
-- Why the ids differed. Measured on production 2026-09-13 over 64,694 kungal
-- claims, as (gid - work_id):
--
--   gid 1-62,810      (the wiki import)   median +185, max +285
--   gid 62,811-70,000 (own sequence)      a constant -163,863
--   gid 200,000+      (games minted now)  0 / 0 / 0  -- already identical
--
-- The design had already converged: a game created today takes catalog's id as
-- its gid, 182 of 182. Only the legacy block was stranded, and every derived
-- copy of the mapping rotted against it -- 63 patch rows carried another
-- game's work id. cmd/align-patch-ids closes the block; gid == work_id is an
-- identity from here on.
--
-- patch_redirect is what makes the renumber survivable. 9,519 of the old ids
-- are also somebody's new id -- work_id is roughly gid - 185, so the number in
-- an old URL usually stays a live page and merely means a different game now.
-- 8,484 moving rows sit in that overlap, 5,731 of them published and indexed.
-- The old numeric namespace therefore cannot keep serving pages: the canonical
-- page moved to /galgame/<id>, and /patch/<n> answers ONLY out of this ledger
-- and 301s. The two namespaces can never be confused for one another again.
--
-- Catalog's own work merges land here too -- a merged-away work takes its page
-- with it -- which is why the table outlives the renumber instead of being a
-- one-off mapping file. `scope` keeps the two apart, and it is load-bearing:
--
--   legacy  old_id is a pre-037 page number. Honoured ONLY under /patch/<n>.
--           4107 was this site's page for work 4082, and 4107 is ALSO a live
--           catalog work of its own -- a different game with its own
--           /galgame/4107. Reading a legacy row on the new path would hand one
--           number both meanings again, which is the bug this table exists to
--           avoid.
--   merge   old_id is a catalog work id catalog merged away. Honoured on
--           /galgame/<id>, and unambiguous there because a merged-away id stops
--           being a work at all.
--
-- No foreign key on either column: old_id names a row that no longer exists,
-- and new_id has to stay rewritable so a later merge can be chased forward
-- rather than leaving the chain pointing at a deleted page.

BEGIN;

CREATE TABLE IF NOT EXISTS patch_redirect (
  old_id  integer     PRIMARY KEY,
  new_id  integer     NOT NULL,
  scope   text        NOT NULL CHECK (scope IN ('legacy', 'merge')),
  created timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_patch_redirect_new ON patch_redirect (new_id);

COMMENT ON TABLE patch_redirect IS
  'Old page id -> the page that holds the game now (037). scope=legacy is read only by the /patch/<n> shim, scope=merge only by /galgame/<id> when the work is gone.';

-- Every child was ON DELETE CASCADE / ON UPDATE NO ACTION, which is the right
-- pair for a key this site mints and the wrong one for a key catalog mints:
-- renumbering a parent answered "update or delete on table patch violates
-- foreign key constraint" and no ordering of the statements avoids it, because
-- the children cannot be moved to a parent that does not exist yet either.
-- ON UPDATE CASCADE makes the children follow the page instead, which is also
-- what a later catalog merge needs.

ALTER TABLE patch_resource
  DROP CONSTRAINT IF EXISTS patch_resource_patch_id_fkey,
  ADD CONSTRAINT patch_resource_patch_id_fkey FOREIGN KEY (galgame_id)
    REFERENCES patch (id) ON UPDATE CASCADE ON DELETE CASCADE;

ALTER TABLE patch_comment
  DROP CONSTRAINT IF EXISTS patch_comment_patch_id_fkey,
  ADD CONSTRAINT patch_comment_patch_id_fkey FOREIGN KEY (galgame_id)
    REFERENCES patch (id) ON UPDATE CASCADE ON DELETE CASCADE;

ALTER TABLE patch_link
  DROP CONSTRAINT IF EXISTS patch_link_patch_id_fkey,
  ADD CONSTRAINT patch_link_patch_id_fkey FOREIGN KEY (galgame_id)
    REFERENCES patch (id) ON UPDATE CASCADE ON DELETE CASCADE;

ALTER TABLE user_patch_contribute_relation
  DROP CONSTRAINT IF EXISTS user_patch_contribute_relation_patch_id_fkey,
  ADD CONSTRAINT user_patch_contribute_relation_patch_id_fkey FOREIGN KEY (galgame_id)
    REFERENCES patch (id) ON UPDATE CASCADE ON DELETE CASCADE;

ALTER TABLE user_patch_favorite_relation
  DROP CONSTRAINT IF EXISTS user_patch_favorite_relation_patch_id_fkey,
  ADD CONSTRAINT user_patch_favorite_relation_patch_id_fkey FOREIGN KEY (galgame_id)
    REFERENCES patch (id) ON UPDATE CASCADE ON DELETE CASCADE;

COMMIT;
