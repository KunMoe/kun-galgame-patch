-- 055: the archive-level comment of a hosted resource's file (letmoe forge,
-- 2026-10-23). A forge zip carries no comment of its own, so the one the
-- author's RAR held (a usage note, a release post, 解压密码：…) was lost with
-- the swap. Sampled 2026-10-23: 15 of 300 original RARs had one; zip and 7z
-- originals had none.
--
-- patch_resource.archive_comment is the comment of the resource's current
-- file's original, plain text. The author replacing the file clears it; the
-- normalize complete writes it.
--
-- patch_resource_normalize.source_comment is what the forge read from the
-- source file: NULL = not read yet, '' = read, no comment. The backfill of
-- already-swapped files reads the originals 050 still holds (30 days from the
-- swap, so the first, 2026-10-15, are deleted from 2026-11-14) and lists only
-- rows still NULL.
--
-- New nullable / defaulted columns only; no existing rows are rewritten.

ALTER TABLE patch_resource ADD COLUMN IF NOT EXISTS archive_comment text NOT NULL DEFAULT '';

ALTER TABLE patch_resource_normalize ADD COLUMN IF NOT EXISTS source_comment text;
