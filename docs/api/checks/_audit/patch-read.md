# Domain: patch-read

## Summary
9 endpoints audited (GET /patch/duplicate, /patch/:id, /patch/:id/detail, /patch/:id/comment, /patch/:id/resource, /patch/:id/contributor, /patch/comment/:commentId/markdown, /patch/resource/:resourceId/link, /home/random). Backend response shapes match the FE `.d.ts` contracts field-by-field (verified live against real data ids 6690 sfw / 9104 nsfw). One real defect found: the `/link` rate limiter is keyed by IP even for logged-in users because no auth/optionalAuth middleware runs before it (medium). One low-severity null-vs-empty-array note on `/patch/:id/detail`. Dropped several initial suspicions as false positives after reading the full path: the "null comment list" and "markdown 404" I first saw were the NSFW content_limit gate working as designed (galgame 9104 is nsfw; anonymous/sfw callers correctly 404, logged-in callers get `content_limit=all` from useApi); the s3 `content` field in the resource list is legacy-stored as a full URL and unused by the FE for download.

## Endpoints

### GET /patch/duplicate — PatchHandler.CheckDuplicate
- verdict: ok
- tested: live. Anon → `{"code":40100,"message":"Please log in first"}` (route has `auth`, router.go:54). Admin `?vndb_id=v17` → `{"code":0,...,"data":{"exists":true}}`. FE consumer components/edit/create/VNDBInput.vue:33 expects `{ exists: boolean }`; BE returns `map[string]bool{"exists":...}` (handler.go:240). The only consumer lives on /edit/create which bounces unauthed users to home (pages/edit/create.vue:22), so the `auth` gate is consistent.
- issues: none

### GET /patch/:id — PatchHandler.GetPatch
- verdict: ok
- tested: live `GET /patch/6690` (anon) → 200 flat GalgameCard; admin tail shows `...,"is_favorite":false}`. Fields match FE `PatchHeader extends GalgameCard` (patch.d.ts:78) incl. `name{en-us,ja-jp,zh-cn,zh-tw}`, `count{favorite_by,contribute_by,resource,comment}`, `release_date` RFC3339, nested `galgame`, `user{id,name,avatar,avatar_image_hash,roles}`. `headerCard` embeds `GalgameCard` + `IsFavorite json:"is_favorite"` (handler.go:123-126) — exact match. NSFW: `GET /patch/9104` anon → 404 (9104 is nsfw); useApi sends `content_limit=all` for logged-in detail routes (useApi.ts:85-93), so logged-in viewers resolve it. Intentional SEO gate.
- issues: none

### GET /patch/:id/detail — PatchHandler.GetPatchDetail
- verdict: ok (one low note)
- tested: live `GET /patch/6690/detail` (admin) → keys = base GalgameCard + `introduction_markdown`, `introduction_html`, `updated`, `tags` (17), `officials` (1), `wiki_engine_ids`, `galgame`. Matches FE `PatchDetail` (patch.d.ts:102-109). Consumers: pages/patch/[id]/introduction.vue:13, prs.vue:83, edit/rewrite.vue:35 all `api.get<PatchDetail>`.
- issues:
  - [low][shape] When the galgame has no engines/tags/officials, the BE emits JSON `null` (not `[]`) for `wiki_engine_ids` / `tags` / `officials` because they are append-built `nil` slices (enricher.go:418-438; live `wiki_engine_ids:null` on 6690). FE types declare non-optional arrays (`wiki_engine_ids: number[]`, patch.d.ts:108). Any `.map`/`.length` on a null without a guard would throw. EVIDENCE: enricher.go:437 — `base.WikiEngineIDs = append(base.WikiEngineIDs, e.EngineID)` (stays nil when no engines). FIX: BE — initialize the three slices to `[]T{}` in `EnrichPatchDetail` (e.g. `base.Tags = []PatchDetailTag{}`) so empty serializes as `[]`; or FE — guard with `?? []` at consumers.

### GET /patch/:id/comment — PatchHandler.GetComments
- verdict: ok
- tested: live `GET /patch/6690/comment?page=1&limit=30` → `{"items":[],"total":0}`. `GET /patch/4893/comment` shows top-level comments each with `reply:[...]` nested (verified comment 27282 has 1 reply). Paginated envelope `{items,total}` (response.Paginated, handler.go:280) matches FE `CommentListResponse {items,total}` (comment.vue:17-20). Row shape matches `PatchPageComment` (comment.d.ts:31): `id,content,content_html,is_liked,like_count,parent_id,user_id,galgame_id,created,updated,reply,user,status`. BE `Replies json:"reply"` (model.go:225). Count restricted to `parent_id IS NULL AND status=0` (repository.go:164), matching the paginated unit (top-level), and replies preload also filters `status=0` (repository.go:170) so pending replies stay hidden. Pagination offset `(page-1)*limit` (service.go:428) — no off-by-one. Missing-query (`page`/`limit` required) → 40000, as designed.
- issues: none

### GET /patch/:id/resource — PatchHandler.GetResources
- verdict: ok
- tested: live `GET /patch/6690/resource` (admin) → bare array (not paginated), len 1, keys match FE `PatchResource` (resource.d.ts:2): storage,name,model_name,size,type,language,platform,note,note_html,blake3,s3_key,content,code,password,like_count,is_liked,status,download,user_id,galgame_id,created,update_time,updated,user. `response.OK(c, resources)` returns a bare array (handler.go:441); consumer pages/patch/[id]/resource.vue:13 `api.get<PatchResource[]>` expects a bare array — match. note_html rendered (service.go:644), is_liked stamped per-user (service.go:652-671), publisher brief attached (service.go:675-687). NSFW gate via `gatePatchByContentLimit` (handler.go:431).
- issues: none

### GET /patch/:id/contributor — PatchHandler.GetContributors
- verdict: ok
- tested: live `GET /patch/6690/contributor` (anon) → `[{id,name,avatar,avatar_image_hash,roles}]` (`model.PatchUser`, handler.go:674). NSFW gate verified: `GET /patch/9104/contributor` anon → 404, with `?content_limit=all` → 200 with briefs. Gate reads content_limit from query (not session), so it works even though the route has no auth/optionalAuth middleware (router.go:59). No current FE consumer (only referenced in comments, e.g. components/galgame/edit/Relations.vue:8); shape would map to `KunUser[]` if consumed.
- issues: none

### GET /patch/comment/:commentId/markdown — PatchHandler.GetCommentMarkdown
- verdict: ok
- tested: live. `GET /patch/comment/27286/markdown` anon → 404 (comment 27286 belongs to nsfw galgame 9104; gate fails closed for sfw default). `?content_limit=all` → `{"markdown":"求补丁"}`. Bad/nonexistent id → 404 "comment not found". Returns `{markdown:string}` (handler.go:415). Gate looks up the owning patch id then applies the same content_limit check as the comment list (handler.go:402-408) — prevents anon exfiltration of NSFW comment bodies by id. No current FE consumer.
- issues: none

### GET /patch/resource/:resourceId/link — PatchHandler.GetResourceDownloadInfo (RateLimit 30/min)
- verdict: fix
- tested: live. `GET /patch/resource/1914/link` anon → `{"storage":"s3","content":"https://oss.moyu.moe/...","code":"","password":""}` — matches FE `ResourceLinkInfo {storage,content,code,password}` (resource.vue:184-189). For s3 the public URL is materialized at read time via `S3Client.PublicURL` (service.go:1033). Disabled resource (status!=0) → 40310 (handler.go:593). NSFW gate present (handler.go:585). Fired 35 rapid requests → first 30 = 200, then 429 (correct cap). BUT Redis showed key `ratelimit:resource-link:ip:127.0.0.1` even when I sent the admin cookie — proving it keyed by IP, not userID (see issue).
- issues:
  - [medium][bug] The `/link` rate limiter keys by IP for ALL callers, never by userID, contradicting the documented intent. The route registers only `RateLimit(...)` with no `auth`/`optionalAuth` before it (router.go:82-86), so `GetUserID(c)` always returns 0 (it reads `c.Locals("user")`, only set by the auth middlewares — auth.go:251-257). The limiter's userID branch (ratelimit.go:21-25) is therefore dead. The router comment (router.go:78-80) states "capping at 30/min per userID (or per IP when anonymous)". Live proof: Redis key was `ratelimit:resource-link:ip:127.0.0.1` while authenticated. Impact: logged-in users behind a shared NAT/proxy share one IP bucket and get throttled collectively (false 429s) instead of each getting their own 30/min. EVIDENCE: router.go:82-86 — `patchRoutes.Get("/resource/:resourceId/link", middleware.RateLimit(a.RDB, "resource-link", 30, time.Minute), a.PatchHandler.GetResourceDownloadInfo)` (no auth/optionalAuth); ratelimit.go:17 — `userID := GetUserID(c)`. FIX: BE — insert `optionalAuth` before the rate limiter on this route: `patchRoutes.Get("/resource/:resourceId/link", optionalAuth, middleware.RateLimit(...), a.PatchHandler.GetResourceDownloadInfo)`. With optionalAuth populating the user context, logged-in callers key by `user:<id>` per the comment and anonymous still key by IP.

### GET /home/random — PatchHandler.GetRandomPatch
- verdict: ok
- tested: live. anon → `{"id":7983}`; admin → `{"id":197}`. Returns `map[string]int{"id":id}` (handler.go:690); FE components/kun/top-bar/RandomGalgameButton.vue:23 `api.get<{ id: number | string }>('/home/random')` — match. content_limit forwarded; with a non-empty cl the service samples 60 random ids and filters via wiki batch, picking a uniform-random survivor (service.go:399-420) so anon (sfw default) can't land on a NSFW patch. Empty filtered set → ErrRecordNotFound → 50000 (rare).
- issues: none

## Cross-cutting
- [low][shape] Nil-slice-as-JSON-null is a recurring pattern in the enricher (detail `tags`/`officials`/`wiki_engine_ids`). Only `/patch/:id/detail` is in this domain; flagged above. A repo-wide fix would be to default these slices to `[]T{}` before serialization. The base `GalgameCard.Type/Language/Platform` are `JSONArray` whose `Value()`/Scan default to `[]`, so the card-level arrays are safe — only the detail-only Wiki-derived slices are affected.
