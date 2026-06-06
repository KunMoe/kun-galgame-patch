-- Revert 017: drop the per-resource subscription table (CASCADE removes the FKs
-- and indexes with it).
DROP TABLE IF EXISTS public.user_patch_resource_favorite_relation;
