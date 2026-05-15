BEGIN;

DROP TABLE IF EXISTS wiki_message_read_state;
DROP TABLE IF EXISTS wiki_message_processed;
DROP TABLE IF EXISTS cron_state;

COMMIT;
