package client

import (
	"encoding/json"
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

func (f *catalogFake) lookupBatch(req *http.Request) string {
	var body catalogLookupBatchRequest
	_ = json.NewDecoder(req.Body).Decode(&body)
	items := make([]string, 0, len(body.Items))
	for _, pair := range body.Items {
		gid, _ := strconv.Atoi(pair.ExternalID)
		fx, ok := gidFixture[gid]
		if !ok {
			items = append(items, `{"source":"galgame_wiki","external_id":"`+pair.ExternalID+`","work":null,"claimed_by":null}`)
			continue
		}
		items = append(items, `{"source":"galgame_wiki","external_id":"`+pair.ExternalID+`",`+
			`"work":{"id":`+strconv.FormatInt(fx.catalogID, 10)+`,"medium":"galgame","display_name":"W","content_rating":"`+ratingForCatalogID(fx.catalogID)+`"},`+
			`"claimed_by":`+claimJSON(gid, fx.state)+`}`)
	}
	return `{"items":[` + strings.Join(items, ",") + `]}`
}

func (f *catalogFake) lookupOne(req *http.Request) string {
	gid, _ := strconv.Atoi(req.URL.Query().Get("external_id"))
	fx, ok := gidFixture[gid]
	if !ok {
		return `{"work":null,"claimed_by":null}`
	}
	return `{"work":{"id":` + strconv.FormatInt(fx.catalogID, 10) + `,"medium":"galgame","display_name":"W","content_rating":"` + ratingForCatalogID(fx.catalogID) + `"},` +
		`"claimed_by":` + claimJSON(gid, fx.state) + `}`
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
	return `{"object":"list","items":[` + strings.Join(items, ",") + `],"next_cursor":null}`
}

func (f *catalogFake) tagRecord(path string) string {
	id, _ := strconv.ParseInt(strings.TrimPrefix(path, "/v2/catalog/tags/"), 10, 64)
	sexual := "false"
	if id == 12 {
		sexual = "true"
	}
	return `{"object":"tag","id":"` + strconv.FormatInt(id, 10) + `","display_name":"tag","tier":"core","tag_kind":"content",` +
		`"is_sexual":` + sexual + `,"work_count":3}`
}

func (f *catalogFake) seriesRecord(path string) string {
	id, _ := strconv.ParseInt(strings.TrimPrefix(path, "/v2/catalog/series/"), 10, 64)
	return `{"object":"series","id":"` + strconv.FormatInt(id, 10) + `","display_name":"Saga",` +
		`"work_count":2,"has_nsfw":true,"intros":[{"lang":"zh-cn","value":"系列简介"}]}`
}

func (f *catalogFake) labelRecord(path string) string {
	id, _ := strconv.ParseInt(strings.TrimPrefix(path, "/v2/catalog/companies/"), 10, 64)
	return `{"object":"company","id":"` + strconv.FormatInt(id, 10) + `","display_name":"Brand","company_kind":"developer","work_count":3}`
}

func (f *catalogFake) workDetail(req *http.Request) string {
	id, _ := strconv.ParseInt(strings.TrimPrefix(req.URL.Path, "/v2/catalog/works/"), 10, 64)
	gid, state := gidForCatalogID(id)
	// credits ride the detail face ONLY behind include=credits; the roster and
	// the ratings are unconditional. Serving credits either way would let the
	// include token be dropped without a test going red.
	credits := `[]`
	if strings.Contains(req.URL.Query().Get("include"), "credits") {
		credits = detailCreditsJSON
	}
	return `{"object":"work","id":"` + strconv.FormatInt(id, 10) + `","medium":"galgame","display_name":"W","olang":"ja",` +
		`"content_rating":"` + ratingForCatalogID(id) + `","release_date":"2026-07-14","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-07-01T00:00:00Z",` +
		`"localized":{"ja":{"value":"タイトル","is_machine":false},` +
		`"zh":{"value":"机翻标题","is_machine":true},` +
		`"zh-Hans":{"value":"标题","is_machine":false},` +
		`"en":{"value":"Title","is_machine":false}},` +
		`"refs":[{"source":"vndb","external_id":"v42"},{"source":"galgame_wiki","external_id":"` + strconv.Itoa(gid) + `"}],` +
		`"claim":` + claimJSON(gid, state) + `,` +
		`"intros":[{"lang":"zh-Hans","value":"介绍","source":"vndb","is_machine":false}],` +
		`"covers":[{"url":"https://cdn/aa/bb/hash1.webp","hash":"hash1","portrait_pinned":true,"sexual":"safe","source":"vndb","width":600,"height":800,"thumbhash":"th"},` +
		`{"url":"https://cdn/aa/bb/hash2.webp","hash":"hash2","portrait_pinned":false,"sexual":"safe","source":"vndb","width":1280,"height":720,"thumbhash":"th2"}],` +
		`"screenshots":[],` +
		`"tags":[{"id":"11","display_name":"純愛","source":"vndb","tier":"core","tag_kind":"content","spoiler":"none","is_sexual":false},` +
		`{"id":"12","display_name":"エロ","source":"vndb","tier":"core","tag_kind":"content","spoiler":"minor","is_sexual":true}],` +
		`"companies":[{"id":"31","display_name":"Brand","company_kind":"game_brand","attribution_role":"developer"}],` +
		`"banner":{"url":"https://cdn/aa/bb/hash2.webp","hash":"hash2","width":1728,"height":1080,"thumbhash":"th2","sexual":"safe","source":"curated"},` +
		`"cover":{"url":"https://cdn/aa/bb/hash3.webp","hash":"hash3","width":850,"height":1080,"thumbhash":"th3","sexual":"safe","source":"upscale"},` +
		`"characters":` + detailCharactersJSON + `,` +
		`"ratings":` + detailRatingsJSON + `,` +
		`"credits":` + credits + `}`
}

// The three blocks below are the prod wire shape of /v1/catalog/works/{id},
// trimmed from a live read of work 3 on 2026-08-19.
const detailCharactersJSON = `[` +
	`{"id":"1699","display_name":"コロナ","localized":{"zh-Hans":{"value":"科罗娜","is_machine":true}},` +
	`"roster_role":"main","spoiler":"none","image":{"url":"https://cdn/aa/bb/chara1.webp","hash":"chara1"},` +
	`"figure":{"url":"https://cdn/aa/bb/figure1.webp","hash":"figure1"}},` +
	`{"id":"1700","display_name":"雪々","localized":{},"roster_role":"secondary","spoiler":"minor"}]`

const detailRatingsJSON = `[` +
	`{"source":"vndb","score":8.1,"vote_count":500,"distribution":[{"score":9,"count":126},{"score":10,"count":38}],"stats":{"average":8.1}},` +
	`{"source":"erogamescape","score":78.5,"vote_count":42,"rank":2917,"distribution":[{"score":70,"count":9},{"score":80,"count":21}]},` +
	`{"source":"dlsite","score":4.6,"vote_count":0}]`

// scenario/剧本 and illustration/原画 are the same role under two source
// vocabularies, developer duplicates the 会社 chips, and 保住圭 rides both a real
// role and other-staff.
const detailCreditsJSON = `[` +
	`{"role_key":"scenario","role_name":"剧本","credits":[{"id":"900","display_name":"保住圭","localized":{}}]},` +
	`{"role_key":"剧本","role_name":"剧本","credits":[{"id":"901","display_name":"丸戸史明","localized":{"zh-Hans":{"value":"丸户史明","is_machine":false}}}]},` +
	`{"role_key":"原画","role_name":"原画","credits":[{"id":"902","display_name":"深崎暮人","localized":{}}]},` +
	`{"role_key":"developer","role_name":"开发","credits":[{"id":"903","display_name":"Brand","localized":{}}]},` +
	`{"role_key":"voice-actor","role_name":"声优","credits":[{"id":"1550","display_name":"榎木実佳","localized":{"zh-Hans":{"value":"榎木实佳","is_machine":false}},"character_id":"1699"}]},` +
	`{"role_key":"other-staff","role_name":"其他","credits":[` +
	`{"id":"900","display_name":"保住圭 (Hozumi Kei)","localized":{}},` +
	`{"id":"904","display_name":"なかひろ","localized":{}}]}]`

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
		blocks += `,"tags":[{"id":"20","display_name":"北欧神话","tier":"core","tag_kind":"content","spoiler":"none","work_count":32}]`
	}
	if strings.Contains(include, "credits") {
		blocks += `,"credits":[{"role_key":"scenario","role_name":"剧本","credits":[{"id":"7","display_name":"なかひろ","localized":{}}]}]`
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
	return `{"object":"list","total":4,"next_cursor":null,"items":[` +
		workItem(900, 7, catalogClaimStateLive) + `,` +
		workItem(920, 20, catalogClaimStateDraft) + `,` +
		workItem(921, 21, catalogClaimStateHidden) + `,` +
		via +
		`]}`
}

func (f *catalogFake) calendar() string {
	return `{"object":"list","total":4,"next_cursor":null,"items":[` +
		workItem(900, 7, catalogClaimStateLive) + `,` +
		workItem(920, 20, catalogClaimStateDraft) + `,` +
		workItem(921, 21, catalogClaimStateHidden) + `,` +
		workItem(930, 0, "") +
		`]}`
}

func workItem(catalogID int64, gid int, state string) string {
	return `{"object":"work","id":"` + strconv.FormatInt(catalogID, 10) + `","medium":"galgame","display_name":"W",` +
		`"content_rating":"` + ratingForCatalogID(catalogID) + `","olang":"ja","release_date":"2026-07-14",` +
		`"claim":` + claimJSON(gid, state) + `,` +
		`"cover":{"url":"https://cdn/aa/bb/hash1.webp","hash":"hash1","width":600,"height":800,"thumbhash":"th","sexual":"safe","source":"vndb"},` +
		`"banner":{"url":"https://cdn/aa/bb/hash2.webp","hash":"hash2","width":1280,"height":720,"thumbhash":"th2","sexual":"safe","source":"vndb"},` +
		`"updated_at":"2026-07-01T00:00:00Z",` +
		`"localized":{"ja":{"value":"タイトル","is_machine":false},` +
		`"zh-Hans":{"value":"标题","is_machine":false},` +
		`"zh-Hant":{"value":"標題","is_machine":false},` +
		`"en":{"value":"Title","is_machine":true},` +
		`"ko":{"value":"타이틀","is_machine":false}},` +
		`"refs":[{"source":"vndb","external_id":"v42"},{"source":"galgame_wiki","external_id":"` + strconv.Itoa(gid) + `"},{"source":"curated","external_id":"` + strconv.Itoa(gid) + `"}]}`
}

func claimJSON(gid int, state string) string {
	if gid == 0 || state == "" {
		return "null"
	}
	return `{"site":"galgame_wiki","site_work_id":"` + strconv.Itoa(gid) + `","state":"` + state + `",` +
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
