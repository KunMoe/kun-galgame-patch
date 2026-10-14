-- 049: clear the BLAKE3 a resource kept after its file was replaced.
--
-- Artifact uploads record no hash (no row created after 2026-06-04 has one),
-- and until PR #109 replacing a file never rewrote patch_resource.blake3. The
-- column went on naming the replaced file's bytes: 119 live rows on production
-- on 2026-10-14, shown as the 下载 tab's checksum and as the public face's
-- `hash`, so a reader verifying a download saw a mismatch. #109 clears the
-- column on every file change from then on; this clears the rows before it.
--
-- A row is cleared when its own file history records the current hash as an
-- OLD file's hash. The value is not lost: it is that history row's old_blake3.
-- A file replaced by identical bytes is cleared too, which costs a hash that
-- was still right: empty means unknown, never wrong.
--
-- blake3 is in none of patch_resource's trigger conditions (043, 045, 047,
-- 048), so this enqueues nothing and writes no change-feed entry.
UPDATE patch_resource r
SET blake3 = ''
WHERE r.blake3 <> ''
  AND EXISTS (
      SELECT 1 FROM patch_resource_file_history h
      WHERE h.resource_id = r.id AND h.old_blake3 = r.blake3
  );
