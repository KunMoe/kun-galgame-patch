-- 052: record what every catalog merge did to this site's rows, so an undone
-- merge can put them back.
--
-- Catalog can undo a merge since spec 2.62.0 (infra #453): within 30 days the
-- retired id comes back live and /v2/catalog/redirects lists the row again with
-- revoked = true. The merge cron had already acted on the merge: it renumbered
-- the retired id's page onto the survivor's number, or folded it into the page
-- already there (resources moved, duplicate join rows dropped, the page row
-- deleted, counters summed). Nothing kept what it moved, so an undo could only
-- drop the redirect and leave the retired work's resources on another game.
--
-- patch_merge_record is one row per merge the cron applied, keyed by the
-- retired id. patch_merge_record_row holds a full snapshot (to_jsonb) of every
-- row the merge touched, taken before it touched it: the retired page and the
-- survivor page (change = before), every child row of the retired page (moved),
-- the child rows the fold dropped as duplicates (dropped), the survivor as the
-- fold left it (after), and the patch_redirect rows the cron flattened onto
-- the survivor (repointed). The undo moves rows back by primary key, re-inserts
-- dropped ones, restores survivor columns the fold set unless something changed
-- them since, and keeps on the survivor whatever was published there after the
-- merge. undone_at / undo_report say what it did and what it could not do.
--
-- New tables only. Merges applied before this migration have no record and
-- cannot be put back.

BEGIN;

CREATE TABLE IF NOT EXISTS patch_merge_record (
    id          bigserial PRIMARY KEY,
    old_id      integer NOT NULL,
    new_id      integer NOT NULL,
    action      text NOT NULL,
    applied_at  timestamptz NOT NULL DEFAULT now(),
    undone_at   timestamptz,
    undo_report jsonb
);

CREATE INDEX IF NOT EXISTS idx_patch_merge_record_open
    ON patch_merge_record (old_id) WHERE undone_at IS NULL;

CREATE TABLE IF NOT EXISTS patch_merge_record_row (
    id         bigserial PRIMARY KEY,
    record_id  bigint NOT NULL REFERENCES patch_merge_record (id) ON DELETE CASCADE,
    table_name text NOT NULL,
    row_id     bigint NOT NULL,
    change     text NOT NULL,
    value      jsonb NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_patch_merge_record_row_record
    ON patch_merge_record_row (record_id);

COMMENT ON TABLE patch_merge_record IS
  'One row per catalog merge the merge cron applied (052): old_id merged into new_id by action renumber, fold or ledger. undone_at/undo_report once catalog revoked it.';
COMMENT ON TABLE patch_merge_record_row IS
  'Snapshots of every row a merge touched, taken before it touched it (052). change: before, after, moved, dropped, repointed.';

COMMIT;
