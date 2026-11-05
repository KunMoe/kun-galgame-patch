-- The cooked fields are catalog's; dropping the copy loses nothing.
ALTER TABLE galgame_rating
    DROP COLUMN IF EXISTS short_summary_html,
    DROP COLUMN IF EXISTS short_summary_text,
    DROP COLUMN IF EXISTS short_summary_has_spoiler;
