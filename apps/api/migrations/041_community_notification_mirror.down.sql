-- Reverse 041: drop the community-notification mirror columns and indexes.
-- Existing user_message rows keep their type/content; only the community_*
-- columns and uniqueness go. The inbox cron cursor is left in cron_state
-- so a roll-forward resumes in place.

DROP INDEX IF EXISTS user_message_community_thread_unread_idx;
DROP INDEX IF EXISTS user_message_community_notification_id_key;

ALTER TABLE user_message
  DROP COLUMN IF EXISTS community_notification_id,
  DROP COLUMN IF EXISTS community_thread_id,
  DROP COLUMN IF EXISTS community_post_number;
