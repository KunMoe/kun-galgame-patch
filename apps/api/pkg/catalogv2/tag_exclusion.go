package catalogv2

import (
	"context"
	"errors"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

const (
	tagExclusionPath = "/v2/catalog/tag-exclusion"

	TagExclusionMax = 100
)

var ErrTooManyExcludedTags = errors.New("catalogv2: more than 100 tag ids in one exclusion parameter")

type TagExclusion struct {
	Exact []int64
	Trees []int64
}

func ParseTagExclusion(exact, trees string) (TagExclusion, error) {
	e := TagExclusion{Exact: parseTagIDList(exact), Trees: parseTagIDList(trees)}
	if len(e.Exact) > TagExclusionMax || len(e.Trees) > TagExclusionMax {
		return TagExclusion{}, ErrTooManyExcludedTags
	}
	return e, nil
}

func parseTagIDList(raw string) []int64 {
	out := []int64{}
	for _, part := range strings.Split(raw, ",") {
		if id, ok := ParseID(part); ok {
			out = append(out, id)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

func (e TagExclusion) Empty() bool { return len(e.Exact) == 0 && len(e.Trees) == 0 }

func (e TagExclusion) apply(v url.Values) {
	if len(e.Exact) > 0 {
		v.Set("exclude_tag_id", joinIDs(e.Exact))
	}
	if len(e.Trees) > 0 {
		v.Set("exclude_tag_tree", joinIDs(e.Trees))
	}
}

// The answer is catalog work ids, and a page id is its catalog work id, so a
// local list drops them from patch.id with no mapping.
func (c *Client) ExcludedWorkIDs(ctx context.Context, e TagExclusion) ([]int64, error) {
	if e.Empty() {
		return nil, nil
	}
	v := url.Values{}
	e.apply(v)
	var out struct {
		WorkIDs []string `json:"work_ids"`
	}
	if err := c.get(ctx, tagExclusionPath+"?"+v.Encode(), &out); err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(out.WorkIDs))
	for _, raw := range out.WorkIDs {
		if id, err := strconv.ParseInt(raw, 10, 64); err == nil && id > 0 {
			ids = append(ids, id)
		}
	}
	return ids, nil
}
