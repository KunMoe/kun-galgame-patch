-- 041: mirror community-service notifications into user_message.
--
-- Community now writes reply / mention / watch / like notices itself (a
-- transactional outbox) and exposes them on GET /notifications/feed. moyu
-- copies each row into user_message so the existing inbox, bell and
-- mute-by-type UI keep working. Without these columns a fold that grows
-- (same notification id, new seq) cannot be upserted, and a thread read
-- cannot mark the mirrored rows read.
--
-- Existing user_message rows are untouched: the new columns stay NULL and
-- the partial unique index does not apply to them.

ALTER TABLE user_message
  ADD COLUMN IF NOT EXISTS community_notification_id bigint,
  ADD COLUMN IF NOT EXISTS community_thread_id bigint,
  ADD COLUMN IF NOT EXISTS community_post_number int;

CREATE UNIQUE INDEX IF NOT EXISTS user_message_community_notification_id_key
  ON user_message (community_notification_id)
  WHERE community_notification_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS user_message_community_thread_unread_idx
  ON user_message (recipient_id, community_thread_id)
  WHERE community_thread_id IS NOT NULL AND status = 0;
