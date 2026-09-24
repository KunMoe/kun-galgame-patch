package client

import (
	"context"
	"strconv"
	"strings"

	"kun-galgame-patch-api/pkg/catalogv2"
)

const CatalogWorksIDsMax = 100

type catalogGate struct {
	contentLimit  string
	contentRating string
}

func gateFor(contentLimit string) catalogGate {
	switch strings.ToLower(strings.TrimSpace(contentLimit)) {
	case "sfw":
		return catalogGate{contentLimit: "sfw"}
	case "nsfw":
		return catalogGate{contentLimit: "nsfw"}
	default:
		return catalogGate{}
	}
}

func (g catalogGate) allows(displayLimit string) bool {
	return g.contentLimit == "" || g.contentLimit == displayLimit
}

// A page id IS the catalog work id (migration 037 / cmd/align-patch-ids), so
// there is no bridge to cross and nothing to cache. What used to live here --
// a `curated` ref lookup, a `galgame_wiki` lookup against a source key catalog
// does not have, an identity fallback guarded against the 10,289 gids that
// were also some unrelated work's catalog id, and an hour-long in-memory map
// of the answers -- was all machinery for a mapping that no longer exists.
func catalogIDsOf(gids []int) []int64 {
	out := make([]int64, 0, len(gids))
	for _, gid := range gids {
		if gid > 0 {
			out = append(out, int64(gid))
		}
	}
	return out
}

// Company ids are a different story and keep their bridge. A galgame's id is
// catalog's because catalog mints the game; a 会社 arrived from the retired
// wiki under its own number and catalog files that as a `curated` ref, which is
// the only thing that names it. `galgame_wiki` is no longer tried alongside it:
// catalog's source table has no such key, so that lookup was always a miss and
// cost a round trip per id.
func (c *Client) ResolveWikiLabel(ctx context.Context, oid int) (int64, bool, error) {
	if oid <= 0 {
		return 0, false, nil
	}
	id, err := c.v2.EntityByRef(ctx, "company", "curated", strconv.Itoa(oid), true)
	if err != nil {
		if IsAbsent(err) {
			return 0, false, nil
		}
		return 0, false, err
	}
	if id <= 0 {
		return 0, false, nil
	}
	return id, true, nil
}

func (c *Client) ClaimStates(ctx context.Context, gids []int) (map[int]string, error) {
	out := make(map[int]string, len(gids))
	ids := catalogIDsOf(gids)
	if len(ids) == 0 {
		return out, nil
	}
	page, err := c.v2.ListWorks(ctx, catalogv2.WorksQuery{
		IDs: ids, NSFW: true, Limit: CatalogWorksIDsMax,
	})
	if err != nil {
		return nil, err
	}
	for i := range page.Items {
		it := workToListItem(page.Items[i])
		out[int(it.ID)] = claimStateOf(it.ClaimedBy)
	}
	return out, nil
}
