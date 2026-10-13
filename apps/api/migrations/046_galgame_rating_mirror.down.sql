-- The ratings themselves live in NextMoe; dropping the copy loses nothing, and
-- the cursor row goes with it so a re-apply drains from the start again.
DROP TABLE IF EXISTS galgame_rating;
DELETE FROM cron_state WHERE name = 'catalog_rating_mirror';
