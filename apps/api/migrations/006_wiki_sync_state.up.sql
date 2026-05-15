-- 006: Tables backing the wiki-messages cron and the per-user read marker.
--
-- See docs/galgame_wiki/00-handbook-for-downstream.md §6 / §7 / §8.
--
--   * cron_state         — generic cron cursor table (one row per cron name).
--   * wiki_message_processed — idempotency log keyed by Wiki message_id.
--   * wiki_message_read_state — per-user "last read" marker for the
--                              notification center's unread badge.
--
-- The cron pulls Wiki's /galgame/messages/feed every 10 minutes and applies
-- approved/declined/banned/unbanned events. Idempotency is critical: a
-- single message_id must not award moemoepoint twice on retry.

BEGIN;

CREATE TABLE IF NOT EXISTS cron_state (
    name        VARCHAR(64) PRIMARY KEY,
    last_id     BIGINT NOT NULL DEFAULT 0,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS wiki_message_processed (
    message_id   BIGINT PRIMARY KEY,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS wiki_message_read_state (
    user_id              INT PRIMARY KEY,
    last_read_message_id BIGINT NOT NULL DEFAULT 0,
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMIT;
