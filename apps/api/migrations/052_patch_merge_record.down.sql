-- Forgets what applied merges moved; an undone merge can then only drop its
-- redirect, as before 052.
BEGIN;

DROP TABLE IF EXISTS patch_merge_record_row;
DROP TABLE IF EXISTS patch_merge_record;

COMMIT;
