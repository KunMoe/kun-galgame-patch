# Domain: galgame-proxy

## Summary
Audited 26 endpoints: 4 submission GETs (mine/search-publish/messages-mine/messages-read-state), the read-state PUT, the submission write ops (submit/claim/PATCH draft/DELETE draft), the `PUT /galgame/:gid` metadata edit, and the §15 editing proxies (revisions×3, revert, prs×2 GET + POST + merge + decline, links GET/POST/DELETE, aliases GET/POST/DELETE). No real defects found — every endpoint verified `ok` or `intentional`. All GETs were tested live against the running stack (admin cookie + anonymous); the read-state PUT was round-trip tested (forward-only GREATEST upsert confirmed) and the DB row reset to 0 afterward. The NSFW content_limit gate on every `:gid` sub-resource was verified live (anonymous sfw → 404 on nsfw gid=33; `content_limit=nsfw/all` → data). Mutating write proxies were not executed (they create real Wiki revisions / award points); ready-to-run curls are provided. Two non-defects examined and dropped: (a) `links` response carries an undeclared `user_id` field — extra field, TS-permissive, harmless; (b) the wiki-messages endpoints (`messages/mine`, `messages/read-state`) currently have NO frontend consumer — dead-but-correct, not a bug.

## Endpoints

### GET /galgame/mine — ListMyGalgames
- verdict: ok
- tested: live `GET /galgame/mine?status=3,4&limit=5` (admin) → `{"code":0,...,"data":{"items":[],"total":0}}` (user id=2 has no drafts). Anonymous → `40100`. Shape `{items,total}` matches FE `MineResp`/`MineItem` (app/pages/me/submissions.vue:26-44) field-by-field: id/status/vndb_id/name_*/banner/effective_banner_hash/content_limit/created/updated/decline_reason. Client guarantees `items != nil` (submission.go:251-253).
- note: handler clamps limit 1..50 → 20, page≥1 (handler.go:980-985); FE sends `status=3,4&limit=50` (in range). Client only forwards `status` when non-empty (submission.go:217); docstring claims a "3,4 default" the code doesn't apply, but FE always sends status so it never matters — no defect.

### GET /galgame/search/publish — SearchGalgameForPublish
- verdict: ok
- tested: live `?q=fate&limit=2` → `{items:[...12 GalgameHit fields...]}`; `?q=zzzznomatch123` → `{"items":[],"pending":[],"total":0}` (nil slices normalized to `[]`, submission.go:298-304). Shape `{items,pending,total}` matches FE `SearchResult` (create.vue:48-52). Forwards `include_pending=true&facets=false&highlight=false` (submission.go:269-271). limit clamped 1..24→10 (handler.go:1008-1010); FE sends limit=12.

### GET /galgame/messages/mine — GetMyWikiMessages
- verdict: ok (no FE consumer)
- tested: live `?limit=3` (admin) → one message `{id,type:"approved",galgame_id,galgame:{...},actor_user_id,target_user_id,payload,created_at}` matching `WikiMessage`/`WikiMessageGalgame` (submission.go:99-121). `since_id` parsed (handler.go:1071), limit clamped 1..100→20. Returns `response.OK(out)` → `{items,total}` from `Paginated[WikiMessage]`. No frontend caller exists yet (grep of app/ for `messages/mine` finds none) — the notification bell/store use local `/message` only. Dead-but-correct.

### GET /galgame/messages/read-state — GetWikiMessagesReadState
- verdict: ok (no FE consumer)
- tested: live (admin) → `{"last_read_message_id":0}`. Anonymous → `40100`. Single-table raw read on `wiki_message_read_state` (handler.go:1027-1036); `_ = row.Scan(&lastRead)` tolerates a missing row (stays 0). No FE consumer.

### PUT /galgame/messages/read-state — UpdateWikiMessagesReadState
- verdict: ok (no FE consumer)
- tested: live round-trip (admin): PUT 5 → read-back 5 → PUT 2 → still 5 (forward-only `GREATEST` upsert, handler.go:1057) → PUT -1 → `40000 last_read_message_id 不能为负`. DB row reset to 0 afterward. Body `{last_read_message_id:int64}` parsed (handler.go:1044-1049). No FE consumer.

### POST /galgame/submit — SubmitGalgame
- verdict: ok
- tested: ready curl (not executed — creates a real Wiki draft + consumes daily quota). JSON + multipart(`data`+`file`) both handled (handler.go:813-850); `ensureCanPublishGalgame` gate applied (handler.go:814); empty token → `40100`. Wiki business codes forwarded verbatim via `writeWikiResult` → `{code:<wiki>,message:<wiki>,data:null}` HTTP 400; FE handles 20009 (quota) + generic (create.vue:277-281). JSON path decodes into typed `SubmitGalgameRequest` (omitempty pointers), so FE's partial payload (only non-empty fields) forwards correctly.
  Ready curl (JSON): `curl -X POST http://127.0.0.1:5214/api/v1/galgame/submit -H 'Content-Type: application/json' -H 'Cookie: moyu_session=<sess>' -d '{"name_zh_cn":"测试","content_limit":"sfw","age_limit":"all","original_language":"ja-jp"}'`

### POST /galgame/:gid/claim — ClaimGalgame
- verdict: ok
- tested: ready curl (not executed — flips a VNDB draft to published + awards +3 + creates local row). Side-effect verified static: single `+3` via `RegisterClaimedGalgame` (service.go:241-278) — early-return when patch row already exists (no re-reward, service.go:245-249); award goes through `s.mp.Award(...,"moyu:claim:<id>")` keyed for replay safety; contributor relation + contribute_count+1 inside the tx. Handler parses Wiki's `{id,vndb_id}`, falls back to path gid on parse failure (handler.go:885-892), returns `{id:patchID}` matching FE `{id}` consumer (create.vue:122-128). On Wiki-claim-success-but-local-register-fail returns a soft 50000 message (handler.go:896-902) — by design, Wiki claim can't be rolled back. No double-+3 (the prior bug; FE no longer also POSTs /patch, create.vue:119-121).
  Ready curl: `curl -X POST http://127.0.0.1:5214/api/v1/galgame/<draft_gid>/claim -H 'Content-Type: application/json' -H 'Cookie: moyu_session=<sess>' -d '{}'`

### PATCH /galgame/:gid — PatchGalgameDraft
- verdict: ok
- tested: ready curl (not executed — edits a real draft). JSON + multipart both handled (handler.go:915-945); empty token → `40100`; invalid gid → `40000`. Wiki enforces submitter + status∈{3,4} and auto-flips 4→3. FE (edit/draft.vue:160-168) sends PATCH with JSON or multipart `data`+`file` — matches.
  Ready curl: `curl -X PATCH http://127.0.0.1:5214/api/v1/galgame/<draft_gid> -H 'Content-Type: application/json' -H 'Cookie: moyu_session=<sess>' -d '{"name_zh_cn":"改名"}'`

### DELETE /galgame/:gid — DeleteGalgameDraft
- verdict: ok
- tested: ready curl (not executed — hard-deletes a draft). Returns `response.OKMessage("OK")` → `{code:0,message:"OK",data:null}` on success; Wiki business error forwarded as HTTP 400 (handler.go:962-967). FE (submissions.vue:78) checks `res.code===0`. Wiki enforces submitter + status∈{3,4}.
  Ready curl: `curl -X DELETE http://127.0.0.1:5214/api/v1/galgame/<draft_gid> -H 'Cookie: moyu_session=<sess>'`

### PUT /galgame/:gid — UpdateGalgame
- verdict: ok
- tested: ready curl (not executed — edits published galgame metadata + creates a Wiki revision). JSON + multipart both handled (handler.go:706-785). JSON decodes into `UpdateGalgameRequest` (omitempty pointers) which silently drops unsupported keys e.g. `vndb_id` (handler.go:723-733). Multipart: `data` JSON + optional `file` (≤10MB), falls back to JSON mode when no file (handler.go:757-761). `writeWikiResult` forwards Wiki code+message. FE rewrite.vue:403-411 sends PUT JSON or multipart — matches.
  Ready curl: `curl -X PUT http://127.0.0.1:5214/api/v1/galgame/1142 -H 'Content-Type: application/json' -H 'Cookie: moyu_session=<sess>' -d '{"intro_zh_cn":"..."}'`

### GET /galgame/:gid/revisions — WikiEditProxy
- verdict: ok
- tested: live `?limit=2` (admin, gid=1142) → `{items:[{id,galgame_id,revision,user_id,action,note,snapshot,...}],total}` matching FE `WikiPage<GalgameRevision>`. NSFW gate verified: anon nsfw gid=33 → `40400`; `?content_limit=nsfw` → data. Verbatim path passthrough via `c.OriginalURL()` strip of `/api/v1` (galgame_edit.go:40-42).

### GET /galgame/:gid/revisions/:rev — WikiEditProxy
- verdict: ok
- tested: live gid=1142 rev=1 (anon) → `GalgameRevisionDetail` shape. NSFW gid=33 anon → `40400`. Path forwarded verbatim.

### GET /galgame/:gid/revisions/:rev/diff — WikiEditProxy
- verdict: ok
- tested: live gid=1142 rev=1 (anon) → `{changed_keys,old,new,...}` matching FE `GalgameDiff`. NSFW gid=33 anon → `40400` (gate keys on `:gid` even on the 3-segment subpath).

### POST /galgame/:gid/revert — WikiEditProxy
- verdict: ok
- tested: anon → `40100` (auth middleware, no token → `WikiEditProxy` returns ErrUnauthorized at galgame_edit.go:49-51). Forwards JSON body `{revision}` (FE useGalgameEdit.ts:227-228) with its Content-Type. Wiki enforces admin/moderator. Ready curl: `curl -X POST .../galgame/<gid>/revert -H 'Content-Type: application/json' -H 'Cookie: ...' -d '{"revision":N}'`

### GET /galgame/:gid/prs — WikiEditProxy
- verdict: ok
- tested: live gid=1142 (admin) → `{"items":[],"total":0}` matching FE `WikiPage<GalgamePR>`. NSFW gid=33 anon → `40400`.

### GET /galgame/:gid/prs/:prid — WikiEditProxy
- verdict: ok
- tested: NSFW gid=33 anon → `40400`. For a public gid Wiki returns `GalgamePRDetail` `{pr,changed_keys,names?}`; live gid=1142 had no PRs so prid lookup returns Wiki's not-found (flattened to HTTP 400 per writeWikiResult convention). Path forwarded verbatim.

### POST /galgame/:gid/prs — WikiPRSubmit
- verdict: ok
- tested: anon → `40100` (galgame_edit.go:208-211). Dual content-type: JSON forwarded raw via `Proxy`; multipart parses `data`(required)+`file`(optional, ≤10MB) and re-forwards via `ProxyMultipart` (galgame_edit.go:215-260). FE useGalgameEdit.ts:237-260 sends both forms. Ready curl (JSON): `curl -X POST .../galgame/<gid>/prs -H 'Content-Type: application/json' -H 'Cookie: ...' -d '{"name_zh_cn":"...","note":"..."}'`

### PUT /galgame/:gid/prs/:prid/merge — WikiEditProxy
- verdict: ok
- tested: anon → `40100`. Body-less PUT forwarded verbatim; Wiki enforces creator/admin. Ready curl: `curl -X PUT .../galgame/<gid>/prs/<prid>/merge -H 'Cookie: ...'`

### PUT /galgame/:gid/prs/:prid/decline — WikiEditProxy
- verdict: ok
- tested: anon → `40100`. Same as merge. Ready curl: `curl -X PUT .../galgame/<gid>/prs/<prid>/decline -H 'Cookie: ...'`

### GET /galgame/:gid/links — WikiEditProxy
- verdict: ok
- tested: live gid=1142 (admin) → bare array `[{id,name,link,galgame_id,user_id,created,updated}]` matching FE `GalgameLink[]` (FE type omits `user_id`; extra field is harmless under TS). NSFW gid=33 anon → `40400`.

### POST /galgame/:gid/links — WikiEditProxy
- verdict: ok
- tested: anon → `40100`. Forwards JSON `{name,link}` (useGalgameEdit.ts:311-312). Wiki enforces owner/admin + creates a revision. Ready curl: `curl -X POST .../galgame/<gid>/links -H 'Content-Type: application/json' -H 'Cookie: ...' -d '{"name":"VNDB","link":"https://vndb.org/vX"}'`

### DELETE /galgame/:gid/links — WikiEditProxy
- verdict: ok
- tested: anon (with body) → `40100`. DELETE-with-body: `WikiEditProxy` forwards `c.Body()` for non-GET (galgame_edit.go:71-74); FE `api.delete(path,{id})` sends JSON `{id}` (useApi.ts:157-158, useGalgameEdit.ts:313-314). Ready curl: `curl -X DELETE .../galgame/<gid>/links -H 'Content-Type: application/json' -H 'Cookie: ...' -d '{"id":<linkId>}'`

### GET /galgame/:gid/aliases — WikiEditProxy
- verdict: ok
- tested: live gid=1142 (admin) → bare array `[{id,name,galgame_id,created,updated}]` matching FE `GalgameAlias[]`. NSFW gid=33 anon → `40400`.

### POST /galgame/:gid/aliases — WikiEditProxy
- verdict: ok
- tested: anon → `40100`. Forwards JSON `{name}` (useGalgameEdit.ts:318-319). Ready curl: `curl -X POST .../galgame/<gid>/aliases -H 'Content-Type: application/json' -H 'Cookie: ...' -d '{"name":"别名"}'`

### DELETE /galgame/:gid/aliases — WikiEditProxy
- verdict: ok
- tested: anon (with body) → `40100`. Same DELETE-with-body pattern as links; FE sends `{id}`. Ready curl: `curl -X DELETE .../galgame/<gid>/aliases -H 'Content-Type: application/json' -H 'Cookie: ...' -d '{"id":<aliasId>}'`

## Cross-cutting
- [info] Route ORDER verified correct: the literal `/galgame/mine|search/publish|messages/*|submit` routes are registered BEFORE the parameterized `/galgame/:gid` family (router.go:107-143), so Fiber never matches `mine`/`submit` as a `:gid`. Live `GET /galgame/mine` resolves to ListMyGalgames (`{items,total}`), not PatchGalgameDraft — confirmed.
- [info] NSFW gate (`gatePatchByContentLimit`, handler.go:60-70) is applied to GET on every `:gid` sub-resource in `WikiEditProxy` (galgame_edit.go:61-69) and verified live to fail-closed (anon sfw default → 404 on nsfw galgames, including for logged-in callers who don't pass an explicit `content_limit`). The FE's `useApi` appends `content_limit` from the NSFW switcher; these list-semantic sub-resource pages stay sfw-default unless the user enables NSFW mode — matches the documented §16 protocol.
- [info] Wiki business-error passthrough is uniform: all in-scope handlers map `*WikiError` → `errors.New(code,message,400)` (handler.go:794, 878, 989, etc.), preserving Wiki's `{code,message}` while flattening to HTTP 400 — intentional per docs.
- [info] No frontend consumer exists for `GET /galgame/messages/mine`, `GET/PUT /galgame/messages/read-state` (grep of apps/web/app finds no caller; the message bell/store consume local `/message` only). Backend is correct and ready; flagged for product awareness, not a defect.
- none (no defects)
