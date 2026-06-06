-- 017: Per-resource subscription ("收藏资源").
--
-- A user can favorite a SINGLE patch resource to subscribe to its updates: when
-- that resource's download link or file changes, PatchService.UpdateResource
-- sends each subscriber a `patchResourceUpdate` notification. Metadata-only
-- edits (note / name / type / …) do NOT notify — only a file-substantive change
-- (storage / s3_key / content) does.
--
-- Distinct from the two existing relations:
--   - user_patch_resource_like_relation : 点赞 a resource (appreciation, no notify)
--   - user_patch_favorite_relation      : 收藏 the GALGAME (notified on NEW resources)
--
-- FKs CASCADE so deleting a user or a resource removes its subscriptions. The
-- user(id) anchor is safe: moyu provisions every local user row at the OAuth
-- callback (FindOrCreateUserByID), so post-cutover ids resolve.

CREATE TABLE IF NOT EXISTS public.user_patch_resource_favorite_relation (
    id          SERIAL PRIMARY KEY,
    user_id     integer NOT NULL,
    resource_id integer NOT NULL,
    created     timestamp(3) without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated     timestamp(3) without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT user_patch_resource_favorite_relation_user_id_fkey
        FOREIGN KEY (user_id) REFERENCES public."user"(id) ON DELETE CASCADE,
    CONSTRAINT user_patch_resource_favorite_relation_resource_id_fkey
        FOREIGN KEY (resource_id) REFERENCES public.patch_resource(id) ON DELETE CASCADE
);

-- One subscription per (user, resource); also the lookup index for IsResourceFavorited.
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_resource_favorite
    ON public.user_patch_resource_favorite_relation (user_id, resource_id);

-- Reverse lookup: "who is subscribed to this resource" (notifyResourceFavoritedUsers).
CREATE INDEX IF NOT EXISTS idx_uprfr_resource
    ON public.user_patch_resource_favorite_relation (resource_id);
