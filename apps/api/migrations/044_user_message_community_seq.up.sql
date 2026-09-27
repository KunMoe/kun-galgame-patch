-- 044: order mirrored community notifications by their upstream seq.
--
-- The mirror (internal/community/inbox) versioned a row by community's
-- updated_at. That is the fold transaction's now(), its START time, not its
-- commit order: a kind-10 retraction can start first, wait for the dispatcher
-- lock and commit after a fold update that started later, carrying the older
-- updated_at. Guarded by updated_at, that retraction reads as stale and the
-- local row is never deleted. seq is assigned under the dispatcher lock and is
-- the contract's order.
--
-- Existing mirrored rows keep NULL here and compare as seq 0, so the next
-- upstream change to any of them always applies.

ALTER TABLE user_message ADD COLUMN IF NOT EXISTS community_seq bigint;
