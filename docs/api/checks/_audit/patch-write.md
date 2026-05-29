# Domain: patch-write

## Summary
15 mutating patch/comment/resource endpoints audited (handler -> service -> repo -> sql/model and the FE consumer in apps/web/app). One CRITICAL end-to-end break: `POST /patch/:id/comment` can never succeed because the DTO marks `galgame_id` as `required` and validation runs BEFORE the handler injects it from the path param, while the FE only sends `{ content }` — confirmed live (returns `40000 "GalgameID is required"`). All like/favorite/disable toggles, view/download counters, and create-patch were tested live (self-reversing toggles reverted cleanly; no data left behind). One LOW structural note on nested-reply comment_count accounting. Dropped two would-be false positives: the legacy-s3-key prefix check (verified consistent with the upload key builder) and the "UpdateResource requires galgame_id" concern (the FE edit payload does send it). UpdateComment / DeleteComment have no FE caller (no edit/delete UI on the comment page) but the BE endpoints are correct — marked ok with ready curls.

## Endpoints

### POST /patch/ — CreatePatch
- verdict: ok
- tested: live. `{"vndb_id":"v823"}` (existing) -> `40000 "该 VNDB ID 已经存在对应的补丁"` (dedup, no row). `{"vndb_id":"xyz"}` -> `40000 "vndb_id 格式不合法（应为 vXXX）"`. FE pages/edit/create.vue:92 sends `{vndb_id}`, expects `{id}` and code 44001 CTA; BE returns `response.OK(c, map[string]int{"id": id})` (handler.go:119) and `ErrWikiGalgameNotFound`=44001 for the missing case (handler.go:115). Shapes match.
- side-effects verified: +3 moemoepoint awarded once post-commit with idempotency key `moyu:patch_create:<id>` (service.go:132), contributor relation + contribute_count++ inside the txn (service.go:116-124). Publish gate `ensureCanPublishGalgame` applied (handler.go:96). Correct.

### PUT /patch/:id — UpdatePatch
- verdict: ok
- tested: live. `PUT /patch/9077 {"vndb_id":"v823"}` -> `{code:0,"Patch updated"}` (rebinding to the same galgame_id is the allowed path; service.go:310 rejects a different galgame_id). Owner/privileged gate enforced in service (service.go:299). FE has no caller (vndb-rebind is an admin/edge operation) — BE-only, correct.
- issues: none

### DELETE /patch/:id — DeletePatch
- verdict: ok (not executed — destructive)
- tested: ready curl for human (deletes a patch + CASCADE children + S3 objects):
  `curl -X DELETE 'http://127.0.0.1:5214/api/v1/patch/<id>' -H 'Cookie: moyu_session=<admin>'`
  Owner-or-admin gate (handler.go:220, service.go:323). S3 cleanup drains live `patch_resource.s3_key` + `patch_resource_file_history.old_s3_key` BEFORE the DB delete (service.go:343-352) since FK CASCADE would otherwise strand the B2 objects — verified the two key sets are disjoint by construction and S3 failures only WARN. Correct.
- issues: none

### PUT /patch/:id/view — IncrementView
- verdict: ok
- tested: live. `PUT /patch/6690/view` -> `{code:0}`; `PUT /patch/99999999/view` (nonexistent) -> `{code:0}` (UPDATE matches 0 rows, no error). Route has no auth (intentional public counter, moyu.put.md). FE pages/patch/[id].vue:84 fires `.catch(()=>{})`. Harmless on missing id.
- issues: none

### PUT /patch/:id/favorite — ToggleFavorite
- verdict: ok
- tested: live self-reversing. ON -> `{favorited:true}`, OFF -> `{favorited:false}`. FE components/patch/header/Actions.vue:69 + pages/resource/[id].vue:145 expect `{favorited:boolean}`. Matches. favorite_count +/-1, owner +/-1 moemoepoint with per-relation idempotency keys `moyu:favorite:<relID>` / `moyu:unfavorite:<relID>` and `patch.UserID != userID` self-guard (service.go:1080-1093). Correct.
- issues: none

### POST /patch/:id/comment — CreateComment
- verdict: fix
- tested: live, BROKEN. Exact FE body `{"content":"audit probe"}` (no galgame_id) -> `{"code":40000,"message":"GalgameID is required","data":null}`. Empty body -> `"GalgameID is required; Content is required"`. No comment is created (rejected at validation).
- issues:
  - [critical][shape-mismatch] Comment creation is completely non-functional from the FE. `PatchCommentCreateRequest.GalgameID` is `validate:"required,min=1"` (dto.go:26) and `utils.ParseAndValidate` validates the struct (validate.go:13-18) BEFORE the handler sets `req.GalgameID = patchID` (handler.go:294). The FE sends only `{ content }` (pages/patch/[id]/comment.vue:54), so every create fails with "GalgameID is required". EVIDENCE: internal/patch/dto/dto.go:26 — `GalgameID int \`json:"galgame_id" validate:"required,min=1"\``; internal/patch/handler/handler.go:291-294 — `ParseAndValidate(...)` then `req.GalgameID = patchID`; apps/web/app/pages/patch/[id]/comment.vue:52-55 — `api.post<PatchPageComment>(\`/patch/${galgameId.value}/comment\`, { content: content.value })`. FIX (BE): drop `required,min=1` from `PatchCommentCreateRequest.GalgameID` (make it `validate:"omitempty,min=1"` or remove the tag) since the canonical source is the URL path param, which the handler already injects. Do NOT "fix" by making the FE send galgame_id — the path param is authoritative and the body field is redundant. After the change, also verify the pending-comment branch: handler.go:306 keys notifications on `comment.Status == 0`, which is correct once creation works.

### PUT /patch/comment/:commentId — UpdateComment
- verdict: ok (no FE caller)
- tested: BE-only ready curl (edits own comment):
  `curl -X PUT 'http://127.0.0.1:5214/api/v1/patch/comment/<id>' -H 'Cookie: moyu_session=<owner>' -H 'Content-Type: application/json' -d '{"content":"new text"}'`
  Owner-only gate in service (service.go:571 `comment.UserID != userID -> error`); no privilege bypass here (edit is strictly own-content, by design). DTO `PatchCommentUpdateRequest{Content}` (dto.go:33) matches the curl body. The moyu FE currently exposes no comment-edit UI, so the endpoint is unused but correct.
- issues: none

### DELETE /patch/comment/:commentId — DeleteComment
- verdict: ok (no FE caller)
- tested: BE-only ready curl (destructive — not executed):
  `curl -X DELETE 'http://127.0.0.1:5214/api/v1/patch/comment/<id>' -H 'Cookie: moyu_session=<owner-or-mod>'`
  Owner-or-privileged gate (handler.go:366, service.go:584). comment_count decremented by `CountCommentAndReplies` which counts only status=0 rows (`(id=? OR parent_id=?) AND status=0`, repository.go:208-211), so deleting a pending (status=1) comment subtracts 0 — matches the deferred-increment model. DB FK `patch_comment_parent_id_fkey ... ON DELETE CASCADE` (migrations/000_baseline.up.sql:1361) cascades replies, kept symmetric with the count. No moemoepoint reversal on comment delete (the owner's +1 from applyCommentSideEffects is not reversed) — pre-existing/by-design (comment delete is not in the documented reversal set; only resource delete reverses -3).
- issues: none

### PUT /patch/comment/:commentId/like — ToggleCommentLike
- verdict: ok
- tested: live self-reversing on comment 26767 (owner 81984 != admin id 2). ON -> `{liked:true}`, OFF -> `{liked:false}`. FE pages/patch/[id]/comment.vue:88 expects `{liked:boolean}`. Matches. like_count +1 / `GREATEST(like_count-1,0)` (service.go:607,619); owner +/-1 moemoepoint with self-guard and per-relation idempotency keys `moyu:comment_like:<relID>` / `moyu:comment_unlike:<relID>` (service.go:609-623). Correct.
- issues: none

### POST /patch/:id/resource — CreateResource
- verdict: ok (not executed for real — would create data; statically verified + shape-checked)
- tested: ready curl for human (storage="user" avoids needing a real upload):
  `curl -X POST 'http://127.0.0.1:5214/api/v1/patch/<id>/resource' -H 'Cookie: moyu_session=<admin>' -H 'Content-Type: application/json' -d '{"galgame_id":<id>,"storage":"user","name":"t","size":"1MB","content":"https://example.com/x","type":["全语言"],"language":["简体中文"],"platform":["windows"]}'`
  FE components/resource/Publish.vue:323 sends `basePayload` incl. `galgame_id` (so the embedded DTO's required galgame_id passes — verified), `s3_key`/`content`/`size`/`type`/`language`/`platform`. s3 branch enforces `s3_key` prefix `patch/<galgame_id>/` and sets Content=S3Key (service.go:706-716); user branch requires non-empty Content (service.go:721). Side-effects: resource_count++, RecalculatePatchAggregates, resource_update_time=now, +3 moemoepoint (`moyu:resource_publish:<resID>`), EnsureContributor, notifyFavoritedUsers (deduped) (service.go:739-754). Response is the full row with note_html + user brief (service.go:757-768) matching FE `PatchResource`. Correct.
- issues: none

### PUT /patch/resource/:resourceId — UpdateResource
- verdict: ok
- tested: not executed for real (mutates a live row's metadata); statically verified + shape-checked. FE Publish.vue:311 sends `{...basePayload, reason}` incl. `galgame_id`/`storage`/`size`/`type`/`language`/`platform`, so the embedded `PatchResourceCreateRequest` required fields all pass. Owner-or-mod bypass via actorRole (service.go:794: `existing.UserID != userID && actorRole < 2`). File-history row written only on file-substantive change (storage/s3_key/content diff, service.go:841-843), inside the same txn. Orphan S3 key deleted post-commit best-effort (service.go:850-908). Legacy-key prefix check only runs when s3_key actually changes (service.go:823) — verified the upload key builder uses the current galgame_id (internal/common/upload/service.go:222 `buildPatchResourceKey(req.GalgameID, ...)`) so a real file replace produces `patch/<galgame_id>/...` and passes. Returns full rendered row matching FE. Correct.
  Ready curl (metadata-only edit of resource 8852, low risk — adjust fields):
  `curl -X PUT 'http://127.0.0.1:5214/api/v1/patch/resource/8852' -H 'Cookie: moyu_session=<admin>' -H 'Content-Type: application/json' -d '{"galgame_id":4893,"storage":"s3","s3_key":"<existing key>","size":"...","name":"...","note":"...","type":["全语言"],"language":["简体中文"],"platform":["windows"],"reason":"audit"}'`
- issues: none

### DELETE /patch/resource/:resourceId — DeleteResource
- verdict: ok (not executed — destructive)
- tested: ready curl for human:
  `curl -X DELETE 'http://127.0.0.1:5214/api/v1/patch/resource/<id>' -H 'Cookie: moyu_session=<owner-or-mod>'`
  Owner-or-privileged gate (handler.go:541, service.go:937). History old_s3_keys snapshotted before DELETE (service.go:944) for S3 cleanup post-CASCADE. resource_count--, RecalculatePatchAggregates, and -3 moemoepoint charged to the resource OWNER (not the deleting mod) with `content_removed` reason + same ref as publish so OAuth reconciles (service.go:978-985). FE pages/patch/[id]/resource.vue:127. Correct.
- issues: none

### PUT /patch/resource/:resourceId/disable — ToggleResourceDisable
- verdict: ok
- tested: live self-reversing on resource 8852 (admin via isPrivileged). ON -> `{status:1}`, OFF -> `{status:0}`; DB verified status back to 0. FE pages/patch/[id]/resource.vue:62 expects `{status:number}`. Matches. Owner-or-privileged gate (handler.go:557, service.go:998). Status flipped atomically via SQL CASE (repository.go:294-296); returned value is the inverse of the pre-read status (service.go:1006-1009) — consistent with the atomic flip. Correct.
- issues: none

### PUT /patch/resource/:resourceId/download — IncrementResourceDownload
- verdict: ok
- tested: live. `PUT /patch/resource/99999999/download` -> `40000 "resource not found"` (service.go:1013 reads the row first). Route has no auth (public counter, moyu.put.md). Increments BOTH resource.download and patch.download in one txn (repository.go:283-291) — no double-count, one increment each. FE pages/patch/[id]/resource.vue:225 fires `.catch(()=>{})`.
- issues: none

### PUT /patch/resource/:resourceId/like — ToggleResourceLike
- verdict: ok
- tested: live self-reversing on resource 8852 (owner 16315 != admin). ON -> `{liked:true}`, OFF -> `{liked:false}`. FE pages/patch/[id]/resource.vue:236 expects `{liked:boolean}`. Matches. like_count +1 / `GREATEST(like_count-1,0)`, owner +/-1 moemoepoint with self-guard + per-relation idempotency keys `moyu:resource_like:<relID>` / `moyu:resource_unlike:<relID>` (service.go:1047-1063). Correct.
- issues: none

## Cross-cutting
- [low][bug] comment_count can drift for nested replies (reply-of-a-reply). `applyCommentSideEffects` bumps comment_count by +1 per approved comment regardless of depth (service.go:536), but `CountCommentAndReplies` on delete only counts `id=? OR parent_id=?` i.e. the target + its DIRECT children (repository.go:210). Deleting a top-level comment whose reply itself has a reply (parent_id = the reply's id) would CASCADE-delete the grandchild in the DB but only subtract the parent + direct children from comment_count, leaving the count too high by the grandchild count. In practice the moyu FE renders/creates only one reply level (no `parent_id` is ever sent by the FE — replies originate from migrated data), so this is latent. EVIDENCE: internal/patch/service/service.go:536 `s.repo.UpdateCount(patchID,"comment_count",1)` vs internal/patch/repository/repository.go:208-211 `Where("(id = ? OR parent_id = ?) AND status = 0", commentID, commentID)`. FIX (BE, optional/low): if multi-level replies become reachable, recurse the count or recompute comment_count from a COUNT(*) over status=0 rows on delete.
- [info] No reply composer in the FE: `parent_id` is never sent by any apps/web caller (grep found zero), and there is no comment edit/delete UI. The CreateComment `parent_id` handling, UpdateComment, and DeleteComment endpoints are therefore exercised only by migrated data / future UI. Not a defect; flagged so the comment-create CRITICAL fix is understood to currently block ALL commenting (top-level included).
