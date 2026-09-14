-- Rolling back needs one scope to win, because the old key cannot hold both
-- meanings of a number. legacy wins: those rows are the renumber's only record
-- of where a pre-037 URL went, while a merge row can be rebuilt by replaying
-- /v2/catalog/redirects from an empty cursor.
BEGIN;

DELETE FROM patch_redirect m
WHERE m.scope = 'merge'
  AND EXISTS (SELECT 1 FROM patch_redirect l WHERE l.old_id = m.old_id AND l.scope = 'legacy');

ALTER TABLE patch_redirect DROP CONSTRAINT IF EXISTS patch_redirect_pkey;

ALTER TABLE patch_redirect ADD PRIMARY KEY (old_id);

COMMIT;
