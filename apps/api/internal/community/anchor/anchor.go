// Package anchor mints and reads back the community anchors moyu owns.
//
// The community tenant is NOT catalog_site. It comes from the calling client's
// oauth_clients.community_site, which is `moyu` here while catalog_site stays
// `kungal` (the catalog claim link depends on that one). Community read
// catalog_site until 2026-09-16, which put this site's walls in the forum's
// tenant: one purge blanked a user's forum comments, and every tenant-wide face
// answered the other site's rows.
//
// Inside that tenant an anchor id is moyu's own bare id — the patch id for a
// game wall (铁律 3: it IS the catalog work id, which is what lets infra's
// cmd/retire-merged-comments read a moyu site_game anchor as one) and the
// resource id for a resource wall.
package anchor

import (
	"context"
	"log/slog"
	"strconv"

	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/pkg/communityclient"
)

// PatchAnchor is the comment wall of a game page.
func PatchAnchor(patchID int) (int32, string) {
	return communityclient.AnchorSiteGame, strconv.Itoa(patchID)
}

// ResourceAnchor is the comment wall of one patch resource.
func ResourceAnchor(resourceID int) (int32, string) {
	return communityclient.AnchorSiteResource, strconv.Itoa(resourceID)
}

// IsMoyu answers whether an anchor is one of this site's, without touching the
// database. It is not the tautology the tenant makes it look like: GET /posts
// and GET /search/posts answer the caller's own site PLUS every catalog-anchored
// thread, which are one network-wide conversation by design. The sweeps that
// walk the whole corpus ask this rather than resolving each row, which would be
// a query per post.
func IsMoyu(kind int32, anchorID string) bool {
	switch kind {
	case communityclient.AnchorSiteGame, communityclient.AnchorSiteResource:
		_, ok := parseID(anchorID)
		return ok
	}
	return false
}

// Ref is a comment wall's anchor as the community service reports it.
type Ref struct {
	Kind int32
	ID   string
}

// Target is the moyu page that hosts a wall, ready to render as a result row.
type Target struct {
	Link  string
	Label string
	// Title is the game's name. Only ResolveNamed fills it: the patch table
	// caches no title, because catalog owns the name.
	Title string
	// PatchID is the game page the wall belongs to — set for both kinds, since a
	// resource wall also lives under a game.
	PatchID int
	// ResourceID is set only for a resource wall.
	ResourceID int
}

// Resolver turns anchors back into moyu links.
type Resolver struct {
	galgame  *galgameClient.Client
	resource ResourceOwner
}

// ResourceOwner answers which game each resource hangs off. A resource wall's
// anchor carries only the resource id, and the row has to name the game.
type ResourceOwner interface {
	ResourcePatchIDs(resourceIDs []int) (map[int]int, error)
}

func New(galgame *galgameClient.Client, resource ResourceOwner) *Resolver {
	return &Resolver{galgame: galgame, resource: resource}
}

// Resolve maps every anchor it recognises. A catalog-anchored wall another site
// opened is left out rather than linked to a moyu page that never held it.
func (r *Resolver) Resolve(refs []Ref) (map[Ref]Target, error) {
	out := make(map[Ref]Target, len(refs))
	resourceRefs := make(map[int][]Ref)

	for _, ref := range refs {
		if _, done := out[ref]; done {
			continue
		}
		id, ok := parseID(ref.ID)
		if !ok {
			continue
		}
		switch ref.Kind {
		case communityclient.AnchorSiteGame:
			out[ref] = Target{Link: patchLink(id), Label: "游戏", PatchID: id}
		case communityclient.AnchorSiteResource:
			resourceRefs[id] = append(resourceRefs[id], ref)
		}
	}

	if len(resourceRefs) > 0 {
		ids := make([]int, 0, len(resourceRefs))
		for id := range resourceRefs {
			ids = append(ids, id)
		}
		patchIDs := map[int]int{}
		if r.resource != nil {
			var err error
			if patchIDs, err = r.resource.ResourcePatchIDs(ids); err != nil {
				return nil, err
			}
		}
		for id, refs := range resourceRefs {
			for _, ref := range refs {
				out[ref] = Target{
					Link: resourceLink(id), Label: "补丁资源",
					PatchID: patchIDs[id], ResourceID: id,
				}
			}
		}
	}
	return out, nil
}

// ResolveNamed is Resolve plus the game names, for the surfaces that print a
// wall as a row the reader has to recognise. Names come from catalog in one
// batch; a name that does not arrive leaves Label standing.
func (r *Resolver) ResolveNamed(ctx context.Context, refs []Ref) (map[Ref]Target, error) {
	targets, err := r.Resolve(refs)
	if err != nil || r.galgame == nil || len(targets) == 0 {
		return targets, err
	}
	ids := make([]int, 0, len(targets))
	seen := make(map[int]bool, len(targets))
	for _, target := range targets {
		if target.PatchID > 0 && !seen[target.PatchID] {
			seen[target.PatchID] = true
			ids = append(ids, target.PatchID)
		}
	}
	if len(ids) == 0 {
		return targets, nil
	}
	// Both gates open: this names a row the reader is already allowed to see,
	// and the surfaces calling it do their own content-limit filtering.
	briefs, err := r.galgame.GalgameBatch(ctx, ids, "")
	if err != nil {
		slog.Warn("anchor: galgame name enrichment failed (best-effort)", "error", err)
		return targets, nil
	}
	names := make(map[int]string, len(briefs))
	for i := range briefs {
		names[briefs[i].ID] = displayName(&briefs[i])
	}
	for ref, target := range targets {
		if name := names[target.PatchID]; name != "" {
			target.Title = name
			targets[ref] = target
		}
	}
	return targets, nil
}

func parseID(anchorID string) (int, bool) {
	id, err := strconv.Atoi(anchorID)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

func patchLink(patchID int) string {
	return "/galgame/" + strconv.Itoa(patchID) + "?tab=comment"
}

func resourceLink(resourceID int) string {
	return "/resource/" + strconv.Itoa(resourceID)
}

func displayName(b *galgameClient.GalgameBrief) string {
	for _, n := range []string{b.NameZhCn, b.NameJaJp, b.NameEnUs, b.NameZhTw} {
		if n != "" {
			return n
		}
	}
	return b.VndbID
}
