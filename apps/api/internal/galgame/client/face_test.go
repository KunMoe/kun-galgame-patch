package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
)

type faceRecorder struct {
	mu     sync.Mutex
	path   string
	query  url.Values
	apiKey string
	auth   string
}

func (r *faceRecorder) server(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r.mu.Lock()
		r.path = req.URL.Path
		r.query = req.URL.Query()
		r.apiKey = req.Header.Get("X-API-Key")
		r.auth = req.Header.Get("Authorization")
		r.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{}}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestFaceSelection_WithKey(t *testing.T) {
	rec := &faceRecorder{}
	srv := rec.server(t)
	c := NewWithKey(srv.URL, "nm_test_key")
	ctx := context.Background()

	t.Run("anonymous calendar read → v1 catalog + key", func(t *testing.T) {
		if _, err := c.GetGalgameCalendar(ctx, "2026-07", ""); err != nil {
			t.Fatalf("GetGalgameCalendar: %v", err)
		}
		if rec.path != "/v2/catalog/calendar" {
			t.Errorf("path = %q, want /v2/catalog/calendar", rec.path)
		}
		if rec.auth != "Bearer nm_test_key" {
			t.Errorf("Authorization = %q, want Bearer nm_test_key", rec.auth)
		}
		if rec.apiKey != "" {
			t.Errorf("X-API-Key = %q, want empty", rec.apiKey)
		}
	})

}

func TestV1ReadRouting(t *testing.T) {
	rec := &faceRecorder{}
	srv := rec.server(t)
	c := NewWithKey(srv.URL, "nm_test_key")
	ctx := context.Background()

	check := func(t *testing.T, wantPath string, call func() error) {
		t.Helper()
		if err := call(); err != nil {
			t.Fatalf("call: %v", err)
		}
		if rec.path != wantPath {
			t.Errorf("path = %q, want %q", rec.path, wantPath)
		}
		if rec.auth != "Bearer nm_test_key" {
			t.Errorf("Authorization = %q, want Bearer nm_test_key", rec.auth)
		}
	}

	t.Run("search → v1 catalog works search", func(t *testing.T) {
		check(t, "/v2/catalog/works", func() error { _, e := c.SearchGalgame(ctx, SearchGalgameParams{Q: "x"}); return e })
	})
	t.Run("calendar → v2 catalog", func(t *testing.T) {
		check(t, "/v2/catalog/calendar", func() error { _, e := c.GetGalgameCalendar(ctx, "", ""); return e })
	})
	t.Run("vndb lookup → v2 works refs", func(t *testing.T) {
		check(t, "/v2/catalog/works", func() error { _, _, e := c.CheckGalgameByVndbID(ctx, "v1"); return e })
		if got := rec.query.Get("refs"); got != "vndb:v1" {
			t.Errorf("refs = %q, want vndb:v1", got)
		}
		if got := rec.query.Get("nsfw"); got != "true" {
			t.Errorf("nsfw = %q, want true", got)
		}
	})

}

func TestCatalogTwoHopReads(t *testing.T) {
	srv := newCatalogFake(t)
	c := NewWithKey(srv.URL, "nm_test_key")
	ctx := context.Background()

	// One request, not two. The batch used to resolve every gid through a
	// `curated` ref lookup first, because a page id and a catalog work id were
	// different numbers; migration 037 made them the same one.
	t.Run("batch = works?ids= in one hop", func(t *testing.T) {
		srv.reset()
		briefs, err := c.GalgameBatch(ctx, []int{7}, "")
		if err != nil {
			t.Fatalf("GalgameBatch: %v", err)
		}
		srv.wantPaths(t, "/v2/catalog/works")
		last := srv.last()
		if got := last.query.Get("ids"); got != "7" {
			t.Errorf("ids = %q, want the page id 7 sent as the catalog id", got)
		}
		if got, want := last.query.Get("include"), strings.Join(cardInclude, ","); got != want {
			t.Errorf("include = %q, want %q", got, want)
		}
		if len(briefs) != 1 || briefs[0].ID != 7 {
			t.Fatalf("briefs = %+v, want one row keyed by 7", briefs)
		}
		if briefs[0].VndbID != "v42" {
			t.Errorf("vndb_id = %q, want v42 (from include=refs)", briefs[0].VndbID)
		}
		if briefs[0].ForumGID != 7 {
			t.Errorf("forum_gid = %d, want the claim's site_work_id 7", briefs[0].ForumGID)
		}
	})

	t.Run("detail = works/{id} in one hop", func(t *testing.T) {
		srv.reset()
		env, err := c.GetGalgame(ctx, 8, "")
		if err != nil {
			t.Fatalf("GetGalgame: %v", err)
		}
		srv.wantPaths(t, "/v2/catalog/works/8")
		if env.Galgame.ID != 8 {
			t.Errorf("detail id = %d, want the gid 8", env.Galgame.ID)
		}
		if got := env.Galgame.IntroZhCn; got != "介绍" {
			t.Errorf("intro_zh_cn = %q, want 介绍", got)
		}
		// v2 answers a bare id+name row for every block the request does not
		// name, and its spoiler ceiling defaults to none. The page renders empty
		// rather than erroring, so only the request itself can be asserted.
		q := srv.last().query
		for _, want := range []string{"intros", "tags", "companies", "characters", "credits", "ratings", "covers", "screenshots", "series"} {
			if !slices.Contains(strings.Split(q.Get("include"), ","), want) {
				t.Errorf("detail include = %q, want %s among them", q.Get("include"), want)
			}
		}
		if got := q.Get("spoiler"); got != "major" {
			t.Errorf("spoiler = %q, want major — none hides every spoilered tag", got)
		}
	})

	t.Run("search hits the works product search", func(t *testing.T) {
		srv.reset()
		if _, err := c.SearchGalgame(ctx, SearchGalgameParams{Q: "x", ContentLimit: "sfw"}); err != nil {
			t.Fatalf("SearchGalgame: %v", err)
		}
		srv.wantPaths(t, "/v2/catalog/works")
		if got := srv.last().query.Get("nsfw"); got != "true" {
			t.Errorf("nsfw = %q, want true — moyu always reads the whole population", got)
		}
		if got := srv.last().query.Get("search_intro"); got != "" {
			t.Errorf("search_intro = %q, want absent unless asked for", got)
		}
	})

	t.Run("search_intro rides the wire when asked for", func(t *testing.T) {
		srv.reset()
		_, err := c.SearchGalgame(ctx, SearchGalgameParams{Q: "x", SearchIntro: true})
		if err != nil {
			t.Fatalf("SearchGalgame: %v", err)
		}
		if got := srv.last().query.Get("search_intro"); got != "true" {
			t.Errorf("search_intro = %q, want true", got)
		}
	})
}

func TestContentLimitCaliber(t *testing.T) {
	srv := newCatalogFake(t)
	c := NewWithKey(srv.URL, "nm_test_key")
	ctx := context.Background()

	t.Run("the three values ride the wire as the editing axis", func(t *testing.T) {
		for _, tc := range []struct {
			cl        string
			wantLimit string
		}{
			{"sfw", "sfw"},
			{"nsfw", "nsfw"},
			{"all", ""},
			{"", ""},
		} {
			srv.reset()
			if _, err := c.SearchGalgame(ctx, SearchGalgameParams{Q: "x", ContentLimit: tc.cl}); err != nil {
				t.Fatalf("SearchGalgame(%q): %v", tc.cl, err)
			}
			q := srv.last().query
			if got := q.Get("nsfw"); got != "true" {
				t.Errorf("content_limit=%q: nsfw = %q, want true unconditionally", tc.cl, got)
			}
			if got := q.Get("content_limit"); got != tc.wantLimit {
				t.Errorf("content_limit=%q: wire content_limit = %q, want %q", tc.cl, got, tc.wantLimit)
			}
			if got := q.Get("content_rating"); got != "" {
				t.Errorf("content_limit=%q: content_rating = %q, want absent — that is the AGE axis", tc.cl, got)
			}
		}
	})

	t.Run("age_limit is the only content_rating caller", func(t *testing.T) {
		srv.reset()
		if _, err := c.SearchGalgame(ctx, SearchGalgameParams{Q: "x", AgeLimit: "r18", ContentLimit: "sfw"}); err != nil {
			t.Fatalf("SearchGalgame: %v", err)
		}
		q := srv.last().query
		if got := q.Get("content_rating"); got != "r18" {
			t.Errorf("content_rating = %q, want r18", got)
		}
		if got := q.Get("content_limit"); got != "sfw" {
			t.Errorf("content_limit = %q, want sfw — the two axes compose", got)
		}
	})

	t.Run("batch and calendar carry the same gate", func(t *testing.T) {
		srv.reset()
		if _, err := c.GalgameBatch(ctx, []int{7}, "sfw"); err != nil {
			t.Fatalf("GalgameBatch: %v", err)
		}
		q := srv.last().query
		if q.Get("nsfw") != "true" || q.Get("content_limit") != "sfw" {
			t.Errorf("works list gate = nsfw %q / content_limit %q, want true / sfw", q.Get("nsfw"), q.Get("content_limit"))
		}

		srv.reset()
		if _, err := c.GetGalgameCalendar(ctx, "2026-07", "nsfw"); err != nil {
			t.Fatalf("GetGalgameCalendar: %v", err)
		}
		q = srv.last().query
		if q.Get("nsfw") != "true" {
			t.Errorf("calendar nsfw = %q, want true", q.Get("nsfw"))
		}
	})

	t.Run("the rendered content_limit is the work's, not the rating's", func(t *testing.T) {
		srv.reset()
		briefs, err := c.GalgameBatch(ctx, []int{22}, "")
		if err != nil {
			t.Fatalf("GalgameBatch: %v", err)
		}
		if len(briefs) != 1 {
			t.Fatalf("briefs = %+v, want the one live row", briefs)
		}
		if got := briefs[0].ContentLimit; got != "sfw" {
			t.Errorf("content_limit = %q, want sfw (work.content_limit)", got)
		}
		if got := briefs[0].AgeLimit; got != "r18" {
			t.Errorf("age_limit = %q, want r18 — the AGE axis is untouched", got)
		}
	})

	t.Run("detail gates the row it fetched", func(t *testing.T) {
		srv.reset()
		if _, err := c.GetGalgame(ctx, 22, "sfw"); err != nil {
			t.Fatalf("GetGalgame(sfw) on an sfw-edited entry: %v", err)
		}
		q := srv.last().query
		if got := q.Get("nsfw"); got != "true" {
			t.Errorf("detail nsfw = %q, want true — an r18-rated entry 404s without it", got)
		}
		if got := q.Get("content_limit"); got != "" {
			t.Errorf("detail content_limit = %q, want absent — that face has no such parameter", got)
		}
		if _, err := c.GetGalgame(ctx, 22, "nsfw"); err == nil {
			t.Error("GetGalgame(nsfw) on an sfw-edited entry: want not-found, got nil")
		}
		if _, err := c.GetGalgame(ctx, 22, ""); err != nil {
			t.Errorf("GetGalgame(no filter): %v", err)
		}
	})
}

func TestContentAxisProjection(t *testing.T) {
	for _, tc := range []struct {
		name            string
		verdict         string
		rating          string
		wantCL, wantAge string
	}{
		{"sfw verdict, rated r18", "sfw", "r18", "sfw", "r18"},
		{"nsfw verdict, rated all_ages", "nsfw", "all_ages", "nsfw", "all"},
		{"no verdict is nsfw whatever the rating", "", "all_ages", "nsfw", "all"},
		{"an unknown verdict is nsfw", "sensitive", "all_ages", "nsfw", "all"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cl, age := contentAxisOf(tc.verdict, tc.rating)
			if cl != tc.wantCL || age != tc.wantAge {
				t.Errorf("contentAxisOf = (%q, %q), want (%q, %q)", cl, age, tc.wantCL, tc.wantAge)
			}
		})
	}
}

func TestEffectiveBannerPrefersLandscape(t *testing.T) {
	t.Run("list rows take the banner slot", func(t *testing.T) {
		portrait := &catalogCoverSlot{URL: "https://cdn/aa/bb/p.webp", Width: 600, Height: 800}
		banner := &catalogCoverSlot{URL: "https://cdn/aa/bb/b.webp", Width: 1280, Height: 720}
		for _, tc := range []struct {
			name string
			it   catalogWorkListItem
			want string
		}{
			{"both slots", catalogWorkListItem{Covers: &catalogCoverSlots{Portrait: portrait, Banner: banner}}, "b"},
			{"portrait only", catalogWorkListItem{Covers: &catalogCoverSlots{Portrait: portrait}}, "p"},
			{"no include=covers", catalogWorkListItem{Cover: "https://cdn/aa/bb/c.webp"}, "c"},
			{"no cover at all", catalogWorkListItem{}, ""},
		} {
			t.Run(tc.name, func(t *testing.T) {
				hash, _, _, _ := coverOf(&tc.it)
				if hash != tc.want {
					t.Errorf("coverOf hash = %q, want %q", hash, tc.want)
				}
			})
		}
	})

	t.Run("the detail hero takes the first landscape row", func(t *testing.T) {
		pinned := catalogDetailCover{URL: "https://cdn/aa/bb/p.webp", PortraitPinned: true, Width: 600, Height: 800}
		wide := catalogDetailCover{URL: "https://cdn/aa/bb/b.webp", Width: 1280, Height: 720}
		unknown := catalogDetailCover{URL: "https://cdn/aa/bb/u.webp"}
		for _, tc := range []struct {
			name   string
			covers []catalogDetailCover
			want   string
		}{
			{"landscape beats the portrait pin", []catalogDetailCover{pinned, wide}, "https://cdn/aa/bb/b.webp"},
			{"portrait-only falls back to the pin", []catalogDetailCover{unknown, pinned}, "https://cdn/aa/bb/p.webp"},
			{"nothing pinned falls back to the first", []catalogDetailCover{unknown}, "https://cdn/aa/bb/u.webp"},
			{"no covers", nil, ""},
		} {
			t.Run(tc.name, func(t *testing.T) {
				got := ""
				if c := heroCover(tc.covers); c != nil {
					got = c.URL
				}
				if got != tc.want {
					t.Errorf("heroCover = %q, want %q", got, tc.want)
				}
			})
		}
	})
}

func TestTagPageCarriesSafetyFlag(t *testing.T) {
	srv := newCatalogFake(t)
	c := NewWithKey(srv.URL, "nm_test_key")
	ctx := context.Background()

	for _, tc := range []struct {
		name         string
		tagID        string
		wantSexual   bool
		wantCategory string
	}{
		{"a sexual tag", "12", true, "sexual"},
		{"an ordinary tag", "11", false, "content"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv.reset()
			raw, handled, err := c.TaxonomyBrowse(ctx, "/tag/_?tag_id="+tc.tagID)
			if err != nil || !handled {
				t.Fatalf("TaxonomyBrowse: handled=%v err=%v", handled, err)
			}
			var got struct {
				Tag struct {
					Sexual   bool   `json:"sexual"`
					Category string `json:"category"`
				} `json:"tag"`
			}
			if e := json.Unmarshal(raw, &got); e != nil {
				t.Fatalf("unmarshal: %v", e)
			}
			if got.Tag.Sexual != tc.wantSexual {
				t.Errorf("sexual = %v, want %v", got.Tag.Sexual, tc.wantSexual)
			}
			if got.Tag.Category != tc.wantCategory {
				t.Errorf("category = %q, want %q (derived from the same flag)", got.Tag.Category, tc.wantCategory)
			}
		})
	}
}

func TestClaimStateGating(t *testing.T) {
	srv := newCatalogFake(t)
	c := NewWithKey(srv.URL, "nm_test_key")
	ctx := context.Background()

	t.Run("batch drops hidden, keeps draft", func(t *testing.T) {
		srv.reset()
		briefs, err := c.GalgameBatch(ctx, []int{7, 20, 21}, "")
		if err != nil {
			t.Fatalf("GalgameBatch: %v", err)
		}
		got := make([]int, 0, len(briefs))
		for _, b := range briefs {
			got = append(got, b.ID)
		}
		if len(got) != 2 || !slices.Contains(got, 7) || !slices.Contains(got, 20) || slices.Contains(got, 21) {
			t.Fatalf("briefs = %v, want live 7 and draft 20, no hidden 21", got)
		}
	})

	t.Run("detail 404s a hidden entry", func(t *testing.T) {
		srv.reset()
		if _, err := c.GetGalgame(ctx, 21, ""); err == nil {
			t.Fatal("GetGalgame on a hidden entry: want an error, got nil")
		}
	})

	t.Run("detail 200s an unclaimed catalog id", func(t *testing.T) {
		srv.reset()
		env, err := c.GetGalgame(ctx, 930, "")
		if err != nil {
			t.Fatalf("GetGalgame on an unclaimed work: %v", err)
		}
		if env.Galgame.ID != 930 {
			t.Errorf("id = %d, want the catalog id 930", env.Galgame.ID)
		}
	})

	t.Run("calendar keeps draft, drops hidden", func(t *testing.T) {
		srv.reset()
		cal, err := c.GetGalgameCalendar(ctx, "2026-07", "")
		if err != nil {
			t.Fatalf("GetGalgameCalendar: %v", err)
		}
		var states []string
		for _, it := range cal.Items {
			states = append(states, it.ClaimState)
		}
		if len(states) != 3 {
			t.Fatalf("claim states = %v, want three rows (live, draft, unclaimed)", states)
		}
		for _, s := range states {
			if s == "hidden" {
				t.Errorf("claim states = %v, a hidden entry must never be rendered", states)
			}
		}
		if cal.Meta.PrevMonth != "2026-06" || cal.Meta.NextMonth != "2026-08" {
			t.Errorf("prev/next = %q/%q, want 2026-06/2026-08", cal.Meta.PrevMonth, cal.Meta.NextMonth)
		}
	})

	t.Run("search does not send claim_state and drops hidden rows", func(t *testing.T) {
		srv.reset()
		res, err := c.SearchGalgame(ctx, SearchGalgameParams{Q: "x"})
		if err != nil {
			t.Fatalf("SearchGalgame: %v", err)
		}
		if got := srv.last().query.Get("claim_state"); got != "" {
			t.Fatalf("claim_state = %q, want it absent — the public library is the catalog", got)
		}
		ids := make([]int, 0, len(res.Items))
		for _, it := range res.Items {
			ids = append(ids, it.ID)
			if it.ClaimState == "hidden" {
				t.Errorf("search returned a hidden row %d", it.ID)
			}
		}
		if len(ids) != 3 {
			t.Errorf("items = %v, want live + draft + unclaimed", ids)
		}
		if res.Total != 4 {
			t.Errorf("total = %d, want the face's 4 verbatim", res.Total)
		}
	})

	t.Run("claim states resolve in one hop", func(t *testing.T) {
		srv.reset()
		states, err := c.ClaimStates(ctx, []int{7, 20, 21})
		if err != nil {
			t.Fatalf("ClaimStates: %v", err)
		}
		srv.wantPaths(t, "/v2/catalog/works")
		want := map[int]string{7: "live", 20: "draft", 21: "hidden"}
		for gid, w := range want {
			if states[gid] != w {
				t.Errorf("state[%d] = %q, want %q", gid, states[gid], w)
			}
		}
	})
}

func TestTaxonomyBrowseMembersAreGated(t *testing.T) {
	srv := newCatalogFake(t)
	c := NewWithKey(srv.URL, "nm_test_key")
	ctx := context.Background()

	for _, tc := range []struct {
		name          string
		path          string
		recPath       string
		filterKey     string
		filterVal     string
		entity        string
		recordInclude []string
	}{
		{"tag page", "/tag/_?tag_id=11&page=2&limit=10", "/v2/catalog/tags/11", "tag_id", "11", "tag", []string{"intros"}},
		{"series page", "/series/_?series_id=11&page=2&limit=10", "/v2/catalog/series/11", "series_id", "11", "series", []string{"intros"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv.reset()
			raw, handled, err := c.TaxonomyBrowse(ctx, tc.path)
			if err != nil || !handled {
				t.Fatalf("TaxonomyBrowse: handled=%v err=%v", handled, err)
			}
			srv.wantPaths(t, tc.recPath, "/v2/catalog/works")
			// The record and the member list are two requests with two include
			// vocabularies: the page's header renders from the record's blocks,
			// which v2 answers without unless they are named.
			for _, want := range tc.recordInclude {
				if !slices.Contains(strings.Split(srv.all()[0].query.Get("include"), ","), want) {
					t.Errorf("record include = %q, want %s among them",
						srv.all()[0].query.Get("include"), want)
				}
			}

			q := srv.last().query
			if got := q.Get(tc.filterKey); got != tc.filterVal {
				t.Errorf("%s = %q, want %q", tc.filterKey, got, tc.filterVal)
			}
			if got := q.Get("claim_state"); got != "" {
				t.Errorf("claim_state = %q, want it absent — a public browse page is catalog membership", got)
			}
			if got := q.Get("cursor"); got == "" {
				t.Errorf("cursor = %q, want a page-2 cursor", got)
			}
			if got, want := q.Get("limit"), "10"; got != want {
				t.Errorf("limit = %q, want %q", got, want)
			}
			if got := q.Get("sort"); got != "released_desc" {
				t.Errorf("sort = %q, want released_desc — ?page=N needs a deterministic order", got)
			}
			if got, want := q.Get("include"), strings.Join(listCardInclude, ","); got != want {
				t.Errorf("member include = %q, want %q", got, want)
			}

			var got struct {
				Total    int64 `json:"total"`
				Galgames []struct {
					ID int `json:"id"`
				} `json:"galgames"`
			}
			if e := json.Unmarshal(raw, &got); e != nil {
				t.Fatalf("unmarshal: %v", e)
			}
			var envelope map[string]json.RawMessage
			if e := json.Unmarshal(raw, &envelope); e != nil {
				t.Fatalf("unmarshal envelope: %v", e)
			}
			var record struct {
				GalgameCount int `json:"galgame_count"`
			}
			if e := json.Unmarshal(envelope[tc.entity], &record); e != nil {
				t.Fatalf("unmarshal %s: %v", tc.entity, e)
			}
			if len(got.Galgames) != 3 {
				t.Errorf("galgames = %d, want 3 — hidden claims drop on the public page", len(got.Galgames))
			}
			if got.Total != 4 {
				t.Errorf("total = %d, want 4 (the gated search's), not the record's 3", got.Total)
			}
			if record.GalgameCount != 4 {
				t.Errorf("%s.galgame_count = %d, want 4 — the header counts the list below it",
					tc.entity, record.GalgameCount)
			}
		})
	}
}

// The company page is the one taxonomy face that must not use the search lane:
// catalog only honours company_rollup on the keyset lane, and sending a sort or
// a facet silently drops it — VISUAL ARTS answered 19 of its 540 works.
func TestCompanyMembersWalkTheRollup(t *testing.T) {
	srv := newCatalogFake(t)
	c := NewWithKey(srv.URL, "nm_test_key")

	raw, handled, err := c.TaxonomyBrowse(context.Background(), "/official/_?official_id=31&page=1&limit=10")
	if err != nil || !handled {
		t.Fatalf("TaxonomyBrowse: handled=%v err=%v", handled, err)
	}
	srv.wantPaths(t, "/v2/catalog/companies/31", "/v2/catalog/works", "/v2/catalog/works")

	q := srv.all()[1].query
	if q.Get("company_id") != "31" || q.Get("company_rollup") != "true" {
		t.Errorf("company_id=%q company_rollup=%q, want 31/true", q.Get("company_id"), q.Get("company_rollup"))
	}
	if got := q.Get("sort"); got != "" {
		t.Errorf("sort = %q, want it absent — any sort flips catalog to the search lane", got)
	}
	if got := q.Get("facets"); got != "" {
		t.Errorf("facets = %q, want it absent — a facet flips catalog to the search lane", got)
	}
	// The walk stays lean and the shelf is read once for the page it slices:
	// this roster carries every work the company ever shipped, and VISUAL ARTS
	// would have paid for 540 works' credits to print 10.
	if got, want := q.Get("include"), strings.Join(cardInclude, ","); got != want {
		t.Errorf("rollup include = %q, want %q", got, want)
	}
	if got, want := srv.last().query.Get("include"), strings.Join(facetInclude, ","); got != want {
		t.Errorf("shelf include = %q, want %q", got, want)
	}
	if got := srv.last().query.Get("ids"); strings.Count(got, ",") != 2 {
		t.Errorf("shelf ids = %q, want only the three works this page renders", got)
	}

	var got struct {
		Total    int64 `json:"total"`
		Galgames []struct {
			ID int `json:"id"`
		} `json:"galgames"`
		Official struct {
			GalgameCount int `json:"galgame_count"`
			Own          int `json:"own_galgame_count"`
			Imprint      int `json:"imprint_galgame_count"`
		} `json:"official"`
	}
	if e := json.Unmarshal(raw, &got); e != nil {
		t.Fatalf("unmarshal: %v", e)
	}
	if len(got.Galgames) != 3 || got.Total != 3 {
		t.Errorf("galgames=%d total=%d, want 3/3 — hidden claims drop, the rest are one page",
			len(got.Galgames), got.Total)
	}
	if got.Official.GalgameCount != 3 || got.Official.Own != 2 || got.Official.Imprint != 1 {
		t.Errorf("counts = %d (own %d, imprint %d), want 3 (2, 1)",
			got.Official.GalgameCount, got.Official.Own, got.Official.Imprint)
	}
}

func TestLocalizedNamesReachBothFaces(t *testing.T) {
	srv := newCatalogFake(t)
	c := NewWithKey(srv.URL, "nm_test_key")
	ctx := context.Background()

	t.Run("list brief folds the catalog tags onto the product columns", func(t *testing.T) {
		raw, handled, err := c.TaxonomyBrowse(ctx, "/tag/_?tag_id=11")
		if err != nil || !handled {
			t.Fatalf("TaxonomyBrowse: handled=%v err=%v", handled, err)
		}
		var got struct {
			Galgames []struct {
				NameJaJp string `json:"name_ja_jp"`
				NameZhCn string `json:"name_zh_cn"`
				NameZhTw string `json:"name_zh_tw"`
				NameEnUs string `json:"name_en_us"`
			} `json:"galgames"`
		}
		if e := json.Unmarshal(raw, &got); e != nil {
			t.Fatalf("unmarshal: %v", e)
		}
		if len(got.Galgames) == 0 {
			t.Fatal("no galgames — the fixture has four")
		}
		g := got.Galgames[0]
		// zh-Hant lands on zh-tw, and ko has nowhere to go and must not break anything.
		if g.NameJaJp != "タイトル" || g.NameZhCn != "标题" ||
			g.NameZhTw != "標題" || g.NameEnUs != "Title" {
			t.Errorf("names = (%q, %q, %q, %q), want (タイトル, 标题, 標題, Title)",
				g.NameJaJp, g.NameZhCn, g.NameZhTw, g.NameEnUs)
		}
	})

	t.Run("a source tag beats a machine tag folding onto the same column", func(t *testing.T) {
		full, err := c.GetGalgame(ctx, 7, "")
		if err != nil {
			t.Fatalf("GetGalgame: %v", err)
		}
		// `zh` and `zh-Hans` both fold to zh-cn; `zh` sorts first and is the
		// machine row, so lowest-tag-wins alone would publish the translation.
		if got := full.Galgame.NameZhCn; got != "标题" {
			t.Errorf("name_zh_cn = %q, want 标题 (the machine `zh` row must lose)", got)
		}
	})
}

func TestLabelAliasRowsFlattenToValues(t *testing.T) {
	srv := newCatalogFake(t)
	c := NewWithKey(srv.URL, "nm_test_key")

	raw, handled, err := c.TaxonomyBrowse(context.Background(), "/official/_?official_id=31")
	if err != nil || !handled {
		t.Fatalf("TaxonomyBrowse: handled=%v err=%v", handled, err)
	}
	var got struct {
		Official struct {
			Aliases []string `json:"aliases"`
		} `json:"official"`
	}
	if e := json.Unmarshal(raw, &got); e != nil {
		t.Fatalf("unmarshal: %v", e)
	}
	if got.Official.Aliases == nil {
		t.Errorf("official.aliases = nil, want []")
	}
}

func TestLabelLogoHashReachesBothFaces(t *testing.T) {
	srv := newCatalogFake(t)
	c := NewWithKey(srv.URL, "nm_test_key")
	ctx := context.Background()

	t.Run("official browse header", func(t *testing.T) {
		srv.reset()
		raw, handled, err := c.TaxonomyBrowse(ctx, "/official/_?official_id=31")
		if err != nil || !handled {
			t.Fatalf("TaxonomyBrowse: handled=%v err=%v", handled, err)
		}
		var got struct {
			Official struct {
				LogoHash string `json:"logo_hash"`
			} `json:"official"`
		}
		if e := json.Unmarshal(raw, &got); e != nil {
			t.Fatalf("unmarshal: %v", e)
		}
		if got.Official.LogoHash != "" {
			t.Errorf("official.logo_hash = %q, v2 company detail has no logo", got.Official.LogoHash)
		}
	})

	t.Run("work detail label", func(t *testing.T) {
		srv.reset()
		full, err := c.GetGalgame(ctx, 7, "")
		if err != nil {
			t.Fatalf("GetGalgame: %v", err)
		}
		if len(full.Galgame.Official) == 0 {
			t.Fatal("detail carries no official — the fixture has one")
		}
		if got := full.Galgame.Official[0].Official.Name; got != "Brand" {
			t.Errorf("official[0].name = %q, want Brand", got)
		}
	})
}

func TestDeprecatedGalgameFaceIsUnreachable(t *testing.T) {
	srv := newCatalogFake(t)
	c := NewWithKey(srv.URL, "nm_test_key")
	ctx := context.Background()

	srv.reset()
	_, _ = c.GalgameBatch(ctx, []int{7}, "")
	_, _ = c.GetGalgame(ctx, 8, "")
	_, _ = c.GetGalgameCalendar(ctx, "2026-07", "")
	_, _ = c.SearchGalgame(ctx, SearchGalgameParams{Q: "x"})
	_, _, _ = c.CheckGalgameByVndbID(ctx, "v1")
	_, _, _ = c.TaxonomyBrowse(ctx, "/tag/_?tag_id=5")
	_, _, _ = c.TaxonomyBrowse(ctx, "/official/_?official_id=9")

	for _, r := range srv.all() {
		if strings.HasPrefix(r.path, "/v1/galgame") {
			t.Errorf("read dialed the DEPRECATED face %q — the whole point of wave A2-2 is that nothing does", r.path)
		}
	}
}

func TestRetiredTaxonomyListsAreUnhandled(t *testing.T) {
	rec := &faceRecorder{}
	srv := rec.server(t)
	c := NewWithKey(srv.URL, "nm_test_key")
	ctx := context.Background()

	for _, p := range []string{
		"/tag", "/tag?page=1", "/official", "/official?page=1",
		"/tag/search?q=x", "/official/search?q=x", "/engine", "/series?page=1",
		"/taxonomy/tag/42",
	} {
		rec.path = ""
		if _, handled, err := c.TaxonomyBrowse(ctx, p); handled || err != nil {
			t.Errorf("TaxonomyBrowse(%q) = handled %v, err %v; want handled=false, err=nil", p, handled, err)
		}
		if rec.path != "" {
			t.Errorf("TaxonomyBrowse(%q) dialed %q; a retired lane must not reach any face", p, rec.path)
		}
	}
}
