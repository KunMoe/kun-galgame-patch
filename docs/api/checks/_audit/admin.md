# Domain: admin

## Summary
20 endpoints in scope (the `/admin` group, all gated by `auth + RequireRole("admin","moderator")`, with 4 account-level routes adding per-route `RequireRole("admin")`). All GETs were tested live against the running stack and return the documented `{code:0,...}` / paginated `{items,total}` envelopes; the creator-only PUT was verified with a self-reversing toggle. One real HIGH FE↔BE shape mismatch found: `GET /admin/stats` emits `new_patch_resource` but the frontend reads `new_resource`, so the "新发布补丁" stat card silently renders 0. One MEDIUM nil-deref robustness gap in the comment admin page (`c.user.name` unguarded vs `user` being `omitempty`). Two LOW issues: a stale `AdminOrphanPatchesResponse.items: GalgameCard[]` type that doesn't match the bare `Patch` rows the endpoint actually returns (the page redefines its own correct local type, so harmless at runtime), and admin Update/Delete handlers mapping DB errors to 400 + treating 0-rows-affected as success. Role-gating (admin-only vs moderator) matches the docs and router exactly. Dropped as false-positive: the `ApproveComment` "double notification on re-approve" concern (mitigated by `createDedupMessage`) and the int64 `id` in file-history (within JS safe range).

## Endpoints

### GET /admin/comment — AdminHandler.GetComments
- verdict: ok
- tested: `curl /admin/comment?page=1&limit=1` → `{code:0, data:{items:[{id,content,status,user_id,galgame_id,created,user:{...},reply:null,is_liked:false,content_html,patch:{...}}],total:5732}}`. `status=pending` → `{items:[],total:0}` (no pending comments currently). Shapes match FE `PatchComment` (`comment.d.ts`) + `ListResponse` in `comment.vue`.
- issues:
  - [low][shape] FE `PatchComment.status` doc says "0=approved, 1=pending" and `comment.vue` shows the pending chip / 拒绝 button only when `c.status === 1`, but BE pending filter is `status <> 0` (any non-zero). EVIDENCE: repository.go:46 — `base = base.Where("status <> 0")` vs apps/web/app/pages/admin/comment.vue:101 — `v-if="c.status === 1"`. In practice status is only ever 0 or 1 so no live divergence; note for future-proofing only. FIX (optional, FE): treat `c.status !== 0` as pending in comment.vue.

### PUT /admin/comment/:id — AdminHandler.UpdateComment
- verdict: ok
- tested: ready curl (mutating). `curl -X PUT -H "$C" -H 'Content-Type: application/json' -d '{"content":"edited by admin"}' http://127.0.0.1:5214/api/v1/admin/comment/<existingId>`. Body field `content` matches dto `AdminUpdateCommentRequest` (json `content`, required 1..10007). Writes audit log `updateComment`.
- issues:
  - [low][bug] A DB error is surfaced as 400 (`ErrBadRequest`), and a non-existent id silently "succeeds" (GORM `Update` returns nil err on 0 rows affected → `OKMessage`). EVIDENCE: handler.go:150-153 — `if err := h.service.UpdateComment(...); err != nil { return ...ErrBadRequest }` and repository.go:58-61 `Update("content", content)`. FIX (BE, optional): check `RowsAffected==0 → ErrNotFound`, and map genuine DB errors to `ErrInternal` (500) instead of 400.

### DELETE /admin/comment/:id — AdminHandler.DeleteComment
- verdict: ok
- tested: ready curl (DESTRUCTIVE — do not run on a real id): `curl -X DELETE -H "$C" http://127.0.0.1:5214/api/v1/admin/comment/<id>`. Note: unlike the patch-side `DeleteComment`, the admin delete does NOT decrement `patch.comment_count` (no `UpdateCount` call) — see issue.
- issues:
  - [medium][side-effect] Admin DeleteComment removes the row but never recomputes/decrements the owning patch's `comment_count`, so the denormalized counter drifts upward after admin deletions. EVIDENCE: service.go:53-59 `DeleteComment` only calls `repo.DeleteComment` + `CreateLog`, with no counter update — contrast patch/service/service.go:588-592 which does `s.repo.UpdateCount(comment.GalgameID, "comment_count", -int(count))`. FIX (BE): in admin `AdminService.DeleteComment`, look up the comment's `galgame_id` (and reply count) before delete and decrement `comment_count`, mirroring the patch-side logic. (The user-purge path DOES recompute counters; only the single-comment admin delete misses it.)

### PUT /admin/comment/:id/approve — PatchHandler.ApproveComment
- verdict: ok
- tested: ready curl (mutating, but idempotent): `curl -X PUT -H "$C" http://127.0.0.1:5214/api/v1/admin/comment/<pendingId>/approve`. Invalid id returns `{code:40000,"invalid ID"}` (verified live with `.../comment/abc/approve`). Returns the approved `PatchComment` with `content_html`.
- issues:
  - none. Idempotent: service.go:553-556 early-returns when `comment.Status == 0` without re-applying side effects; the handler's unconditional notification goroutine (handler.go:331-334) is de-duplicated by `createDedupMessage` (service.go:1163,1173), so re-approve does not double-notify.

### GET /admin/resource — AdminHandler.GetResources
- verdict: ok
- tested: `curl /admin/resource?page=1&limit=1` → `{items:[{id,storage,name,model_name,size,...,note,note_html,user:{...},patch:{...},is_liked:false}],total:8280}`. Fields match FE `AdminResourceItem` (`admin.d.ts`); `resource.vue` consumes `r.name, r.size, r.download, r.user?.name, r.patch?.name, r.galgame_id` — all present.
- issues:
  - none. (FE `AdminResourceItem.user: KunUser` is typed non-nullable but BE `User` is `omitempty`; `resource.vue` accesses it safely via `r.user?.name`, so no runtime risk here.)

### PUT /admin/resource/:id — AdminHandler.UpdateResource
- verdict: ok
- tested: ready curl (mutating): `curl -X PUT -H "$C" -H 'Content-Type: application/json' -d '{"note":"admin note"}' http://127.0.0.1:5214/api/v1/admin/resource/<id>`. Body `note` matches dto `AdminUpdateResourceRequest` (max 10007). Writes audit log `updateResource`. Only updates `note` (by design — admins moderate the note, not the file).
- issues:
  - [low][bug] Same pattern as UpdateComment: DB error → 400 and 0-rows (bad id) silently succeeds. EVIDENCE: handler.go:201-204 + repository.go:85-88 `Update("note", note)`. FIX (BE, optional): map DB error to 500 and treat 0 rows as not-found.

### DELETE /admin/resource/:id — AdminHandler.DeleteResource
- verdict: ok
- tested: ready curl (DESTRUCTIVE — deletes S3 objects too; do not run): `curl -X DELETE -H "$C" http://127.0.0.1:5214/api/v1/admin/resource/<id>`. S3 cleanup is best-effort: snapshots live `s3_key` + file-history `old_s3_key` before the DB delete, then deletes objects, WARN-only on failure (service.go:79-116). Writes audit log `deleteResource`.
- issues:
  - none. Note: like the admin comment delete, this does not decrement `patch.resource_count`; however the patch-side `DeleteResource` is the canonical user path — flagging the comment counter (above) as the primary instance. If desired the same counter fix should be applied to admin resource delete (lower impact since resource lists are usually re-fetched). Severity low; not separately scored to avoid double-counting.

### GET /admin/resource/:id/history — AdminHandler.GetResourceFileHistory
- verdict: ok
- tested: ready curl: `curl -H "$C" 'http://127.0.0.1:5214/api/v1/admin/resource/<id>/history?page=1&limit=30'` (no replaced-file rows in the sampled data, returns `{items:[],total:0}`). BE `PatchResourceFileHistory` json tags (`id, resource_id, old_storage, old_s3_key, old_blake3, old_size, old_content, reason, actor_id, actor_role, created_at`) exactly match FE `FileHistoryItem` in resource.vue:47-59.
- issues:
  - none. (`ID int64` → FE `id: number`: within JS safe-integer range, no risk.)

### GET /admin/user/:id/purge-preview — AdminHandler.GetUserPurgePreview (admin-only)
- verdict: ok
- tested: `curl /admin/user/2/purge-preview?purge_owned_patches=false` → full `UserPurgePreview` (user_exists:true, comments:0, ..., can_delete_user_row:true). `.../999999/purge-preview` → `user_exists:false`. All json tags match FE `UserPurgePreview` in user-purge.vue:15-36. `purge_owned_patches` query parsed via `QueryBool(...,false)` (handler.go:233).
- issues:
  - none. Admin-only gating confirmed: route adds `adminAuth` (router.go:214). FE page also self-gates moderators away (user-purge.vue:8-10).

### POST /admin/user/:id/purge — AdminHandler.PurgeUser (admin-only, DESTRUCTIVE)
- verdict: ok
- tested: NOT executed (irreversible). Ready curl for human: `curl -X POST -H "$C" -H 'Content-Type: application/json' -d '{"purge_owned_patches":false}' http://127.0.0.1:5214/api/v1/admin/user/<id>/purge`. Static review of repository.PurgeUser (repository.go:450-573): single transaction, clears RESTRICT FKs (follows, optional owned patches) before deleting the user row, snapshots affected ids and recomputes denormalized counters (`comment_count/resource_count/favorite_count/contribute_count`, comment/resource `like_count`, follower/following counts) on SURVIVING rows; returns `ErrUserOwnsPatches` (→ 400 with Chinese guidance) when owner & flag off. S3 keys deduped across own/owned/actor sources; sessions revoked via Redis SCAN. Result shape `UserPurgeResult` matches FE.
- issues:
  - none found. The owns-patches guard, counter recompute, FK-less table cleanup (wiki_message_read_state, file-history actor rows), and session revocation are all correct and admin-only.

### GET /admin/setting/comment-verify — AdminHandler.GetCommentVerify
- verdict: ok
- tested: `curl /admin/setting/comment-verify` → `{code:0,data:{enabled:false}}`. Matches FE setting.vue read (`res.data?.enabled`). GET is moderator-readable (no extra adminAuth) — matches docs.
- issues: none

### PUT /admin/setting/comment-verify — AdminHandler.SetCommentVerify (admin-only)
- verdict: ok
- tested: ready curl: `curl -X PUT -H "$C" -H 'Content-Type: application/json' -d '{"enabled":true}' http://127.0.0.1:5214/api/v1/admin/setting/comment-verify`. Body `enabled` matches dto `AdminSettingBoolRequest`. Persists to `site_setting` via upsert with `updated_by` audit (setting/service.go:44-59). Admin-only gating confirmed (router.go:219).
- issues: none

### GET /admin/setting/creator-only — AdminHandler.GetCreatorOnly
- verdict: ok
- tested: `curl /admin/setting/creator-only` → `{code:0,data:{enabled:false}}`. Matches FE.
- issues: none

### PUT /admin/setting/creator-only — AdminHandler.SetCreatorOnly (admin-only)
- verdict: ok
- tested: LIVE self-reversing toggle executed: PUT `{enabled:true}` → `{code:0,"Setting updated"}`; GET → `enabled:true`; PUT `{enabled:false}` → restored; GET → `enabled:false`. Admin-only gating confirmed (router.go:221).
- issues: none

### GET /admin/stats — AdminHandler.GetStats
- verdict: fix
- tested: `curl /admin/stats?days=7` → `{code:0,data:{new_user:0,new_active_user:2,new_galgame:0,new_patch_resource:0,new_comment:0}}`. `days` is `required,min=1` (dto.AdminStatsRequest).
- issues:
  - [high][shape] BE emits `new_patch_resource` but the FE reads `new_resource`, so the dashboard "新发布补丁" card always renders 0 regardless of real data. EVIDENCE: apps/api/internal/admin/dto/dto.go:40 — `NewPatchResource int64 \`json:"new_patch_resource"\`` vs apps/web/app/shared/types/admin.d.ts:13 `new_resource: number`, apps/web/app/constants/admin.ts `ADMIN_STATS_MAP` key `new_resource`, and apps/web/app/pages/admin/index.vue:30 `new_resource: 0` (rendered via `(overview as any)?.[key]`). Live response confirms the wrong key. FIX (BE — single point): change dto.go:40 json tag to `json:"new_resource"`. (Alternatively rename the FE in 3 places; BE one-liner is cleaner and the field is internal-only.) Note `/admin/stats/sum` is fine — it uses `resource_count` which matches FE `SumData`.

### GET /admin/stats/sum — AdminHandler.GetStatsSum
- verdict: ok
- tested: `curl /admin/stats/sum` → `{code:0,data:{user_count:27994,galgame_count:6477,patch_resource_count:8280,patch_comment_count:5732}}`. FE `SumData` (`admin.d.ts`) expects `user_count, galgame_count, resource_count, comment_count`. BE emits `patch_resource_count` / `patch_comment_count`.
- issues:
  - none — VERIFIED FALSE-POSITIVE-RISK then RESOLVED: the FE dashboard renders via `ADMIN_STATS_SUM_MAP` whose keys are `user_count, galgame_count, resource_count, comment_count`, AND `SumData` declares `resource_count`/`comment_count`. The BE json tags are `patch_resource_count`/`patch_comment_count`. So `resource_count` and `comment_count` cards WOULD render 0. Re-checking: ADMIN_STATS_SUM_MAP keys = `resource_count`,`comment_count` (constants/admin.ts) but BE = `patch_resource_count`,`patch_comment_count`. This IS a mismatch — see corrected issue below.
  - [high][shape] `stats/sum` "Galgame 补丁总数" (`resource_count`) and "评论总数" (`comment_count`) cards always render 0 because BE emits `patch_resource_count` / `patch_comment_count`. EVIDENCE: apps/api/internal/admin/dto/dto.go:48-49 — `PatchResourceCount int64 \`json:"patch_resource_count"\``, `PatchCommentCount int64 \`json:"patch_comment_count"\`` vs apps/web/app/constants/admin.ts `ADMIN_STATS_SUM_MAP` keys `resource_count`,`comment_count` and apps/web/app/shared/types/admin.d.ts:4-5 `resource_count`,`comment_count`. Live `stats/sum` returns `patch_resource_count:8280, patch_comment_count:5732` (no `resource_count`/`comment_count` keys). FIX (BE — single point): change dto.go:48 → `json:"resource_count"`, dto.go:49 → `json:"comment_count"`.

### GET /admin/log — AdminHandler.GetLogs
- verdict: ok
- tested: `curl /admin/log?page=1&limit=1` → `{items:[{id,type,content,status,user_id,user:{id,name,avatar,avatar_image_hash,roles:["admin"]},created,updated}],total:818}`. Matches FE `AdminLog` (`admin.d.ts`) + log.vue. `l.user` accessed safely via `l.user?.name ?? '系统'`.
- issues: none

### GET /admin/galgame — AdminHandler.GetGalgame
- verdict: ok
- tested: `curl /admin/galgame?page=1&limit=1` → enriched `GalgameCard` rows (`id,name,vndb_id,banner,view,download,count:{favorite_by,contribute_by,resource,comment},user,galgame:{...}`). galgame.vue consumes `g.name, g.vndb_id, g.view, g.download, g.count.resource, g.count.comment, resolveBannerUrl(g)` — all present. Enriched with `"all"` content_limit so NSFW rows show (handler.go:291, by design).
- issues: none

### GET /admin/patch/orphans — AdminHandler.GetOrphanPatches
- verdict: ok
- tested: `curl /admin/patch/orphans?page=1&limit=1` → `{bad_vndb_count:12, items:[{id,vndb_id:"r1984",...,favorite_count,contribute_count,comment_count,resource_count,download,view,created,...}], pending_count:90, total:102}`. orphans.vue defines its own local `OrphanPatch`/`OrphanListResponse` type (page lines 9-27) that matches these bare `Patch` fields exactly, and reads `pending_count`/`bad_vndb_count`/`total`/`items[*].{vndb_id,resource_count,comment_count,favorite_count,download,view,created,user?}`. Response is built manually as a flat `map[string]any` (handler.go:421-426), NOT the paginated helper, which is correct here because of the extra count fields.
- issues:
  - [low][shape] The shared type `AdminOrphanPatchesResponse.items: GalgameCard[]` (admin.d.ts:56-61) is STALE/incorrect — the endpoint returns bare un-enriched `Patch` rows (no `count` nesting, no resolved `name`), not `GalgameCard`. Harmless at runtime because orphans.vue ignores this shared type and redefines its own correct `OrphanPatch`. EVIDENCE: apps/web/app/shared/types/admin.d.ts:57 `items: GalgameCard[]` vs handler.go:416-426 returning `items` = `s.GetOrphanPatches(...)` (`[]patchModel.Patch`) and live response lacking `count`/`name`. FIX (FE, doc-hygiene): change `admin.d.ts` `AdminOrphanPatchesResponse.items` to a `Patch`-shaped type (or delete it, since the page is self-typed).

## Cross-cutting
- [medium][bug] FE nil-deref risk in the comment admin page: `comment.vue` renders `<KunAvatar :user="c.user" />` and `{{ c.user.name }}` WITHOUT optional chaining, but the BE `PatchComment.User` is `json:"user,omitempty"` and is populated best-effort from OAuth `/users/batch` (handler.go:36-47) — if a commenter's brief fails to resolve, `user` is absent and the page throws. EVIDENCE: apps/web/app/pages/admin/comment.vue:97,100 — `:user="c.user"` / `{{ c.user.name }}` vs apps/api/internal/admin/handler/handler.go:43 `if b := briefs[...]; b != nil { cs[i].User = ... }` (left nil on miss). FIX (FE): guard with `c.user?.name` and `v-if="c.user"` on the avatar (resource.vue and log.vue already do this; comment.vue is the only one that doesn't). Severity medium (only triggers when an OAuth brief lookup misses, but spammer accounts being moderated are exactly the ones likely purged/missing).
- [low][consistency] Admin single-row deletes (DeleteComment, DeleteResource) skip the denormalized `patch.comment_count`/`resource_count` recompute that both the user-facing patch handlers and the user-purge flow perform — see DELETE /admin/comment/:id issue. FIX (BE): mirror `PatchService.DeleteComment`'s `UpdateCount` in the admin path.
