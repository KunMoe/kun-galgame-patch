package service

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"kun-galgame-patch-api/internal/face/dto"
	"kun-galgame-patch-api/internal/infrastructure/markdown"
	patchModel "kun-galgame-patch-api/internal/patch/model"
	"kun-galgame-patch-api/pkg/problem"
	"kun-galgame-patch-api/pkg/userclient"
)

func (s *Service) buildPatches(ctx context.Context, rows []patchModel.Patch, include Includes) ([]dto.Patch, error) {
	if len(rows) == 0 {
		return nil, nil
	}
	var attached []patchModel.PatchResource
	if include.Has("resources") {
		ids := make([]int, 0, len(rows))
		for i := range rows {
			ids = append(ids, rows[i].ID)
		}
		var err error
		if attached, err = s.repo.ResourcesForPatches(ids); err != nil {
			return nil, err
		}
	}

	briefs := map[int]*userclient.Brief{}
	if include.Has("publisher") {
		uids := make([]int, 0, len(rows)+len(attached))
		for i := range rows {
			uids = append(uids, rows[i].UserID)
		}
		for i := range attached {
			uids = append(uids, attached[i].UserID)
		}
		briefs = userclient.BriefMapByInt(ctx, s.users, uids)
	}

	byPatch := make(map[int][]patchModel.PatchResource, len(rows))
	for i := range attached {
		id := attached[i].GalgameID
		byPatch[id] = append(byPatch[id], attached[i])
	}

	out := make([]dto.Patch, 0, len(rows))
	for i := range rows {
		item := s.patchDTO(&rows[i])
		if include.Has("publisher") {
			item.Publisher = s.userDTO(briefs[rows[i].UserID], rows[i].UserID)
		}
		if include.Has("resources") {
			list := make([]dto.Resource, 0, len(byPatch[rows[i].ID]))
			for _, r := range byPatch[rows[i].ID] {
				res := s.resourceDTO(&r)
				if include.Has("publisher") {
					res.Publisher = s.userDTO(briefs[r.UserID], r.UserID)
				}
				list = append(list, res)
			}
			item.Resources = &list
		}
		out = append(out, item)
	}
	return out, nil
}

func (s *Service) buildResources(ctx context.Context, rows []patchModel.PatchResource, include Includes) ([]dto.Resource, error) {
	briefs := map[int]*userclient.Brief{}
	if include.Has("publisher") {
		uids := make([]int, 0, len(rows))
		for i := range rows {
			uids = append(uids, rows[i].UserID)
		}
		briefs = userclient.BriefMapByInt(ctx, s.users, uids)
	}
	out := make([]dto.Resource, 0, len(rows))
	for i := range rows {
		item := s.resourceDTO(&rows[i])
		if include.Has("publisher") {
			item.Publisher = s.userDTO(briefs[rows[i].UserID], rows[i].UserID)
		}
		out = append(out, item)
	}
	return out, nil
}

func (s *Service) patchDTO(p *patchModel.Patch) dto.Patch {
	item := dto.Patch{
		Object:            "patch",
		ID:                strconv.Itoa(p.ID),
		VndbID:            p.VndbID,
		Type:              jsonArray(p.Type),
		Language:          jsonArray(p.Language),
		Platform:          jsonArray(p.Platform),
		ResourceCount:     p.ResourceCount,
		DownloadCount:     p.Download,
		ViewCount:         p.View,
		FavoriteCount:     p.FavoriteCount,
		CommentCount:      p.CommentCount,
		WebURL:            s.siteURL + "/galgame/" + strconv.Itoa(p.ID),
		CreatedAt:         stamp(p.Created),
		UpdatedAt:         stamp(p.Updated),
		ResourceUpdatedAt: stamp(p.ResourceUpdateTime),
	}
	workID := item.ID
	item.CatalogWorkID = &workID
	if p.ContentLimit != nil && *p.ContentLimit != "" {
		limit := *p.ContentLimit
		item.ContentLimit = &limit
	}
	if p.ReleaseDate != nil {
		date := p.ReleaseDate.Format(time.DateOnly)
		item.ReleaseDate = &date
	}
	return item
}

func (s *Service) resourceDTO(r *patchModel.PatchResource) dto.Resource {
	return dto.Resource{
		Object:                "patch_resource",
		ID:                    strconv.Itoa(r.ID),
		PatchID:               strconv.Itoa(r.GalgameID),
		Name:                  r.Name,
		Storage:               r.Storage,
		Size:                  r.Size,
		Hash:                  r.Blake3,
		ModelName:             r.ModelName,
		LocalizationGroupName: r.LocalizationGroupName,
		Note:                  markdown.ResolveContentImageTokens(r.Note),
		Type:                  jsonArray(r.Type),
		Language:              jsonArray(r.Language),
		Platform:              jsonArray(r.Platform),
		DownloadCount:         r.Download,
		LikeCount:             r.LikeCount,
		WebURL:                s.siteURL + "/resource/" + strconv.Itoa(r.ID),
		CreatedAt:             stamp(r.Created),
		UpdatedAt:             stamp(r.UpdateTime),
	}
}

// userDTO answers even when the profile lookup came back empty. OAuth owns the
// profile (contract C6) and can be slow or down; a resource whose publisher is
// an id and a blank name is still a usable answer, a 503 for the whole page is
// not.
func (s *Service) userDTO(b *userclient.Brief, fallbackID int) *dto.User {
	item := &dto.User{Object: "user", ID: strconv.Itoa(fallbackID)}
	if b == nil {
		return item
	}
	item.Name = b.Name
	item.AvatarURL = b.Avatar
	if b.AvatarImageHash != "" && s.img != nil {
		if url := s.img.MainURL(b.AvatarImageHash); url != "" {
			item.AvatarURL = url
		}
	}
	return item
}

// missingAnchors echoes back the anchors that matched nothing, in the caller's
// own spelling. It is the whole reason a batch lookup beats one request per id:
// "which of these 100 games do you have" is answered in one round trip.
func missingAnchors(q *PatchQuery, rows []patchModel.Patch) []string {
	found := make(map[string]bool, len(rows)*3)
	for i := range rows {
		found[strconv.Itoa(rows[i].ID)] = true
		found["vndb:"+rows[i].VndbID] = true
		found["catalog:"+strconv.Itoa(rows[i].ID)] = true
	}
	asked := q.IDs
	if len(asked) == 0 {
		asked = q.Refs
	}
	missing := make([]string, 0)
	for _, token := range asked {
		if !found[token] {
			missing = append(missing, token)
		}
	}
	return missing
}

func jsonArray(a patchModel.JSONArray) []string {
	if a == nil {
		return []string{}
	}
	return a
}

func stamp(t time.Time) string { return t.UTC().Format(time.RFC3339) }

// unavailable is the one shape a caller sees for anything that went wrong
// underneath. The cause is logged rather than returned: a public face must not
// hand a third party the shape of a query that failed.
func unavailable(err error) *problem.Problem {
	slog.Error("face request failed", "error", err)
	return problem.New(problem.CodeServiceUnavailable, "the request could not be completed; it may be retried")
}
