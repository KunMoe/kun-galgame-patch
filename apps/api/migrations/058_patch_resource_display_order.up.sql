-- 058: a curated display order for a page's resources (2026-10-31). Staff
-- holding resource.reorder pin resources to the top of a game's resource list
-- in a fixed order; every resource left unpinned is shuffled on each read so no
-- uploader owns the slot under the pins.
--
-- display_order is the 1-based position among the page's pinned resources;
-- NULL = not pinned. It is local to galgame_id: a catalog merge moves rows
-- with their numbers, and the survivor then orders both pages' pins by number.
--
-- New nullable column; every existing row starts unpinned.

ALTER TABLE patch_resource ADD COLUMN IF NOT EXISTS display_order integer;
