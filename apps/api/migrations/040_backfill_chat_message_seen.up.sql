-- 040: chat_message_seen is the read model the new unread badges count on, and
-- it shipped empty. Without this backfill every message somebody else ever sent
-- becomes "new" the moment the badge goes live, and each member's 私聊 count
-- opens at the full backlog of every room they ever joined -- 68 messages on
-- the scrubbed dev snapshot, the whole history on production.
--
-- The read event is "opened the room", which happened for nobody before the
-- chat page learned to write seen rows, so the only defensible starting state
-- is "everything already there is read". Messages a member sent are not
-- notifications to themselves, so own rows are skipped.
--
-- Idempotent: ON CONFLICT DO NOTHING. A member with no rows in a room inserts
-- nothing, which is why this stays cheap on a re-run.
BEGIN;

INSERT INTO chat_message_seen (chat_message_id, user_id)
SELECT m.id, cm.user_id
FROM chat_message m
JOIN chat_member cm ON cm.chat_room_id = m.chat_room_id
WHERE m.sender_id <> cm.user_id
ON CONFLICT (user_id, chat_message_id) DO NOTHING;

COMMIT;
