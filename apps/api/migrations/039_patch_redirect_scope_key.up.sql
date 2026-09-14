-- 039: patch_redirect gets a second producer, and its primary key was wrong for
-- two of them.
--
-- 037 created the table with `old_id` alone as the key, which was right while
-- cmd/align-patch-ids was the only writer: every row it wrote was scope=legacy.
-- The catalog merge cron now writes scope=merge rows, and the two scopes number
-- their old ids out of different spaces -- a pre-037 page number and a catalog
-- work id catalog merged away. Measured on production the day this shipped, 24
-- of catalog's 4,473 merged-away work ids are also a pre-037 page number that
-- already holds a legacy row. Under the old key the cron's upsert would have
-- taken those rows over, flipping their scope, and /patch/<n> would have
-- started answering 404 for 24 URLs that work today -- silently, because both
-- rows are individually well-formed.
--
-- The two lookups already filter on scope (repository/redirect_repo.go), so
-- widening the key changes no read. idx_patch_redirect_new stays: the merge
-- consumer flattens chains with `WHERE new_id = ?`, which is what that index
-- is for.

BEGIN;

ALTER TABLE patch_redirect DROP CONSTRAINT IF EXISTS patch_redirect_pkey;

ALTER TABLE patch_redirect ADD PRIMARY KEY (scope, old_id);

COMMENT ON TABLE patch_redirect IS
  'Old id -> the page that holds the game now (037, rekeyed 039). scope=legacy is a pre-037 page number, read only by the /patch/<n> shim; scope=merge is a catalog work id catalog merged away, read only by /galgame/<id>. The same number can be both.';

COMMIT;
