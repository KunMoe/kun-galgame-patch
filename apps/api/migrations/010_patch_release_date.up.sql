-- Galgame 发售日期本地镜像列。
--
-- Purpose: enable sort/filter by 发售日期 on GET /api/galgame. The list
-- endpoint queries the local `patch` table (WHERE type + ORDER BY + paginate)
-- then enriches via Wiki batch. release_date lives on the Wiki galgame entity
-- (D12 moved it off the local row), so without a local mirror we can't
-- ORDER BY / WHERE on it at the SQL/pagination stage — enrichment happens
-- AFTER pagination, too late to drive it.
--
-- This column mirrors Wiki's galgame.release_date (PG `date`, day precision,
-- no timezone). It's a low-churn, near-static field (a game's release date is
-- a historical fact), so the A-lite sync model is: backfill once + stamp on
-- patch creation. See docs/galgame_wiki/00-handbook §17 for the filter
-- protocol (YYYY / YYYY-MM bounds, NULL auto-excluded by >= / <=).
--
-- Nullable: ~30% of user-published galgames have no release_date on Wiki.
-- Filtering by a date range auto-drops NULL rows (intended — "show me 2024
-- games" means games with a known 2024 date).

ALTER TABLE patch ADD COLUMN IF NOT EXISTS release_date date;

CREATE INDEX IF NOT EXISTS idx_patch_release_date ON patch (release_date);
