package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	"kun-galgame-patch-api/pkg/catalogv2"
	"kun-galgame-patch-api/pkg/catalogv2/catalogv2test"
)

type tagTreeFake struct {
	*httptest.Server
	mu      sync.Mutex
	batches [][]string
	include []string
}

const tagTreeManyChildren = 120

func tagTreeChildren() []string {
	rows := []string{
		`{"object":"tag","id":"40","display_name":"少","is_sexual":false}`,
		`{"object":"tag","id":"41","display_name":"多","is_sexual":false}`,
		`{"object":"tag","id":"42","display_name":"色","is_sexual":true}`,
		`{"object":"tag","id":"43","display_name":"藏","is_sexual":false}`,
	}
	for id := 1000; id < 1000+tagTreeManyChildren; id++ {
		rows = append(rows, fmt.Sprintf(`{"object":"tag","id":"%d","display_name":"t%d","is_sexual":false}`, id, id))
	}
	return rows
}

func tagTreeBatchRow(id string) string {
	works, descendants, sexual, tier := 1, 0, false, "core"
	switch id {
	case "40":
		works, descendants = 5, 2
	case "41":
		works = 90
	case "42":
		works, sexual = 50, true
	case "43":
		works, tier = 500, "hidden"
	}
	return fmt.Sprintf(`{"object":"tag","id":"%s","display_name":"n%s","tier":"%s","tag_kind":"content",`+
		`"is_sexual":%t,"work_count":%d,"descendant_count":%d,"localized":{}}`, id, id, tier, sexual, works, descendants)
}

func newTagTreeFake(t *testing.T) *tagTreeFake {
	t.Helper()
	f := &tagTreeFake{}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		q := r.URL.Query()
		switch r.URL.Path {
		case "/v2/catalog/tags/30":
			f.mu.Lock()
			f.include = append(f.include, q.Get("include"))
			f.mu.Unlock()
			if q.Get("include") == "descendants" {
				_, _ = w.Write([]byte(`{"object":"tag","id":"30","display_name":"根","descendant_count":3,"parents":[],"descendants":[` +
					`{"object":"tag","id":"40","display_name":"少","is_sexual":false,"parent_ids":["30"]},` +
					`{"object":"tag","id":"42","display_name":"色","is_sexual":true,"parent_ids":["30"]},` +
					`{"object":"tag","id":"50","display_name":"孙","is_sexual":false,"parent_ids":["40","42"]}]}`))
				return
			}
			_, _ = w.Write([]byte(`{"object":"tag","id":"30","display_name":"根","tier":"core","tag_kind":"content",` +
				`"is_sexual":false,"work_count":3,"descendant_count":127,"intros":[],` +
				`"parents":[{"object":"tag","id":"20","display_name":"色情","is_sexual":true},{"object":"tag","id":"21","display_name":"剧情","is_sexual":false}],` +
				`"ancestors":[{"object":"tag","id":"20","display_name":"色情","is_sexual":true,"parent_ids":["10"]},` +
				`{"object":"tag","id":"21","display_name":"剧情","is_sexual":false,"parent_ids":[]},` +
				`{"object":"tag","id":"10","display_name":"内容","is_sexual":false,"parent_ids":[]}],` +
				`"children":[` + strings.Join(tagTreeChildren(), ",") + `]}`))
		case "/v2/catalog/tags":
			ids := strings.Split(q.Get("ids"), ",")
			f.mu.Lock()
			f.batches = append(f.batches, ids)
			f.mu.Unlock()
			rows := make([]string, 0, len(ids))
			for _, id := range ids {
				rows = append(rows, tagTreeBatchRow(id))
			}
			_, _ = w.Write([]byte(`{"object":"list","items":[` + strings.Join(rows, ",") + `]}`))
		case "/v2/catalog/tags/31":
			catalogv2test.Problem(w, r, "NOT_FOUND", "no such tag", nil)
		default:
			_, _ = w.Write([]byte(`{"object":"list","items":[],"total":0,"next_cursor":null}`))
		}
	}))
	t.Cleanup(f.Server.Close)
	return f
}

type tagTreeRow struct {
	ID              int64   `json:"id"`
	ParentIDs       []int64 `json:"parent_ids"`
	WorkCount       int     `json:"work_count"`
	DescendantCount int     `json:"descendant_count"`
}

type tagTreePage struct {
	Tag struct {
		DescendantCount int          `json:"descendant_count"`
		Parents         []tagTreeRow `json:"parents"`
		Ancestors       []tagTreeRow `json:"ancestors"`
		Subtags         []tagTreeRow `json:"subtags"`
	} `json:"tag"`
}

func tagTreeBrowse(t *testing.T, c *Client, contentLimit string) tagTreePage {
	t.Helper()
	raw, handled, err := c.TaxonomyBrowse(context.Background(), "/tag/_?tag_id=30&content_limit="+contentLimit, catalogv2.TagExclusion{})
	if err != nil || !handled {
		t.Fatalf("TaxonomyBrowse: handled=%v err=%v", handled, err)
	}
	var page tagTreePage
	if err := json.Unmarshal(raw, &page); err != nil {
		t.Fatal(err)
	}
	return page
}

func rowIDs(rows []tagTreeRow) []int64 {
	out := make([]int64, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.ID)
	}
	return out
}

func TestTagPageCarriesItsPlaceInTheHierarchy(t *testing.T) {
	f := newTagTreeFake(t)
	c := NewWithKey(f.URL, "nm_test_key")

	page := tagTreeBrowse(t, c, "all")
	if page.Tag.DescendantCount != 127 {
		t.Errorf("descendant_count = %d, want catalog's 127", page.Tag.DescendantCount)
	}
	f.mu.Lock()
	include, batches := f.include[0], f.batches
	f.mu.Unlock()
	if got := strings.Split(include, ","); !slices.Contains(got, "children") || !slices.Contains(got, "ancestors") {
		t.Errorf("record include = %q, want children and ancestors named", include)
	}
	if got := rowIDs(page.Tag.Parents); !slices.Equal(got, []int64{20, 21}) {
		t.Errorf("parents = %v", got)
	}
	if got := rowIDs(page.Tag.Ancestors); !slices.Equal(got, []int64{20, 21, 10}) {
		t.Errorf("ancestors = %v, want catalog's nearest-first order", got)
	}
	if got := page.Tag.Ancestors[0].ParentIDs; !slices.Equal(got, []int64{10}) {
		t.Errorf("ancestor parent_ids = %v", got)
	}
	if got := page.Tag.Ancestors[1].ParentIDs; got == nil || len(got) != 0 {
		t.Errorf("a root ancestor's parent_ids = %#v, want an empty array", got)
	}

	subs := page.Tag.Subtags
	if got := rowIDs(subs[:3]); !slices.Equal(got, []int64{41, 42, 40}) {
		t.Errorf("subtags lead with %v, want most works first", got)
	}
	if len(subs) != 3+tagTreeManyChildren {
		t.Errorf("%d subtags, want every child but the hidden one", len(subs))
	}
	if subs[2].WorkCount != 5 || subs[2].DescendantCount != 2 {
		t.Errorf("subtag counts = %+v", subs[2])
	}
	if len(batches) != 2 || len(batches[0]) != 100 {
		t.Errorf("%d batch requests, first %d ids; want the children split at catalog's 100-id cap",
			len(batches), len(batches[0]))
	}

	sfw := tagTreeBrowse(t, c, "sfw")
	if got := rowIDs(sfw.Tag.Parents); !slices.Equal(got, []int64{21}) {
		t.Errorf("sfw parents = %v, want the adult parent left out", got)
	}
	if got := rowIDs(sfw.Tag.Ancestors); !slices.Equal(got, []int64{20, 21, 10}) {
		t.Errorf("sfw ancestors = %v, want adult ones kept: a block on them still hides this tag", got)
	}
	if slices.Contains(rowIDs(sfw.Tag.Subtags), 42) {
		t.Errorf("sfw subtags carry the adult child: %v", rowIDs(sfw.Tag.Subtags)[:3])
	}
}

func TestTagDescendantsAreTheWholeSubtree(t *testing.T) {
	f := newTagTreeFake(t)
	c := NewWithKey(f.URL, "nm_test_key")

	nodes, err := c.TagDescendants(context.Background(), 30)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(nodes)
	var rows []tagTreeRow
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	if got := rowIDs(rows); !slices.Equal(got, []int64{40, 42, 50}) {
		t.Errorf("descendants = %v, want adult ones too: a tree block covers them", got)
	}
	if got := rows[2].ParentIDs; !slices.Equal(got, []int64{40, 42}) {
		t.Errorf("parent_ids = %v", got)
	}

	if _, err := c.TagDescendants(context.Background(), 31); !catalogv2.IsNotFound(err) {
		t.Errorf("an unknown tag: err = %v, want not found", err)
	}
}

func TestTagResolveCarriesSubtreeSizeAndAdultFlag(t *testing.T) {
	f := newTagTreeFake(t)
	c := NewWithKey(f.URL, "nm_test_key")

	items, err := c.ResolveEntities(context.Background(), EntityFamilyTag, []int{40, 42})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(items)
	for _, want := range []string{
		`"id":40,"family":"tag"`, `"descendant_count":2`, `"is_sexual":true`,
	} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("wire %s lacks %s", raw, want)
		}
	}
}
