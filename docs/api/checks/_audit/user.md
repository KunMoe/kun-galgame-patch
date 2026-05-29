# Domain: user

## Summary
16 user endpoints audited (3 mutating self-reversing-tested live, 9 GETs tested live, 2 file/legacy untestable). FE↔BE shapes match the `.d.ts` contracts everywhere (UserInfoResponse, paginated `{items,total}`, moemoepoint log `{items,has_more}`, check-in `{moemoepoint}`, search `[]UserBasic`). One HIGH data-integrity bug found and PROVEN live: `DELETE /user/:id/follow` decrements the target's `follower_count` even when no follow relation exists (no rows-affected guard), letting any user corrupt anyone's follower count. Two MEDIUM issues: `PUT /user/:id/follow` leaks the raw Postgres FK-violation string when the followee has no local user row; and the `validate:"min=1"` on the shared list DTO makes the handlers' `if page==0` defaulting dead code (omitting `page`/`limit` returns 40000 — FE always sends them, so low real impact). Dropped as false-positive: the resource list exposing `content`/`s3_key`/`password` — verified the canonical `GET /patch/:id/resource` returns the identical fields, so this is a pre-existing app-wide pattern owned by patch/common, not a user-domain defect. `/user/search` and `/user/image` have no current FE consumer (search is auth-gated and shape-correct; image is a legacy endpoint superseded by `/upload/image-service`).

## Endpoints

### GET /user/search — UserHandler.SearchUsers
- verdict: ok
- tested: `curl /user/search?query=鲲` (admin cookie) → `{"code":0,"data":[{"id":2,"name":"鲲","avatar":"..."},...]}`. No-auth → 40100 (auth middleware present, line 160). `?keyword=x` → `{"code":40000,"message":"Query is required"}` (DTO field is `query` not `keyword`, validated `required,min=1,max=20`). Limit hard-capped at 50 server-side (service line 211). No FE consumer found in apps/web/app (grep for `/user/search` empty) — endpoint exists but is currently unused.
- issues: none

### GET /user/moemoepoint/log — UserHandler.GetMoemoepointLog
- verdict: ok
- tested: `curl /user/moemoepoint/log?limit=3` (admin) → `{"code":0,"data":{"has_more":false,"items":[{"id":74315,"delta":8134,"reason":"migration","source_app":"oauth","ref":"","created_at":"2026-05-29T..."}]}}`. No-auth → 40100. Shape matches FE `MoemoepointLogEntry` (id/delta/reason/source_app/ref/created_at) and `{items, has_more}` exactly (MoemoepointLog.vue). user id is taken from session (`middleware.MustGetUser`, handler line 320), never a path param — no IDOR. Registered BEFORE `/:id` (router line 163 vs 166) so Fiber won't match "moemoepoint" as `:id`.
- issues: none

### GET /user/:id — UserHandler.GetUserInfo
- verdict: ok
- tested: `curl /user/2` → all 14 fields present and matching FE `UserInfo` (id,name,avatar,bio,roles,moemoepoint,follower_count,following_count,register_time,patch_count,resource_count,comment_count,favorite_count,is_followed). `register_time` is RFC3339 (`2026-05-11T22:11:51Z`), consumed by FE `formatDate` — fine. `optionalAuth` (router 166) lets `is_followed` reflect the viewer; anonymous → false. OAuth lookup failure degrades gracefully (name/avatar/bio empty) per service lines 114-121.
- issues: none

### GET /user/:id/floating — UserHandler.GetUserFloating
- verdict: ok
- tested: `curl /user/2/floating` → identical `UserInfoResponse` to `/user/:id` but with `is_followed:false` always (service `GetUserFloating` passes currentUID=0, line 133). No auth middleware (router 167) — public, intentional for the floating hover card. No FE consumer found (grep `floating` in app/ empty) — endpoint unused but harmless.
- issues: none

### GET /user/:id/patch — UserHandler.GetUserPatches
- verdict: fix
- tested: `curl /user/2310/patch?page=1&limit=2` → `{"code":0,"data":{"items":[],"total":0}}`. With params present works. Omitting params: `curl /user/2310/patch` → `{"code":40000,"message":"Page length must not be less than 1; Limit length must not be less than 1"}`. Returns enriched `GalgameCard` items; FE `galgame.vue`/`info.vue` consume `{items: GalgameCard[], total}` — matches.
- issues:
  - [low][bug] The handler's `if req.Page == 0 { req.Page = 1 }` / `if req.Limit == 0 { req.Limit = 10 }` defaulting (handler lines 98-103) is DEAD CODE: the shared `GetUserProfileRequest` has `validate:"min=1"` on both fields (dto.go:7-8), so `ParseQueryAndValidate` rejects an omitted (0) value with 40000 BEFORE the defaulting runs. Confirmed live. FE always sends `page=1&limit=N`, so low real impact, but the endpoint is brittle for any future caller and the defaulting is misleading. FIX (BE): either drop the `min=1` validate tags from `GetUserProfileRequest` (dto/dto.go) so the handler defaults take effect, or remove the now-useless defaulting blocks. Same applies to the 5 sibling list endpoints below sharing this DTO.

### GET /user/:id/resource — UserHandler.GetUserResources
- verdict: ok
- tested: `curl /user/2310/resource?page=1&limit=1&content_limit=all` → 28-key PatchResource rows; FE `UserResourceItem` consumes id/galgame_id/size/type/language/platform/created/patch (resource.vue + info.vue, plus `r.name`). NSFW gating verified: without `content_limit` the list defaults to `sfw` (`ContentLimitForListBrowse`) and the wiki filter drops NSFW-owned rows (`FilterByGalgameContentLimit`). No auth middleware (router 169) — public list, consistent with `GET /patch/:id/resource`. `attachPatchSummaries` fills `patch` (id/vndb_id/banner/name) matching FE `PatchSummary`.
- issues:
  - [low][mismatch] FE `UserResourceItem` (user.d.ts) omits `name`, yet `info.vue:121` reads `r.name`. Works at runtime (extra BE field, TS just untyped here) but the type is incomplete. FIX (FE): add `name?: string` (and optionally note_html/user) to `UserResourceItem` for type accuracy. Non-blocking.

### GET /user/:id/favorite — UserHandler.GetUserFavorites
- verdict: ok
- tested: returns `enricher.EnrichPatches` → `{items: GalgameCard[], total}` (handler 159-163); FE `favorite.vue` consumes `GalgameCard` items — matches. NSFW filtered via `ContentLimitForListBrowse` (sfw default). Same `min=1` DTO defaulting dead-code as `/patch` (see that entry) — omitting params 40000; FE sends `page=1&limit=20`.
- issues: none (shape ok; shares the low-sev validate dead-code noted under `/patch`)

### GET /user/:id/comment — UserHandler.GetUserComments
- verdict: ok
- tested: `curl /user/16286/comment?page=1&limit=2` → `{items:[],total:346}` under default `sfw` (all 346 commented games are NSFW → filtered, by design). With `content_limit=all` → full rows: id/content/like_count/user_id/galgame_id/created/patch + user brief. FE `UserComment` (comment.vue/info.vue) consumes content/like_count/galgame_id/created/patch — matches. The `total` (346) is the unfiltered DB count while `items` is post-NSFW-filtered — a known consequence of the filter-after-paginate design (consistent with other list endpoints), not unique to user.
- issues: none

### GET /user/:id/contribute — UserHandler.GetUserContributions
- verdict: ok
- tested: returns `EnrichPatches` → `{items: GalgameCard[], total}`; FE `contribute.vue` consumes `c.id/c.name/c.created` from GalgameCard — matches. NSFW filtered (sfw default). Subquery on `user_patch_contribute_relation.galgame_id` (repo 108-118) is correct.
- issues: none

### GET /user/:id/follower — UserHandler.GetFollowers
- verdict: ok
- tested: `optionalAuth` (router 173) stamps per-row `is_followed` relative to viewer via `WhichFollowed` (one query/page, repo 178). Returns `{items: UserFollowItem[], total}` (id/name/avatar/is_followed) matching FE FollowListModal `FollowItem`. Default limit 20 (handler 266), within DTO max=20. Live-verified empty list for user 2.
- issues: none

### GET /user/:id/following — UserHandler.GetFollowing
- verdict: ok
- tested: `curl /user/2/following?page=1&limit=10` → `{"code":0,"data":{"items":[],"total":0}}`. Same `UserFollowItem` shape + per-row `is_followed`. `GetFollowingIDs` filters `follower_id = :id` (repo 162) — correct direction (users that :id follows).
- issues: none

### POST /user/image — UserHandler.UploadImage
- verdict: untestable
- tested: file upload — not executed. Ready curl for human:
  `curl -X POST 'http://127.0.0.1:5214/api/v1/user/image' -H 'Cookie: moyu_session=b3ee0dd5c51f4846f037cff3590e40b3b1c17b9968e04a32d1e41eb260fb3012' -F 'image=@/path/to/test.jpg'`
  Static review: reads form field `image` (handler 356, `readImageFormFile`), 10MB cap, refits to 1920x1080 JPEG q=50, S3 key `user_<id>/image/...`, increments `daily_image_count` (limit 20). Returns `{url: string}` (handler 364). No FE consumer found (editor uploads go through `/upload/image-service`, uploader.ts:22) — legacy endpoint. Auth-gated (router 158).
- issues: none

### POST /user/check-in — UserHandler.CheckIn
- verdict: ok
- tested: not re-run (user 2 already daily_check_in toggles state). Ready curl (self-reversing only via cron reset, so don't spam):
  `curl -X POST 'http://127.0.0.1:5214/api/v1/user/check-in' -H 'Cookie: moyu_session=b3ee0dd5c51f4846f037cff3590e40b3b1c17b9968e04a32d1e41eb260fb3012'`
  Static review: returns `{moemoepoint: int}` (handler 311) matching FE UserDropdown `{moemoepoint}`. Idempotent per day: `daily_check_in==1` → "already checked in today" (service 244). Side-effect: `rand.Intn(8)` (0-7) awarded async via `mp.Award` with idempotency key `moyu:checkin:<uid>:<date>` (service 256-257) — replay-safe; `points==0` is a no-op award. DB flag is set BEFORE the async award so a missed award can't enable re-check-in. No Redis rate-limit (router comment 21-27 — DB flag is single source of truth, correct to avoid post-midnight dead window).
- issues: none

### PUT /user/:id/follow — UserHandler.Follow
- verdict: fix
- tested: Live: `PUT /user/2/follow` as user 2 → `{"code":40000,"message":"cannot follow yourself"}` (self-follow guard works, service 138). `PUT /user/551/follow` (551 has no local user row) → `{"code":40000,"message":"ERROR: insert or update on table \"user_follow_relation\" violates foreign key constraint \"user_follow_relation_following_id_fkey\" (SQLSTATE 23503)"}`. Idempotent for an EXISTING relation: re-follow returns "already following this user" (service 143-145).
- issues:
  - [medium][security] Raw Postgres error string is leaked to the client when the followee has no local `user` row (the `following_id` FK is RESTRICT, migration 000). Handler wraps the service error verbatim: `errors.ErrBadRequest(err.Error())` (handler 230), and the service returns `repo.CreateFollow`'s raw gorm/pq error (service 148). Followee ids come from OAuth and may legitimately lack a local row, so this is reachable. EVIDENCE: handler.go:229-231 — `if err := h.service.Follow(...); err != nil { return response.Error(c, errors.ErrBadRequest(err.Error())) }`; service.go:147-150 returns the raw `CreateFollow` error. FIX (BE): in `service.Follow`, detect the FK violation (or pre-check the followee exists via `repo.FindByID`) and return a clean `fmt.Errorf("用户不存在")` instead of the raw driver error; never surface SQLSTATE strings to clients.

### DELETE /user/:id/follow — UserHandler.Unfollow
- verdict: fix
- tested: PROVEN live. As admin user 2 (which does NOT follow user 1, and no relation exists): `DELETE /user/1/follow` → `{"code":0,"message":"Unfollowed"}`. DB before: `user(id=1).follower_count = 11` (11 real follow relations exist). DB after: `follower_count = 10` — decremented despite NO relation being deleted (the 11 real relations are intact). Restored to 11 via psql after the test.
- issues:
  - [high][bug/security] `Unfollow` decrements follow counts unconditionally, even when no follow relation existed — corrupting another user's `follower_count`. `repo.DeleteFollow` uses `db.Where(...).Delete(...)` which returns nil error on 0 rows-affected (repository.go:132-135), so `service.Unfollow` always proceeds to `repo.UpdateFollowCounts(followerID, followingID, -1)` (service.go:156-160). Any logged-in user can repeatedly call `DELETE /user/:victim/follow` (without ever following) to drive `victim.follower_count` toward 0 while real relations remain — a data-integrity / harassment vector. `GREATEST(...,0)` only prevents going negative; it does not prevent corrupting a legitimate positive count. EVIDENCE: repository.go:132-135 (`DeleteFollow` ignores `RowsAffected`); service.go:157-160 (`if err := DeleteFollow(...); err != nil {...}; return UpdateFollowCounts(...,-1)`). FIX (BE): make `DeleteFollow` return `(rowsAffected int64, err error)` (or check `.RowsAffected`); in `service.Unfollow` only call `UpdateFollowCounts(-1)` when a row was actually deleted, else return "not following this user" (or a no-op success without decrement). Mirror the existing-relation guard that `Follow` already has.

## Cross-cutting
- [info][note] `GET /user/:id/resource` (and `/comment`) return the full `PatchResource` including `content` (which for legacy-archived rows is a live `https://oss.moyu.moe/...` download URL), `s3_key`, `password`, `code` — NOT gated behind the rate-limited `/resource/:id/link` reveal flow. VERIFIED this is NOT a user-domain defect: the canonical `GET /patch/:id/resource` (patch/service.GetResources, service.go:639-648) returns the IDENTICAL fields with no stripping, so the user-profile list merely mirrors the established app-wide behavior. For modern s3 uploads `content == s3_key` (a harmless relative key, the public URL is only minted at `/link` time); the leak is specific to legacy rows whose `content` column already holds an absolute URL. This is a pre-existing design state owned by the patch/common domains (note: common/handler.go:437-440 and Hikari:524-528 DO strip `content` in their own contexts, showing the redaction pattern exists but isn't applied to either resource-list path). Out of scope to "fix" from the user domain; flagging for the patch/common audit.
- [low][note] All six `/user/:id/{patch,resource,favorite,comment,contribute}` + follower/following list endpoints share `GetUserProfileRequest{Page,Limit}` with `validate:"min=1"` (+ Limit `max=20`), which negates the per-handler `if ==0 { default }` blocks — omitting `page`/`limit` yields 40000 rather than defaulting. See the `/user/:id/patch` entry for the concrete fix.
