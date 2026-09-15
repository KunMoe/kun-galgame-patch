-- 040 down is a no-op on purpose. chat_message_seen is app-owned state: the
-- chat page rewrites it every time a room opens, and the rows this migration
-- wrote are indistinguishable from the ones the app writes. There is no subset
-- that is "the backfill's" to delete, so a down that removed anything would
-- silently un-read messages people have since seen.
BEGIN;

COMMIT;
