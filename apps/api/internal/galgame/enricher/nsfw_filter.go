package enricher

import (
	"context"
	"log/slog"

	galgameClient "kun-galgame-patch-api/internal/galgame/client"
)

// FilterByGalgameContentLimit drops items whose owning galgame_id wiki excludes
// under the given content_limit. Used by list endpoints whose primary rows are
// PatchResource / PatchComment (i.e. don't go through EnrichPatches, but still
// need NSFW gating because they expose the owning patch via attach helpers).
//
// Returns the input unchanged when cl == "" (no filter requested) or wiki is
// nil. On wiki error we fail closed — returning nil — for the same reason
// EnrichPatches does: an unfiltered fallback would defeat the safe-by-default
// guarantee, and an empty list is the right answer for the SEO case.
//
// gidOf extracts the galgame_id from a row. Generic over T so the same helper
// works for both []PatchResource and []PatchComment without sacrificing the
// concrete element type at call sites.
func FilterByGalgameContentLimit[T any](
	ctx context.Context,
	wiki *galgameClient.Client,
	items []T,
	gidOf func(T) int,
	cl string,
) []T {
	if cl == "" || len(items) == 0 || wiki == nil {
		return items
	}
	seen := make(map[int]struct{}, len(items))
	gids := make([]int, 0, len(items))
	for _, it := range items {
		gid := gidOf(it)
		if gid <= 0 {
			continue
		}
		if _, ok := seen[gid]; ok {
			continue
		}
		seen[gid] = struct{}{}
		gids = append(gids, gid)
	}
	if len(gids) == 0 {
		return items
	}

	briefs, err := wiki.GalgameBatch(ctx, gids, cl)
	if err != nil {
		slog.Warn("NSFW filter: wiki batch 失败，返回空 slice 兜底以防泄漏", "error", err, "content_limit", cl, "count", len(items))
		return nil
	}
	allowed := make(map[int]struct{}, len(briefs))
	for i := range briefs {
		allowed[briefs[i].ID] = struct{}{}
	}

	out := items[:0]
	for _, it := range items {
		if _, ok := allowed[gidOf(it)]; ok {
			out = append(out, it)
		}
	}
	return out
}
