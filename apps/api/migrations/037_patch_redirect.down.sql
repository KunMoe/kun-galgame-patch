BEGIN;

ALTER TABLE patch_resource
  DROP CONSTRAINT IF EXISTS patch_resource_patch_id_fkey,
  ADD CONSTRAINT patch_resource_patch_id_fkey FOREIGN KEY (galgame_id)
    REFERENCES patch (id) ON DELETE CASCADE;

ALTER TABLE patch_comment
  DROP CONSTRAINT IF EXISTS patch_comment_patch_id_fkey,
  ADD CONSTRAINT patch_comment_patch_id_fkey FOREIGN KEY (galgame_id)
    REFERENCES patch (id) ON DELETE CASCADE;

ALTER TABLE patch_link
  DROP CONSTRAINT IF EXISTS patch_link_patch_id_fkey,
  ADD CONSTRAINT patch_link_patch_id_fkey FOREIGN KEY (galgame_id)
    REFERENCES patch (id) ON DELETE CASCADE;

ALTER TABLE user_patch_contribute_relation
  DROP CONSTRAINT IF EXISTS user_patch_contribute_relation_patch_id_fkey,
  ADD CONSTRAINT user_patch_contribute_relation_patch_id_fkey FOREIGN KEY (galgame_id)
    REFERENCES patch (id) ON DELETE CASCADE;

ALTER TABLE user_patch_favorite_relation
  DROP CONSTRAINT IF EXISTS user_patch_favorite_relation_patch_id_fkey,
  ADD CONSTRAINT user_patch_favorite_relation_patch_id_fkey FOREIGN KEY (galgame_id)
    REFERENCES patch (id) ON DELETE CASCADE;

DROP INDEX IF EXISTS idx_patch_redirect_new;
DROP TABLE IF EXISTS patch_redirect;

COMMIT;
