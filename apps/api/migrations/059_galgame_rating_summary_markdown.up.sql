-- 059: a rating's short review is Markdown (owner ruling 2026-11-05).
--
-- Catalog keeps the author's source in short_summary and cooks three read-only
-- fields from it at write time; moyu shows those and never renders the source:
--   * short_summary_html: sanitized HTML (catalog spec 2.83.0), spoilers as kun-spoiler-hidden elements;
--   * short_summary_text: plain text, spoilers masked as ███, for excerpts;
--   * short_summary_has_spoiler: the source holds at least one spoiler mask.
--
-- Existing rows: the new columns start empty, and the mirror cursor is reset so
-- the next drain replays every rating (~5k on 2026-10-13, one tick) and the
-- upsert fills them. The reset is what makes deploy order not matter: had
-- catalog's backfill reached the feed while moyu still ran the old binary, the
-- old mirror would have consumed those rows without the new fields and moved
-- its cursor past them for good.

ALTER TABLE galgame_rating
    ADD COLUMN IF NOT EXISTS short_summary_html        text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS short_summary_text        text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS short_summary_has_spoiler boolean NOT NULL DEFAULT false;

DELETE FROM cron_state WHERE name = 'catalog_rating_mirror';
