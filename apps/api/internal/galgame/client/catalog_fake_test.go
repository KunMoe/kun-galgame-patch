package client

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
)

type fakeReq struct {
	method string
	path   string
	query  url.Values
}

type catalogFake struct {
	*httptest.Server
	mu   sync.Mutex
	reqs []fakeReq
}

var gidFixture = map[int]struct {
	catalogID int64
	state     string
	limit     string
}{
	// A page id IS the catalog work id (migration 037). The fixture used to
	// map gid 7 to catalog 900 because those were two id spaces; keeping the
	// two columns equal is what the whole codebase now assumes.
	7:  {7, catalogClaimStateLive, "sfw"},
	8:  {8, catalogClaimStateLive, "sfw"},
	20: {20, catalogClaimStateDraft, "nsfw"},
	21: {21, catalogClaimStateHidden, "nsfw"},
	22: {22, catalogClaimStateLive, "sfw"},
}

func ratingForCatalogID(id int64) string {
	if id == 22 {
		return "r18"
	}
	return "all_ages"
}

// Every body here is the shape nextmoe-infra's apiv2 handlers emit
// (handler/map_work.go, map_blocks.go, catalog_feed.go) for the types in
// apiv2/repr: string ids, name rows as objects, and every nullable key present
// as null. A fake that serves a retired shape cannot disagree with the code
// that reads it, and twice kept this suite green while a catalog wave took
// production down (aliases in wave 209, names in wave 210).
func newCatalogFake(t *testing.T) *catalogFake {
	t.Helper()
	f := &catalogFake{}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		f.mu.Lock()
		f.reqs = append(f.reqs, fakeReq{method: req.Method, path: req.URL.Path, query: req.URL.Query()})
		f.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		body := f.route(req)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(f.Server.Close)
	return f
}

func (f *catalogFake) route(req *http.Request) string {
	p := req.URL.Path
	q := req.URL.Query()
	switch {
	case p == "/v2/catalog/works" && q.Get("refs") != "":
		return f.worksByRefs(q.Get("refs"))
	case p == "/v2/catalog/works" && q.Get("ids") != "":
		return f.worksList(req)
	case p == "/v2/catalog/works" && q.Get("company_rollup") == "true":
		return f.companyRollup()
	case p == "/v2/catalog/works":
		return f.search(q.Get("include"))
	case p == "/v2/catalog/calendar":
		return f.calendar()
	case strings.HasPrefix(p, "/v2/catalog/tags/"):
		return f.tagRecord(p)
	case strings.HasPrefix(p, "/v2/catalog/companies/"):
		return f.labelRecord(p)
	case strings.HasPrefix(p, "/v2/catalog/series/"):
		return f.seriesRecord(p)
	case strings.HasPrefix(p, "/v2/catalog/works/"):
		return f.workDetail(req)
	}
	return `{"object":"list","items":[]}`
}

func (f *catalogFake) worksByRefs(raw string) string {
	items := make([]string, 0, 4)
	seen := map[int64]bool{}
	for _, part := range strings.Split(raw, ",") {
		_, ext, ok := strings.Cut(strings.TrimSpace(part), ":")
		if !ok {
			continue
		}
		gid, _ := strconv.Atoi(ext)
		fx, hit := gidFixture[gid]
		if !hit || seen[fx.catalogID] {
			continue
		}
		seen[fx.catalogID] = true
		items = append(items, workItem(fx.catalogID, gid, fx.state))
	}
	return `{"object":"list","items":[` + strings.Join(items, ",") + `]}`
}

func (f *catalogFake) worksList(req *http.Request) string {
	raw := req.URL.Query().Get("ids")
	items := make([]string, 0, 4)
	include := req.URL.Query().Get("include")
	for _, s := range strings.Split(raw, ",") {
		id, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
		if err != nil {
			continue
		}
		gid, state := gidForCatalogID(id)
		items = append(items, facetBlocks(workItem(id, gid, state), include))
	}
	return `{"object":"list","items":[` + strings.Join(items, ",") + `]}`
}

func (f *catalogFake) tagRecord(path string) string {
	id, _ := strconv.ParseInt(strings.TrimPrefix(path, "/v2/catalog/tags/"), 10, 64)
	sexual := "false"
	if id == 12 {
		sexual = "true"
	}
	return `{"object":"tag","id":"` + strconv.FormatInt(id, 10) + `","display_name":"tag","tier":"core","tag_kind":"content",` +
		`"work_count":3,"is_sexual":` + sexual + `,"intros":[]}`
}

func (f *catalogFake) seriesRecord(path string) string {
	id, _ := strconv.ParseInt(strings.TrimPrefix(path, "/v2/catalog/series/"), 10, 64)
	return `{"object":"series","id":"` + strconv.FormatInt(id, 10) + `","display_name":"Saga",` +
		`"work_count":2,"has_nsfw":true,` +
		`"intros":[{"lang":"zh-Hans","value":"系列简介","is_machine":false,"source":""}]}`
}

// include=aliases,logo,intros,links: the three list blocks always answer,
// empty or not, and logo is absent for a company that has none.
func (f *catalogFake) labelRecord(path string) string {
	id, _ := strconv.ParseInt(strings.TrimPrefix(path, "/v2/catalog/companies/"), 10, 64)
	return `{"object":"company","id":"` + strconv.FormatInt(id, 10) + `","display_name":"Brand","latin":null,"lang":"ja",` +
		`"localized":{},"company_kind":"game_brand","work_count":3,` +
		`"aliases":[{"lang":"en","value":"Brand Soft","alias_kind":"spelling_variant","is_machine":false}],` +
		`"intros":[],"links":[]}`
}

func (f *catalogFake) workDetail(req *http.Request) string {
	id, _ := strconv.ParseInt(strings.TrimPrefix(req.URL.Path, "/v2/catalog/works/"), 10, 64)
	gid, state := gidForCatalogID(id)
	// credits ride the detail face ONLY behind include=credits; the roster and
	// the ratings are unconditional. Serving credits either way would let the
	// include token be dropped without a test going red.
	credits := ``
	if strings.Contains(req.URL.Query().Get("include"), "credits") {
		credits = `,"credits":` + detailCreditsJSON
	}
	return `{"object":"work","id":"` + strconv.FormatInt(id, 10) + `","medium":"galgame","display_name":"タイトル","latin":null,` +
		`"olang":"ja","content_rating":"` + ratingForCatalogID(id) + `","content_limit":"` + workLimitFor(id) + `",` +
		`"release_date":"2026-07-14","release_date_precision":"day","release_status":"released",` +
		`"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-07-01T00:00:00Z",` +
		`"localized":{"ja":{"value":"タイトル","is_machine":false},` +
		`"zh":{"value":"机翻标题","is_machine":true},` +
		`"zh-Hans":{"value":"标题","is_machine":false},` +
		`"en":{"value":"Title","is_machine":false}},` +
		`"titles":[{"lang":"ja","title":"タイトル","latin":null,"title_kind":"official","is_machine":false}],` +
		`"refs":[{"source":"vndb","external_id":"v42"},{"source":"curated","external_id":"` + strconv.Itoa(gid) + `"}],` +
		`"claim":` + claimJSON(gid, state) + `,` +
		`"intros":[{"lang":"zh-Hans","value":"介绍","is_machine":false,"source":"vndb"}],` +
		`"covers":[` + detailCoverJSON("901", "hash1", true, "main", 600, 800) + `,` +
		detailCoverJSON("902", "hash2", false, "dig", 1280, 720) + `],` +
		`"screenshots":` + detailScreenshotsJSON + `,` +
		`"tags":[{"id":"11","display_name":"純愛","source":"vndb","tier":"core","tag_kind":"content","spoiler":"none","is_sexual":false,"work_count":40},` +
		`{"id":"12","display_name":"エロ","source":"vndb","tier":"core","tag_kind":"content","spoiler":"minor","is_sexual":true,"work_count":90}],` +
		`"companies":[{"object":"company","id":"31","display_name":"Brand","localized":{},"company_kind":"game_brand","attribution_role":"developer","work_count":3}],` +
		`"series":[],` +
		`"banner":` + coverSlotJSON("hash2", 1728, 1080, "curated") + `,` +
		`"cover":` + coverSlotJSON("hash3", 850, 1080, "upscale") + `,` +
		`"characters":` + detailCharactersJSON + `,` +
		`"ratings":` + detailRatingsJSON +
		credits + `}`
}

func coverSlotJSON(hash string, width, height int, source string) string {
	return `{"url":"https://cdn/aa/bb/` + hash + `.webp","hash":"` + hash + `","width":` + strconv.Itoa(width) +
		`,"height":` + strconv.Itoa(height) + `,"thumbhash":"th","sexual":"safe","violence":null,` +
		`"source":"` + source + `","origin":"cover"}`
}

func detailCoverJSON(rowID, hash string, pinned bool, kind string, width, height int) string {
	return `{"id":"` + rowID + `","vote_count":0,"portrait_pinned":` + strconv.FormatBool(pinned) +
		`,"cover_kind":"` + kind + `","url":"https://cdn/aa/bb/` + hash + `.webp","hash":"` + hash + `",` +
		`"width":` + strconv.Itoa(width) + `,"height":` + strconv.Itoa(height) + `,"thumbhash":"th",` +
		`"sexual":"safe","violence":null,"source":"vndb"}`
}

// The null grade is not invented: map_blocks.go emits sexual null exactly when
// the image has no metadata row, which is why its width and height are null
// beside it. violence is null on every row catalog holds today.
const detailScreenshotsJSON = `[` +
	`{"url":"https://cdn/aa/bb/shot1.webp","hash":"shot1","width":1280,"height":720,"thumbhash":"t1","sexual":"safe","violence":null,"source":"vndb","caption":""},` +
	`{"url":"https://cdn/aa/bb/shot2.webp","hash":"shot2","width":null,"height":null,"thumbhash":null,"sexual":null,"violence":null,"source":"vndb","caption":""},` +
	`{"url":"https://cdn/aa/bb/shot3.webp","hash":"shot3","width":1280,"height":720,"thumbhash":"t3","sexual":"suggestive","violence":null,"source":"vndb","caption":"水着"}]`

// Trimmed from a live read of work 3 on 2026-08-19, reshaped to the v2
// WorkCharacter the roster block answers. 雪々's art is graded suggestive so
// an SFW read has something to keep back.
const detailCharactersJSON = `[` +
	`{"object":"character","id":"1699","display_name":"コロナ","latin":null,` +
	`"localized":{"zh-Hans":{"value":"科罗娜","is_machine":true}},` +
	`"roster_role":"main","spoiler":"none","identity":"r1699","voices":[],` +
	`"image":{"url":"https://cdn/aa/bb/chara1.webp","hash":"chara1","width":250,"height":300,"thumbhash":"c1","sexual":"safe","violence":null,"source":""},` +
	`"figure":{"url":"https://cdn/aa/bb/figure1.webp","hash":"figure1","width":490,"height":492,"thumbhash":"f1","sexual":"safe","violence":null,"source":""}},` +
	`{"object":"character","id":"1700","display_name":"雪々","latin":null,"localized":{},` +
	`"roster_role":"secondary","spoiler":"minor","identity":"r1700","voices":[],` +
	`"image":{"url":"https://cdn/aa/bb/chara2.webp","hash":"chara2","width":250,"height":300,"thumbhash":"c2","sexual":"suggestive","violence":null,"source":""},` +
	`"figure":null}]`

const detailRatingsJSON = `[` +
	`{"source":"vndb","score":8.1,"vote_count":500,"rank":null,"distribution":[{"score":9,"count":126},{"score":10,"count":38}],` +
	`"stats":{"average":8.1,"stdev":null,"min":null,"max":null}},` +
	`{"source":"erogamescape","score":78.5,"vote_count":42,"rank":2917,"distribution":[{"score":70,"count":9},{"score":80,"count":21}]},` +
	`{"source":"dlsite","score":4.6,"vote_count":0,"rank":null}]`

func creditJSON(id, name, localized, characterID string) string {
	return `{"object":"credit_name","id":"` + id + `","display_name":"` + name + `","latin":null,` +
		`"localized":` + localized + `,"character_id":` + characterID + `,"identity":"c` + id + `"}`
}

// scenario/剧本 and illustration/原画 are the same role under two source
// vocabularies, developer duplicates the 会社 chips, and 保住圭 rides both a real
// role and other-staff.
var detailCreditsJSON = `[` +
	`{"role_key":"scenario","role_name":"剧本","credits":[` + creditJSON("900", "保住圭", `{}`, "null") + `]},` +
	`{"role_key":"剧本","role_name":"剧本","credits":[` + creditJSON("901", "丸戸史明", `{"zh-Hans":{"value":"丸户史明","is_machine":false}}`, "null") + `]},` +
	`{"role_key":"原画","role_name":"原画","credits":[` + creditJSON("902", "深崎暮人", `{}`, "null") + `]},` +
	`{"role_key":"developer","role_name":"开发","credits":[` + creditJSON("903", "Brand", `{}`, "null") + `]},` +
	`{"role_key":"voice-actor","role_name":"声优","credits":[` + creditJSON("1550", "榎木実佳", `{"zh-Hans":{"value":"榎木实佳","is_machine":false}}`, `"1699"`) + `]},` +
	`{"role_key":"other-staff","role_name":"其他","credits":[` +
	creditJSON("900", "保住圭 (Hozumi Kei)", `{}`, "null") + `,` +
	creditJSON("904", "なかひろ", `{}`, "null") + `]}]`

func (f *catalogFake) search(include string) string {
	total := int64(4)
	return `{"object":"list","total":` + strconv.FormatInt(total, 10) + `,"items":[` +
		facetBlocks(workItem(900, 7, catalogClaimStateLive), include) + `,` +
		facetBlocks(workItem(920, 20, catalogClaimStateDraft), include) + `,` +
		facetBlocks(workItem(921, 21, catalogClaimStateHidden), include) + `,` +
		facetBlocks(workItem(930, 0, ""), include) +
		`]}`
}

// The works face answers only the blocks a request names, so the fake answers
// only those too: a lane that forgets include=tags,credits has to render the
// bare card here, the way it does in production.
func facetBlocks(item, include string) string {
	blocks := ""
	if strings.Contains(include, "tags") {
		blocks += `,"tags":[{"id":"20","display_name":"北欧神话","source":"vndb","tier":"core","tag_kind":"content","spoiler":"none","is_sexual":false,"work_count":32}]`
	}
	if strings.Contains(include, "credits") {
		blocks += `,"credits":[{"role_key":"scenario","role_name":"剧本","credits":[` + creditJSON("7", "なかひろ", `{}`, "null") + `]}]`
	}
	if blocks == "" {
		return item
	}
	return strings.TrimSuffix(item, "}") + blocks + "}"
}

// One of the four is reached through an imprint, which is the only difference
// between the rollup lane and the plain company filter.
func (f *catalogFake) companyRollup() string {
	via := strings.TrimSuffix(workItem(930, 0, ""), "}") +
		`,"via_company":{"object":"company","id":"77","display_name":"Imprint","localized":{}}}`
	return `{"object":"list","total":4,"items":[` +
		workItem(900, 7, catalogClaimStateLive) + `,` +
		workItem(920, 20, catalogClaimStateDraft) + `,` +
		workItem(921, 21, catalogClaimStateHidden) + `,` +
		via +
		`]}`
}

func (f *catalogFake) calendar() string {
	return `{"object":"list","total":4,"items":[` +
		workItem(900, 7, catalogClaimStateLive) + `,` +
		workItem(920, 20, catalogClaimStateDraft) + `,` +
		workItem(921, 21, catalogClaimStateHidden) + `,` +
		workItem(930, 0, "") +
		`],"meta":{"today":"2026-07-15","min_month":"2020-01","max_month":"2026-12","has_prev":true,"has_next":true}}`
}

func workItem(catalogID int64, gid int, state string) string {
	return `{"object":"work","id":"` + strconv.FormatInt(catalogID, 10) + `","medium":"galgame","display_name":"タイトル","latin":null,` +
		`"content_rating":"` + ratingForCatalogID(catalogID) + `","content_limit":"` + workLimitFor(catalogID) + `","olang":"ja",` +
		`"release_date":"2026-07-14","release_date_precision":"day","release_status":"released",` +
		`"claim":` + claimJSON(gid, state) + `,` +
		`"cover":` + coverSlotJSON("hash1", 600, 800, "vndb") + `,` +
		`"banner":` + coverSlotJSON("hash2", 1280, 720, "vndb") + `,` +
		`"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-07-01T00:00:00Z",` +
		`"localized":{"ja":{"value":"タイトル","is_machine":false},` +
		`"zh-Hans":{"value":"标题","is_machine":false},` +
		`"zh-Hant":{"value":"標題","is_machine":false},` +
		`"en":{"value":"Title","is_machine":true},` +
		`"ko":{"value":"타이틀","is_machine":false}},` +
		`"refs":[{"source":"vndb","external_id":"v42"},{"source":"curated","external_id":"` + strconv.Itoa(gid) + `"}]}`
}

// Every work carries catalog's verdict since spec 2.26.0, claimed or not.
func workLimitFor(catalogID int64) string {
	if gid, _ := gidForCatalogID(catalogID); gid != 0 {
		return gidFixture[gid].limit
	}
	if ratingForCatalogID(catalogID) == "r18" {
		return "nsfw"
	}
	return "sfw"
}

// moyu's claims live under catalog site kungal (catalogv2.SiteKungal), and
// site_work_id is the forum's page number for the work.
func claimJSON(gid int, state string) string {
	if gid == 0 || state == "" {
		return "null"
	}
	return `{"site":"kungal","site_work_id":"` + strconv.Itoa(gid) + `","state":"` + state + `",` +
		`"content_limit":"` + gidFixture[gid].limit + `"}`
}

func gidForCatalogID(id int64) (int, string) {
	for gid, fx := range gidFixture {
		if fx.catalogID == id {
			return gid, fx.state
		}
	}
	return 0, ""
}

func (f *catalogFake) reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reqs = nil
}

func (f *catalogFake) all() []fakeReq {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]fakeReq, len(f.reqs))
	copy(out, f.reqs)
	return out
}

func (f *catalogFake) last() fakeReq {
	reqs := f.all()
	if len(reqs) == 0 {
		return fakeReq{}
	}
	return reqs[len(reqs)-1]
}

func (f *catalogFake) wantPaths(t *testing.T, want ...string) {
	t.Helper()
	got := make([]string, 0, len(f.all()))
	for _, r := range f.all() {
		got = append(got, r.path)
	}
	if strings.Join(got, " → ") != strings.Join(want, " → ") {
		t.Errorf("call sequence = %v, want %v", got, want)
	}
}
