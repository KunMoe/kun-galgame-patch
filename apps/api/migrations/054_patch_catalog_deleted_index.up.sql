-- 054: index the pages catalog deleted (051).
--
-- utils.LiveWorkResource probes patch for a deleted work once per resource.
-- With no index on catalog_deleted_at, production planned that probe as a seq
-- scan of all of patch per resource (1.3ms each, 2026-10-23), so the
-- normalize claim and the per-user resource counts grew with patch x resources.
-- The partial index holds only deleted works, which is almost always none.
--
-- Index only; no rows are touched.

CREATE INDEX IF NOT EXISTS idx_patch_catalog_deleted
    ON patch (id) WHERE catalog_deleted_at IS NOT NULL;
