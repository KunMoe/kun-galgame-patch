package activitypush

import (
	"context"
	"slices"
	"strconv"
	"time"

	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	patchModel "kun-galgame-patch-api/internal/patch/model"
	"kun-galgame-patch-api/pkg/communityclient"

	"gorm.io/gorm"
)

type Catalog interface {
	GalgameBatch(ctx context.Context, ids []int, contentLimit string) ([]galgameClient.GalgameBrief, error)
}

type resolver struct {
	db      *gorm.DB
	catalog Catalog
	origin  string
}

type revisionRow struct {
	ResourceID int
	ActorID    int
	Changes    patchModel.ResourceChangeList
	CreatedAt  time.Time
}

// resolve reads the current state of each key: the item to push, or nil when
// the content is not public now. A key that does not parse is left out.
//
// Public is what the resource page itself shows: status 0 and a work catalog
// still renders. The catalog read is the same both-gates-open hydrate the page
// runs. Its error fails the whole call — an outage is not a verdict, and
// reading it as "work gone" would tombstone every resource in the batch.
func (r *resolver) resolve(ctx context.Context, keys []string) (map[string]*communityclient.ActivityItem, error) {
	parsed := make(map[string]parsedKey, len(keys))
	var resourceIDs, editResourceIDs []int
	for _, k := range keys {
		pk, ok := parseKey(k)
		if !ok {
			continue
		}
		parsed[k] = pk
		resourceIDs = append(resourceIDs, pk.resourceID)
		if pk.kind == kindEdit {
			editResourceIDs = append(editResourceIDs, pk.resourceID)
		}
	}

	resources, err := r.resources(ctx, resourceIDs)
	if err != nil {
		return nil, err
	}
	works, err := r.works(ctx, resources)
	if err != nil {
		return nil, err
	}
	revisions, err := r.revisions(ctx, editResourceIDs)
	if err != nil {
		return nil, err
	}

	out := make(map[string]*communityclient.ActivityItem, len(parsed))
	for k, pk := range parsed {
		out[k] = nil
		res, ok := resources[pk.resourceID]
		if !ok || res.Status != 0 {
			continue
		}
		work, ok := works[res.GalgameID]
		if !ok {
			continue
		}
		switch pk.kind {
		case kindResource:
			out[k] = r.publishItem(res, work)
		case kindEdit:
			if revs := revisions[k]; len(revs) > 0 {
				out[k] = r.editItem(k, pk.actorID, res, work, revs)
			}
		}
	}
	return out, nil
}

func (r *resolver) resources(ctx context.Context, ids []int) (map[int]patchModel.PatchResource, error) {
	out := make(map[int]patchModel.PatchResource, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	var rows []patchModel.PatchResource
	err := r.db.WithContext(ctx).
		Select("id", "name", "size", "type", "language", "platform", "status", "user_id", "galgame_id", "created").
		Where("id IN ?", slices.Compact(slices.Sorted(slices.Values(ids)))).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.ID] = row
	}
	return out, nil
}

func (r *resolver) works(ctx context.Context, resources map[int]patchModel.PatchResource) (map[int]*galgameClient.GalgameBrief, error) {
	seen := map[int]bool{}
	var ids []int
	for _, res := range resources {
		if res.Status == 0 && !patchModel.IsLocalOnly(res.GalgameID) && !seen[res.GalgameID] {
			seen[res.GalgameID] = true
			ids = append(ids, res.GalgameID)
		}
	}
	out := make(map[int]*galgameClient.GalgameBrief, len(ids))
	for chunk := range slices.Chunk(ids, galgameClient.CatalogWorksIDsMax) {
		briefs, err := r.catalog.GalgameBatch(ctx, chunk, "")
		if err != nil {
			return nil, err
		}
		for i := range briefs {
			out[briefs[i].ID] = &briefs[i]
		}
	}
	return out, nil
}

// revisions groups the (resource, editor, Beijing day) keys' rows.
func (r *resolver) revisions(ctx context.Context, resourceIDs []int) (map[string][]revisionRow, error) {
	out := map[string][]revisionRow{}
	if len(resourceIDs) == 0 {
		return out, nil
	}
	var rows []revisionRow
	err := r.db.WithContext(ctx).Model(&patchModel.PatchResourceRevision{}).
		Select("resource_id", "actor_id", "changes", "created_at").
		Where("resource_id IN ? AND actor_id > 0 AND action = ?", slices.Compact(slices.Sorted(slices.Values(resourceIDs))), "updated").
		Order("created_at").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		k := editKey(row.ResourceID, row.ActorID, row.CreatedAt)
		out[k] = append(out[k], row)
	}
	return out, nil
}

func (r *resolver) publishItem(res patchModel.PatchResource, work *galgameClient.GalgameBrief) *communityclient.ActivityItem {
	item := r.baseItem(resourceKey(res.ID), res.UserID, verbPublish, res, work, res.Created)
	item.Excerpt = cutRunes(withoutControls(joinNonEmpty(" · ",
		labelled(res.Type, typeLabels),
		labelled(res.Language, languageLabels),
		labelled(res.Platform, platformLabels),
		res.Size,
	)), excerptRunes)
	return item
}

// editItem is one editor's day on one resource; the excerpt names what they
// changed, never the values — a download link or password is only "下载信息".
func (r *resolver) editItem(key string, actorID int, res patchModel.PatchResource, work *galgameClient.GalgameBrief, revs []revisionRow) *communityclient.ActivityItem {
	var labels []string
	latest := revs[0].CreatedAt
	for _, rev := range revs {
		if rev.CreatedAt.After(latest) {
			latest = rev.CreatedAt
		}
		for _, ch := range rev.Changes {
			label := ch.Label
			if ch.Field == "download" {
				label = "下载信息"
			}
			if label != "" && !slices.Contains(labels, label) {
				labels = append(labels, label)
			}
		}
	}
	item := r.baseItem(key, actorID, verbEdit, res, work, latest)
	if len(labels) > 0 {
		item.Excerpt = cutRunes(withoutControls("修改了 "+joinNonEmpty("、", labels...)), excerptRunes)
	}
	return item
}

func (r *resolver) baseItem(key string, actorID int, verb string, res patchModel.PatchResource, work *galgameClient.GalgameBrief, occurred time.Time) *communityclient.ActivityItem {
	workID := int64(res.GalgameID)
	at := occurred.UTC().Truncate(time.Microsecond)
	title := cutRunes(singleLine(joinNonEmpty(" · ", work.PreferredName(), res.Name)), titleRunes)
	if title == "" {
		title = objectLabel
	}
	contentLimit := "nsfw"
	if work.ContentLimit == "sfw" {
		contentLimit = "sfw"
	}
	return &communityclient.ActivityItem{
		Key:            key,
		ActorID:        int64(actorID),
		Verb:           verb,
		ObjectKind:     objectKind,
		ObjectLabel:    objectLabel,
		Title:          title,
		URL:            r.origin + "/resource/" + strconv.Itoa(res.ID),
		CoverImageHash: coverHash(work.EffectiveBannerHash),
		WorkID:         &workID,
		ContentLimit:   contentLimit,
		OccurredAt:     &at,
	}
}
