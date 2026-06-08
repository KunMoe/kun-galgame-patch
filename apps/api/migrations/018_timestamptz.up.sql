-- 018: Convert all naive `timestamp without time zone` columns to `timestamptz`.
--
-- WHY: the baseline schema (000) + 017 store created / updated / *_time as naive
-- `timestamp(3) without time zone` — a bare wall-clock with no zone. That is
-- fragile: the correct instant depends on the writer's and reader's timezones
-- agreeing, so any process / DB-session TZ drift silently shifts every value
-- (exactly the class of "off by 8h" bug). timestamptz stores an absolute
-- instant, so reads are correct regardless of session / process TZ and the
-- frontend renders per visitor automatically — Postgres's standing advice.
--
-- CONVERSION BASIS = 'UTC' (deliberately NOT 'Asia/Shanghai'). moyu's API
-- container runs in UTC, so GORM's time.Now() wrote the UTC wall-clock into
-- these naive columns. Verified against a DB-written tz-aware reference:
-- patch_resource.update_time (naive) AT TIME ZONE 'UTC' matches
-- patch_resource_revision.created_at (timestamptz) to the millisecond. So
-- `<col> AT TIME ZONE 'UTC'` reinterprets each stored wall-clock AS the UTC
-- instant it actually is, preserving the true instant.
--   ⚠ The sibling kun-galgame-forum stores BEIJING wall-clock (its API runs in
--   Asia/Shanghai) and converts with 'Asia/Shanghai'. Do NOT copy that here —
--   on moyu it would shift every timestamp 8h. The basis was re-verified for
--   moyu specifically.
--
-- ASSUMPTION: every existing naive value was written by a UTC process. If any
-- historical rows were written under a non-UTC process, they would need
-- separate handling — none were found (the tz-aware cross-check held across the
-- data sampled), but re-verify on prod before applying there.
--
-- EXCLUDED: `_migrations.applied_at` (migrate tooling's own table) and
-- `patch_resource_update_time_bak_20260606` (a one-off manual backup table).
-- Already-tzaware columns (migrations 006/007/013/014/015 + site_setting /
-- cron_state / doc / wiki_message_*) are left untouched.
--
-- No views / matviews / generated columns depend on these columns (pre-flight
-- checked), so ALTER COLUMN TYPE rebuilds their indexes in place without error.

BEGIN;

ALTER TABLE admin_log ALTER COLUMN created TYPE timestamptz USING created AT TIME ZONE 'UTC';
ALTER TABLE admin_log ALTER COLUMN updated TYPE timestamptz USING updated AT TIME ZONE 'UTC';

ALTER TABLE chat_member ALTER COLUMN created TYPE timestamptz USING created AT TIME ZONE 'UTC';
ALTER TABLE chat_member ALTER COLUMN updated TYPE timestamptz USING updated AT TIME ZONE 'UTC';

ALTER TABLE chat_message ALTER COLUMN created TYPE timestamptz USING created AT TIME ZONE 'UTC';
ALTER TABLE chat_message ALTER COLUMN updated TYPE timestamptz USING updated AT TIME ZONE 'UTC';
ALTER TABLE chat_message ALTER COLUMN deleted_at TYPE timestamptz USING deleted_at AT TIME ZONE 'UTC';

ALTER TABLE chat_message_edit_history ALTER COLUMN edited_at TYPE timestamptz USING edited_at AT TIME ZONE 'UTC';

ALTER TABLE chat_message_reaction ALTER COLUMN created TYPE timestamptz USING created AT TIME ZONE 'UTC';
ALTER TABLE chat_message_reaction ALTER COLUMN updated TYPE timestamptz USING updated AT TIME ZONE 'UTC';

ALTER TABLE chat_message_seen ALTER COLUMN read_at TYPE timestamptz USING read_at AT TIME ZONE 'UTC';

ALTER TABLE chat_room ALTER COLUMN created TYPE timestamptz USING created AT TIME ZONE 'UTC';
ALTER TABLE chat_room ALTER COLUMN updated TYPE timestamptz USING updated AT TIME ZONE 'UTC';
ALTER TABLE chat_room ALTER COLUMN last_message_time TYPE timestamptz USING last_message_time AT TIME ZONE 'UTC';

ALTER TABLE patch ALTER COLUMN created TYPE timestamptz USING created AT TIME ZONE 'UTC';
ALTER TABLE patch ALTER COLUMN updated TYPE timestamptz USING updated AT TIME ZONE 'UTC';
ALTER TABLE patch ALTER COLUMN resource_update_time TYPE timestamptz USING resource_update_time AT TIME ZONE 'UTC';

ALTER TABLE patch_comment ALTER COLUMN created TYPE timestamptz USING created AT TIME ZONE 'UTC';
ALTER TABLE patch_comment ALTER COLUMN updated TYPE timestamptz USING updated AT TIME ZONE 'UTC';

ALTER TABLE patch_link ALTER COLUMN created TYPE timestamptz USING created AT TIME ZONE 'UTC';
ALTER TABLE patch_link ALTER COLUMN updated TYPE timestamptz USING updated AT TIME ZONE 'UTC';

ALTER TABLE patch_resource ALTER COLUMN created TYPE timestamptz USING created AT TIME ZONE 'UTC';
ALTER TABLE patch_resource ALTER COLUMN updated TYPE timestamptz USING updated AT TIME ZONE 'UTC';
ALTER TABLE patch_resource ALTER COLUMN update_time TYPE timestamptz USING update_time AT TIME ZONE 'UTC';

ALTER TABLE "user" ALTER COLUMN created TYPE timestamptz USING created AT TIME ZONE 'UTC';
ALTER TABLE "user" ALTER COLUMN updated TYPE timestamptz USING updated AT TIME ZONE 'UTC';

ALTER TABLE user_message ALTER COLUMN created TYPE timestamptz USING created AT TIME ZONE 'UTC';
ALTER TABLE user_message ALTER COLUMN updated TYPE timestamptz USING updated AT TIME ZONE 'UTC';

ALTER TABLE user_patch_comment_like_relation ALTER COLUMN created TYPE timestamptz USING created AT TIME ZONE 'UTC';
ALTER TABLE user_patch_comment_like_relation ALTER COLUMN updated TYPE timestamptz USING updated AT TIME ZONE 'UTC';

ALTER TABLE user_patch_contribute_relation ALTER COLUMN created TYPE timestamptz USING created AT TIME ZONE 'UTC';
ALTER TABLE user_patch_contribute_relation ALTER COLUMN updated TYPE timestamptz USING updated AT TIME ZONE 'UTC';

ALTER TABLE user_patch_favorite_relation ALTER COLUMN created TYPE timestamptz USING created AT TIME ZONE 'UTC';
ALTER TABLE user_patch_favorite_relation ALTER COLUMN updated TYPE timestamptz USING updated AT TIME ZONE 'UTC';

ALTER TABLE user_patch_resource_favorite_relation ALTER COLUMN created TYPE timestamptz USING created AT TIME ZONE 'UTC';
ALTER TABLE user_patch_resource_favorite_relation ALTER COLUMN updated TYPE timestamptz USING updated AT TIME ZONE 'UTC';

ALTER TABLE user_patch_resource_like_relation ALTER COLUMN created TYPE timestamptz USING created AT TIME ZONE 'UTC';
ALTER TABLE user_patch_resource_like_relation ALTER COLUMN updated TYPE timestamptz USING updated AT TIME ZONE 'UTC';

COMMIT;
