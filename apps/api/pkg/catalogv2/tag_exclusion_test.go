package catalogv2_test

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"kun-galgame-patch-api/pkg/catalogv2"
)

func TestParseTagExclusionKeepsOnlyIDsInOneOrder(t *testing.T) {
	got, err := catalogv2.ParseTagExclusion(" 9, 3,abc,0,-2,3 ", "12,5,")
	if err != nil {
		t.Fatal(err)
	}
	want := catalogv2.TagExclusion{Exact: []int64{3, 9}, Trees: []int64{5, 12}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseTagExclusion = %+v, want %+v", got, want)
	}
	if empty, _ := catalogv2.ParseTagExclusion("", ""); !empty.Empty() {
		t.Fatalf("no parameters parsed as %+v, want an empty exclusion", empty)
	}
}

func TestParseTagExclusionRefusesMoreThanCatalogTakes(t *testing.T) {
	ids := make([]string, catalogv2.TagExclusionMax+1)
	for i := range ids {
		ids[i] = strconv.Itoa(i + 1)
	}
	if _, err := catalogv2.ParseTagExclusion("", strings.Join(ids, ",")); !errors.Is(err, catalogv2.ErrTooManyExcludedTags) {
		t.Fatalf("err = %v, want ErrTooManyExcludedTags", err)
	}
	if _, err := catalogv2.ParseTagExclusion(strings.Join(ids[:catalogv2.TagExclusionMax], ","), ""); err != nil {
		t.Fatalf("exactly %d ids: %v", catalogv2.TagExclusionMax, err)
	}
}

func TestWorkListsSpellTheExclusionInCatalogsNames(t *testing.T) {
	var seen []url.Values
	c, _ := cachedClient(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.URL.Query())
		emptyWorkList(w)
	})
	ctx := context.Background()
	exclude := catalogv2.TagExclusion{Exact: []int64{9}, Trees: []int64{5, 12}}

	if _, err := c.ListWorks(ctx, catalogv2.WorksQuery{Q: "kun", Exclude: exclude}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Calendar(ctx, "2026-10", true, "", 100, exclude); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ListWorks(ctx, catalogv2.WorksQuery{Q: "kun"}); err != nil {
		t.Fatal(err)
	}
	for i, q := range seen[:2] {
		if q.Get("exclude_tag_id") != "9" || q.Get("exclude_tag_tree") != "5,12" {
			t.Errorf("request %d sent exclude_tag_id=%q exclude_tag_tree=%q", i, q.Get("exclude_tag_id"), q.Get("exclude_tag_tree"))
		}
	}
	if q := seen[2]; q.Has("exclude_tag_id") || q.Has("exclude_tag_tree") {
		t.Errorf("a reader with no blocks sent %v", q)
	}
}

func TestExcludedWorkIDsIsCachedPerParameterSet(t *testing.T) {
	var seen []string
	c, mr := cachedClient(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.URL.Path+"?"+r.URL.RawQuery)
		_, _ = w.Write([]byte(`{"object":"tag_exclusion","tag_ids":["5","6","9"],"work_ids":["11","3021","70001"]}`))
	})
	ctx := context.Background()
	exclude := catalogv2.TagExclusion{Exact: []int64{9}, Trees: []int64{5}}

	for range 2 {
		ids, err := c.ExcludedWorkIDs(ctx, exclude)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(ids, []int64{11, 3021, 70001}) {
			t.Fatalf("ExcludedWorkIDs = %v", ids)
		}
	}
	if len(seen) != 1 || seen[0] != "/v2/catalog/tag-exclusion?exclude_tag_id=9&exclude_tag_tree=5" {
		t.Fatalf("upstream saw %v, want one tag-exclusion read", seen)
	}
	if ttl := mr.TTL(mr.Keys()[0]); ttl != 10*time.Minute {
		t.Fatalf("tag-exclusion TTL = %v, want 10m", ttl)
	}

	if ids, err := c.ExcludedWorkIDs(ctx, catalogv2.TagExclusion{}); err != nil || ids != nil || len(seen) != 1 {
		t.Fatalf("an empty exclusion answered %v, %v after %d requests; want no request", ids, err, len(seen))
	}
}
