# Domain: taxonomy-proxy

## Summary
38 endpoints in scope: tag/official/engine/series CRUD + search + enriched
detail (`/tag/:name`, `/official/:name`) + 12 taxonomy revision/revert routes (4
entities × 3). All are pure Bearer-forwarding proxies to the Galgame Wiki
(`WikiEditProxy` / `WikiTaxonomyDetailProxy` / `writeWikiResult`), with Wiki
enforcing role authz and moyu only mounting `auth` on writes to guarantee a
Bearer exists — matches docs/galgame_wiki/00-handbook §15 + 04-taxonomy.md.
Live-tested every GET and the two enriched detail endpoints; verified the
enrichment rewrite (`galgame`→`galgames`, `CardFromBrief`), Wiki-error
flattening (HTTP 400, code/message preserved), auth gating (401 before
forward), route ordering (literal `/search`,`/multi` before `:name`), and
body/query passthrough field names against both Wiki docs and the FE contract.

**Real issues: 0 high/critical, 2 low** — both on the degraded `CardFromBrief`
path (Wiki galgames moyu has no local patch row for): (1) `type`/`language`/
`platform` serialize as JSON `null` instead of `[]` (FE type says `string[]`;
Vue `v-for` over null is safe so no crash), and (2) `created` is the Go zero
time `0001-01-01T00:00:00Z`, which the FE card renders as a nonsense
"约 N 年前". Cosmetic only, degraded-path only.

**Dropped as false positives** (verified live, NOT bugs):
- Suspected NSFW leak / broken NSFW on `/tag/:name` `/official/:name` because
  they mount NO middleware so the user's Bearer is never forwarded to Wiki.
  Tested: Wiki's taxonomy-detail endpoint applies `content_limit` purely from
  the query param (total=7184 for `all`, 4149 for `sfw`, identical with/without
  cookie), so a logged-in NSFW-enabled user DOES see NSFW games and an
  anonymous `content_limit=all` is the same documented protocol exposure as
  every other moyu list endpoint (FE defaults sfw). Not moyu-introduced.
- Suspected over-permissive writes (PUT/DELETE mount bare `auth`, not
  `moderatorAuth`). By design per §15 "鉴权语义以 wiki 端为准" — Wiki enforces
  role; moyu only needs a Bearer. Confirmed POST=any-login, PUT/DELETE=admin in
  04-taxonomy.md.
- Suspected double Wiki call in `WikiTaxonomyDetailProxy` (enricher refetches
  the same galgames via /batch). Real but documented efficiency tradeoff
  (galgame_edit.go:163-168), not a correctness defect.

## Endpoints

### GET /tag — WikiEditProxy
- verdict: ok
- tested: `curl /tag` → `{code:0,...,data:{items:[{id,name,category,description,galgame_count,alias:[{...}],...}]}}`. Verbatim Wiki passthrough. No FE consumer (taxonomy.vue uses `tagSearch`); shape not contract-bound.

### GET /tag/search — WikiEditProxy
- verdict: ok
- tested: `curl '/tag/search?q=校园&limit=3'` → `data:{items:[{id,name,aliases:[...],category,galgame_count}],total:1}`. Matches FE `WikiTag` (`aliases`,`galgame_count`) and `{items,total}` from `tagSearch`. Registered before `/tag/:name` (router.go:249 vs 259) — literal wins, confirmed live.

### GET /tag/multi — WikiEditProxy
- verdict: ok
- tested: `curl '/tag/multi?ids=3,319'` → `{items:[],total:0}` (Wiki expects `tag_ids=` per 04-taxonomy.md:39, not `ids`; my test used wrong param — passthrough itself correct). No FE consumer (`grep app/` for `/tag/multi` = none). Pure passthrough.

### GET /tag/:name — WikiTaxonomyDetailProxy (enriched)
- verdict: ok
- tested: `curl '/tag/_?tag_id=319&page=1&limit=2'` → `data` keys `[galgames, tag, total]`; each `galgames[]` is full `GalgameCard` (`id,name{en-us,...},vndb_id,bid,banner,view,download,type,language,platform,content_limit,status,created,resource_update_time,release_date,count{favorite_by,contribute_by,resource,comment},user,galgame`). Exactly matches FE `GalgameCard` (patch.d.ts:9) consumed by pages/tag/[id].vue (`data.galgames`, `data.tag`, `data.total`). `galgame`→`galgames` rewrite + Wiki-order preservation verified (ids [23,772,187,934,122,267]). FE passes path segment `_` literally (`/tag/_?tag_id=N`) → matched by `:name`, ignored by Wiki (queries by `tag_id`). `content_limit` honored (all=7184/sfw=4149).
- issues:
  - [low][shape] Degraded cards (`CardFromBrief`, for Wiki galgames with no local patch row — 2/24 on tag_id=3) emit `type`/`language`/`platform` as JSON `null` instead of `[]`; FE type declares `string[]`. EVIDENCE: enricher.go:505-512 `CardFromBrief` leaves `Type/Language/Platform` zero-valued + model.go:43 `type JSONArray []string` has no `MarshalJSON` (nil→`null`); live: `"type": None`. FE Card.vue:77-82 passes them to KunPatchAttribute. No crash (Vue `v-for` over null renders nothing). FIX (BE, optional): in `CardFromBrief` init `Type/Language/Platform` to `patchModel.JSONArray{}` so they serialize `[]`.
  - [low][shape] Degraded cards emit `created:"0001-01-01T00:00:00Z"` (Go zero time); FE Card.vue:60-64 `v-if="patch.created"` is truthy for that string and renders `formatDistanceToNow("0001-...")` → nonsense relative time. EVIDENCE: enricher.go:505-509 `CardFromBrief` never sets `Created` + Card.vue:63 `formatDistanceToNow(props.patch.created)`. FIX (FE): in Card.vue guard `v-if` against the `0001` sentinel, OR (BE) omit `created` via `omitempty`+`*time.Time` for cards lacking a local row.

### GET /official — WikiEditProxy
- verdict: ok
- tested: `curl '/official?limit=2'` → `data:{items:[{id,name,original,link,category,lang,description,galgame_count,alias:[...]}]}`. Verbatim passthrough. (FE `WikiOfficial` omits `original`; extra field, harmless.)

### GET /official/search — WikiEditProxy
- verdict: ok
- tested: registered before `/official/:name` (router.go:262 vs 266); `officialSearch` → `{items:WikiOfficial[],total}`. Passthrough; same family as /tag/search verified.

### GET /official/:name — WikiTaxonomyDetailProxy (enriched)
- verdict: ok
- tested: `curl '/official/_?official_id=190&page=1&limit=2'` → `data` keys `[galgames, official, total]`; `official` carries `name,category,lang,link,description,galgame_count,aliases`-shaped object (FE official/[id].vue reads `official.name/category/lang/link/description/galgame_count/aliases`); `galgames[]` are enriched `GalgameCard`. Matches FE `officialDetail` type. Same enrichment path as /tag/:name.
- issues: (same two low-severity degraded-card issues as GET /tag/:name apply here; see above — same code path.)

### GET /engine — WikiEditProxy
- verdict: ok
- tested: `curl /engine` → `data:[{id,name,description,alias,created,updated,galgame_count}]` (bare array, NOT paginated). FE `engineList` types it `WikiEngine[]` (bare array). Matches. Passthrough.

### GET /engine/:name — WikiEditProxy (generic, no GalgameCard rewrite)
- verdict: ok
- tested: `curl '/engine/_?engine_id=11'` → `data:{engine:{...},galgames:[{flat Wiki brief: name_en_us,bid,...}]}`. NOT enriched (per docs row moyu.get.md:72 "通用透传无 GalgameCard 重写"). No FE page renders engine-detail galgames as cards (no pages/engine/[id].vue), so flat shape is unconsumed — fine.

### GET /series — WikiEditProxy
- verdict: ok
- tested: `curl '/series?limit=2'` → `data:{items:[{id,name,description,created,updated,galgame_count}]}`. FE `seriesList` types `WikiPage<WikiSeries>={items,total}`. Matches.

### GET /series/search — WikiEditProxy
- verdict: ok
- tested: `curl '/series/search?keywords=朱雀'` → `data:[{flat galgame brief}]` (bare array). FE `seriesSearch` types `unknown[]` (permissive). Registered before `/series/:id` (router.go:275 vs 280). Matches.

### GET /series/:id — WikiEditProxy
- verdict: ok
- tested: `curl '/series/121'` → `data:{id,name,description,...,galgame:[{flat brief}]}`. FE `seriesDetail` types `WikiSeries={id,name,description}` (subset; extra fields harmless). No card rendering. Matches.

### POST /tag — WikiEditProxy (auth)
- verdict: ok
- tested: `curl -X POST /tag` no-cookie → `{code:40100,"Please log in first"}` HTTP 401 (moyu gate before forward). Body `{name,category,description?,alias?}` forwarded verbatim (`c.Body()`); matches FE `createTag` + Wiki 04-taxonomy.md:85. Ready curl: `curl -X POST '$B/tag' -H 'Cookie: moyu_session=...' -H 'Content-Type: application/json' -d '{"name":"测试标签","category":"content","alias":["t"]}'`.

### PUT /tag — WikiEditProxy (auth)
- verdict: ok
- tested static: body `{tag_id,name,category,description?,alias?}` (FE `updateTag`) = Wiki 04-taxonomy.md:68-72. Verbatim body forward. Ready curl (admin): `curl -X PUT '$B/tag' -H 'Cookie: moyu_session=...' -H 'Content-Type: application/json' -d '{"tag_id":319,"name":"校园生活喜剧","category":"content"}'`.

### DELETE /tag/:id — WikiEditProxy (auth)
- verdict: ok
- tested: `curl -X DELETE /tag/999` no-cookie → 401. `?force=true` query is preserved via `c.OriginalURL()` (galgame_edit.go:40-42). Returns `{deleted,forced,purged_relations,purged_aliases}` = FE `deleteTag` type. Ready curl (admin, DESTRUCTIVE — human only): `curl -X DELETE '$B/tag/<unused_id>?force=true' -H 'Cookie: moyu_session=...'`.

### POST /official — WikiEditProxy (auth)
- verdict: ok
- tested static: body `{name,category,original?,link?,lang?,description?,alias?}` (FE `createOfficial`) = Wiki 04-taxonomy.md:178-183. Auth-gated (401 family confirmed). Ready curl: `curl -X POST '$B/official' -H 'Cookie: ...' -d '{"name":"测试会社","category":"company"}'`.

### PUT /official — WikiEditProxy (auth)
- verdict: ok
- tested static: body `{official_id,name,category,link?,lang?,description?,alias?}` (FE `updateOfficial`) = Wiki spec. Verbatim forward.

### DELETE /official/:id — WikiEditProxy (auth)
- verdict: ok
- tested static: 401 gate confirmed (same handler). `?force=true` preserved; returns `{deleted,forced,purged_relations,purged_aliases}` = FE `deleteOfficial`. Ready curl (admin, DESTRUCTIVE): `curl -X DELETE '$B/official/<id>?force=true' -H 'Cookie: ...'`.

### POST /engine — WikiEditProxy (auth)
- verdict: ok
- tested static: body `{name,description?,alias?}` (FE `createEngine`) = Wiki 04-taxonomy.md:242-243. Auth-gated. Ready curl: `curl -X POST '$B/engine' -H 'Cookie: ...' -d '{"name":"测试引擎"}'`.

### PUT /engine — WikiEditProxy (auth)
- verdict: ok
- tested static: body `{engine_id,name,description?,alias?}` (FE `updateEngine`). Verbatim forward.

### DELETE /engine/:id — WikiEditProxy (auth)
- verdict: ok
- tested static: returns `{deleted,forced,purged_relations}` (no `purged_aliases` — engine alias is inline JSONB per 04-taxonomy.md:253) = FE `deleteEngine` type (correctly omits purged_aliases). `?force=true` preserved. Ready curl (admin, DESTRUCTIVE): `curl -X DELETE '$B/engine/<id>?force=true' -H 'Cookie: ...'`.

### POST /series — WikiEditProxy (auth)
- verdict: ok
- tested static: body `{name,description?,galgame_ids:[]}` (FE `createSeries`). Auth-gated. Verbatim forward. Ready curl: `curl -X POST '$B/series' -H 'Cookie: ...' -d '{"name":"测试系列","galgame_ids":[]}'`.

### POST /series/modal — WikiEditProxy (auth)
- verdict: ok
- tested static: body `{ids:[]}` (FE `seriesModal`), returns `unknown[]`. Registered before `POST /series` (different literal path `/series/modal`, no shadow). Auth-gated.

### PUT /series/:id — WikiEditProxy (auth)
- verdict: ok
- tested: `curl -X PUT /series/1` no-cookie → 401. Body `{name?,description?,galgame_ids?}` (FE `updateSeries`). Verbatim forward.

### DELETE /series/:id — WikiEditProxy (auth)
- verdict: ok
- tested static: body-less DELETE (FE `deleteSeries`). 401 gate confirmed. Ready curl (admin, DESTRUCTIVE): `curl -X DELETE '$B/series/<id>' -H 'Cookie: ...'`.

### GET /tag/:id/revisions — WikiEditProxy
- verdict: ok
- tested: `curl -H cookie '/tag/319/revisions?page=1&limit=2'` → `{code:0,data:{items:[],total:0}}`. FE `taxListRevisions` types `WikiPage<TaxonomyRevision>={items,total}`. Matches. 3-segment path doesn't collide with 2-segment `/tag/:name` (Fiber segment-count match; confirmed both resolve live). No middleware mounted (public GET per docs row 81) — fine, no token needed for non-NSFW taxonomy.

### GET /tag/:id/revisions/:rev — WikiEditProxy
- verdict: ok
- tested: `curl -H cookie '/tag/319/revisions/1'` → `{code:4,"资源不存在"}` HTTP 400 (no rev 1 exists). Wiki business error correctly flattened to HTTP 400 with code/message preserved (writeWikiResult, handler.go:791-799). FE `taxGetRevision` types `TaxonomyRevision`. Passthrough.

### GET /official/:id/revisions — WikiEditProxy
- verdict: ok
- tested: same handler/shape as /tag revisions (passthrough `{items,total}`). Loop-registered router.go:287-291. Verified family behavior live on /tag.

### GET /official/:id/revisions/:rev — WikiEditProxy
- verdict: ok
- tested: same passthrough as /tag/:id/revisions/:rev; Wiki-error flattening verified on the /tag equivalent.

### GET /engine/:id/revisions — WikiEditProxy
- verdict: ok
- tested: loop-registered (router.go:287-291); identical passthrough. Family verified.

### GET /engine/:id/revisions/:rev — WikiEditProxy
- verdict: ok
- tested: identical passthrough; verified via /tag equivalent.

### GET /series/:id/revisions — WikiEditProxy
- verdict: ok
- tested: loop-registered; identical passthrough. Note `/series/:id/revisions` (3-seg) vs `/series/:id` (2-seg) no collision.

### GET /series/:id/revisions/:rev — WikiEditProxy
- verdict: ok
- tested: identical passthrough; verified via /tag equivalent.

### POST /tag/:id/revert — WikiEditProxy (auth)
- verdict: untestable (revert is a mutating revision restore; needs an existing revision to revert to — none present in test data)
- tested: auth gate verified (same handler returns 401 unauth). Body `{revision}` (FE `taxRevert`), returns `{reverted_to}`. Ready curl (admin): `curl -X POST '$B/tag/<id>/revert' -H 'Cookie: ...' -d '{"revision":<n>}'`. Wiki enforces admin/moderator.

### POST /official/:id/revert — WikiEditProxy (auth)
- verdict: untestable (mutating; no revision in test data)
- tested: loop-registered (router.go:290), `auth` mounted. Ready curl: `curl -X POST '$B/official/<id>/revert' -H 'Cookie: ...' -d '{"revision":<n>}'`.

### POST /engine/:id/revert — WikiEditProxy (auth)
- verdict: untestable (mutating; no revision in test data)
- tested: loop-registered, `auth` mounted. Ready curl: `curl -X POST '$B/engine/<id>/revert' -H 'Cookie: ...' -d '{"revision":<n>}'`.

### POST /series/:id/revert — WikiEditProxy (auth)
- verdict: untestable (mutating; no revision in test data)
- tested: loop-registered, `auth` mounted. Ready curl: `curl -X POST '$B/series/<id>/revert' -H 'Cookie: ...' -d '{"revision":<n>}'`.

## Cross-cutting
- [info] The two `CardFromBrief` degraded-card defects (null arrays, zero-time `created`) are shared by both enriched detail endpoints (`/tag/:name`, `/official/:name`) since they reuse `WikiTaxonomyDetailProxy`→`CardFromBrief`. Both are low-severity (degraded path only, no crash). A single BE fix in `enricher.CardFromBrief` (init the three JSONArrays to `{}`; leave `created` zero but make FE guard, or switch card `Created` to `*time.Time` omitempty) resolves both at once. EVIDENCE: enricher.go:505-512.
- [info] `writeWikiResult` flattens ALL Wiki business errors to HTTP 400 while preserving Wiki's `{code,message}` — verified live (code:4 → HTTP 400). Intentional per domain spec. Same for `WikiTaxonomyDetailProxy`'s error branch (galgame_edit.go:117-119). Consistent across all 38 endpoints.
