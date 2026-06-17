-- Revert 020: drop the user_id ranking indexes.
DROP INDEX IF EXISTS public.idx_patch_user_id;
DROP INDEX IF EXISTS public.idx_patch_resource_user_id;
DROP INDEX IF EXISTS public.idx_patch_comment_user_id;
