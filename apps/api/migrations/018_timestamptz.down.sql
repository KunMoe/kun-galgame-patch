-- Revert 018: timestamptz → naive `timestamp(3) without time zone`.
--
-- `<col> AT TIME ZONE 'UTC'` on a timestamptz yields the wall-clock at UTC as a
-- naive timestamp — the exact inverse of the up migration, so it restores the
-- original stored values (UTC wall-clock) byte-for-byte. timestamp(3) preserves
-- the original millisecond precision.

BEGIN;

ALTER TABLE admin_log ALTER COLUMN created TYPE timestamp(3) without time zone USING created AT TIME ZONE 'UTC';
ALTER TABLE admin_log ALTER COLUMN updated TYPE timestamp(3) without time zone USING updated AT TIME ZONE 'UTC';

ALTER TABLE chat_member ALTER COLUMN created TYPE timestamp(3) without time zone USING created AT TIME ZONE 'UTC';
ALTER TABLE chat_member ALTER COLUMN updated TYPE timestamp(3) without time zone USING updated AT TIME ZONE 'UTC';

ALTER TABLE chat_message ALTER COLUMN created TYPE timestamp(3) without time zone USING created AT TIME ZONE 'UTC';
ALTER TABLE chat_message ALTER COLUMN updated TYPE timestamp(3) without time zone USING updated AT TIME ZONE 'UTC';
ALTER TABLE chat_message ALTER COLUMN deleted_at TYPE timestamp(3) without time zone USING deleted_at AT TIME ZONE 'UTC';

ALTER TABLE chat_message_edit_history ALTER COLUMN edited_at TYPE timestamp(3) without time zone USING edited_at AT TIME ZONE 'UTC';

ALTER TABLE chat_message_reaction ALTER COLUMN created TYPE timestamp(3) without time zone USING created AT TIME ZONE 'UTC';
ALTER TABLE chat_message_reaction ALTER COLUMN updated TYPE timestamp(3) without time zone USING updated AT TIME ZONE 'UTC';

ALTER TABLE chat_message_seen ALTER COLUMN read_at TYPE timestamp(3) without time zone USING read_at AT TIME ZONE 'UTC';

ALTER TABLE chat_room ALTER COLUMN created TYPE timestamp(3) without time zone USING created AT TIME ZONE 'UTC';
ALTER TABLE chat_room ALTER COLUMN updated TYPE timestamp(3) without time zone USING updated AT TIME ZONE 'UTC';
ALTER TABLE chat_room ALTER COLUMN last_message_time TYPE timestamp(3) without time zone USING last_message_time AT TIME ZONE 'UTC';

ALTER TABLE patch ALTER COLUMN created TYPE timestamp(3) without time zone USING created AT TIME ZONE 'UTC';
ALTER TABLE patch ALTER COLUMN updated TYPE timestamp(3) without time zone USING updated AT TIME ZONE 'UTC';
ALTER TABLE patch ALTER COLUMN resource_update_time TYPE timestamp(3) without time zone USING resource_update_time AT TIME ZONE 'UTC';

ALTER TABLE patch_comment ALTER COLUMN created TYPE timestamp(3) without time zone USING created AT TIME ZONE 'UTC';
ALTER TABLE patch_comment ALTER COLUMN updated TYPE timestamp(3) without time zone USING updated AT TIME ZONE 'UTC';

ALTER TABLE patch_link ALTER COLUMN created TYPE timestamp(3) without time zone USING created AT TIME ZONE 'UTC';
ALTER TABLE patch_link ALTER COLUMN updated TYPE timestamp(3) without time zone USING updated AT TIME ZONE 'UTC';

ALTER TABLE patch_resource ALTER COLUMN created TYPE timestamp(3) without time zone USING created AT TIME ZONE 'UTC';
ALTER TABLE patch_resource ALTER COLUMN updated TYPE timestamp(3) without time zone USING updated AT TIME ZONE 'UTC';
ALTER TABLE patch_resource ALTER COLUMN update_time TYPE timestamp(3) without time zone USING update_time AT TIME ZONE 'UTC';

ALTER TABLE "user" ALTER COLUMN created TYPE timestamp(3) without time zone USING created AT TIME ZONE 'UTC';
ALTER TABLE "user" ALTER COLUMN updated TYPE timestamp(3) without time zone USING updated AT TIME ZONE 'UTC';

ALTER TABLE user_message ALTER COLUMN created TYPE timestamp(3) without time zone USING created AT TIME ZONE 'UTC';
ALTER TABLE user_message ALTER COLUMN updated TYPE timestamp(3) without time zone USING updated AT TIME ZONE 'UTC';

ALTER TABLE user_patch_comment_like_relation ALTER COLUMN created TYPE timestamp(3) without time zone USING created AT TIME ZONE 'UTC';
ALTER TABLE user_patch_comment_like_relation ALTER COLUMN updated TYPE timestamp(3) without time zone USING updated AT TIME ZONE 'UTC';

ALTER TABLE user_patch_contribute_relation ALTER COLUMN created TYPE timestamp(3) without time zone USING created AT TIME ZONE 'UTC';
ALTER TABLE user_patch_contribute_relation ALTER COLUMN updated TYPE timestamp(3) without time zone USING updated AT TIME ZONE 'UTC';

ALTER TABLE user_patch_favorite_relation ALTER COLUMN created TYPE timestamp(3) without time zone USING created AT TIME ZONE 'UTC';
ALTER TABLE user_patch_favorite_relation ALTER COLUMN updated TYPE timestamp(3) without time zone USING updated AT TIME ZONE 'UTC';

ALTER TABLE user_patch_resource_favorite_relation ALTER COLUMN created TYPE timestamp(3) without time zone USING created AT TIME ZONE 'UTC';
ALTER TABLE user_patch_resource_favorite_relation ALTER COLUMN updated TYPE timestamp(3) without time zone USING updated AT TIME ZONE 'UTC';

ALTER TABLE user_patch_resource_like_relation ALTER COLUMN created TYPE timestamp(3) without time zone USING created AT TIME ZONE 'UTC';
ALTER TABLE user_patch_resource_like_relation ALTER COLUMN updated TYPE timestamp(3) without time zone USING updated AT TIME ZONE 'UTC';

COMMIT;
