package client

import (
	"cmp"
	"context"
	"log/slog"
	"slices"

	"kun-galgame-patch-api/pkg/catalogv2"
)

type TagRef struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	IsSexual bool   `json:"is_sexual"`
}

type TagNode struct {
	TagRef
	ParentIDs []int64 `json:"parent_ids"`
}

type TagSubtag struct {
	TagRef
	WorkCount       int `json:"work_count"`
	DescendantCount int `json:"descendant_count"`
}

const (
	tagTierHidden = "hidden"
	tagBatchSize  = 100
)

func tagRefOf(n catalogv2.TagNode) (TagRef, bool) {
	id, ok := n.IntID()
	return TagRef{ID: id, Name: n.DisplayName, IsSexual: n.IsSexual}, ok
}

func tagRefs(in []catalogv2.TagNode, hideSexual bool) []TagRef {
	out := make([]TagRef, 0, len(in))
	for _, n := range in {
		ref, ok := tagRefOf(n)
		if ok && !(hideSexual && n.IsSexual) {
			out = append(out, ref)
		}
	}
	return out
}

func tagNodes(in []catalogv2.TagNode) []TagNode {
	out := make([]TagNode, 0, len(in))
	for _, n := range in {
		ref, ok := tagRefOf(n)
		if !ok {
			continue
		}
		parents := make([]int64, 0, len(n.ParentIDs))
		for _, p := range n.ParentIDs {
			if id, ok := catalogv2.ParseID(p); ok {
				parents = append(parents, id)
			}
		}
		out = append(out, TagNode{TagRef: ref, ParentIDs: parents})
	}
	return out
}

func (c *Client) subtags(ctx context.Context, children []catalogv2.TagNode, hideSexual bool) []TagSubtag {
	ids := make([]int64, 0, len(children))
	for _, ch := range children {
		if id, ok := ch.IntID(); ok && !(hideSexual && ch.IsSexual) {
			ids = append(ids, id)
		}
	}
	out := make([]TagSubtag, 0, len(ids))
	for chunk := range slices.Chunk(ids, tagBatchSize) {
		rows, err := c.v2.TagsByIDs(ctx, chunk, true)
		if err != nil {
			slog.Warn("标签页的子标签读取失败，子标签区不显示", "error", err, "children", len(ids))
			return []TagSubtag{}
		}
		for _, r := range rows {
			id, ok := r.IntID()
			if !ok || r.Tier == tagTierHidden || (hideSexual && r.IsSexual) {
				continue
			}
			out = append(out, TagSubtag{
				TagRef:    TagRef{ID: id, Name: r.DisplayName, IsSexual: r.IsSexual},
				WorkCount: r.WorkCount, DescendantCount: r.DescendantCount,
			})
		}
	}
	slices.SortFunc(out, func(a, b TagSubtag) int {
		return cmp.Or(cmp.Compare(b.WorkCount, a.WorkCount), cmp.Compare(a.ID, b.ID))
	})
	return out
}

func (c *Client) TagDescendants(ctx context.Context, id int64) ([]TagNode, error) {
	nodes, err := c.v2.TagDescendants(ctx, id)
	if err != nil {
		return nil, err
	}
	return tagNodes(nodes), nil
}
