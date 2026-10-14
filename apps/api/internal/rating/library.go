package rating

import (
	"context"

	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/internal/galgame/enricher"
	patchModel "kun-galgame-patch-api/internal/patch/model"
	"kun-galgame-patch-api/pkg/catalogv2"
)

const libraryPageSize = 24

const (
	VisibilityPublic    = "public"
	VisibilityOwnerOnly = "owner_only"
	VisibilityHidden    = "hidden"
)

type LibraryCatalog interface {
	WorkRecords(ctx context.Context, userID int, q catalogv2.WorkRecordsQuery) (catalogv2.WorkRecordPage, error)
	MyWorkRecords(ctx context.Context, accessToken string, q catalogv2.WorkRecordsQuery) (catalogv2.WorkRecordPage, error)
	PlayRecordsPrivate(ctx context.Context, accessToken string) (bool, error)
	SetPlayRecordsPrivate(ctx context.Context, accessToken string, private bool) (bool, error)
}

type LibraryWorks interface {
	GalgameCardBatch(ctx context.Context, ids []int, contentLimit string) ([]galgameClient.GalgameBrief, error)
}

type patchRows interface {
	PatchRows(ids []int) (map[int]patchModel.Patch, error)
}

// Library is a profile's 游戏库. NextMoe offers no mirror channel for play
// records, so every page is read live.
type Library struct {
	catalog LibraryCatalog
	works   LibraryWorks
	patches patchRows
}

func NewLibrary(catalog LibraryCatalog, works LibraryWorks, patches patchRows) *Library {
	return &Library{catalog: catalog, works: works, patches: patches}
}

type LibraryQuery struct {
	UserID       int
	ViewerID     int
	AccessToken  string
	State        string
	Cursor       string
	ContentLimit string
}

type LibraryItem struct {
	enricher.GalgameCard
	PlayState string `json:"play_state"`
	Minutes   int    `json:"minutes"`
	Overall   *int   `json:"overall"`
}

type LibraryPage struct {
	Visibility string         `json:"visibility"`
	Items      []LibraryItem  `json:"items"`
	NextCursor string         `json:"next_cursor"`
	Total      int64          `json:"total"`
	Counts     map[string]int `json:"counts,omitempty"`
}

func catalogContentLimit(cl string) string {
	switch cl {
	case "sfw", "nsfw":
		return cl
	}
	return ""
}

// facets=state counts after state=, so a state tab takes the tab counts from a
// second read without it.
func (l *Library) Page(ctx context.Context, q LibraryQuery) (*LibraryPage, error) {
	out := &LibraryPage{Visibility: VisibilityPublic, Items: []LibraryItem{}}
	read := func(wq catalogv2.WorkRecordsQuery) (catalogv2.WorkRecordPage, error) {
		return l.catalog.WorkRecords(ctx, q.UserID, wq)
	}
	if q.ViewerID == q.UserID {
		private, err := l.catalog.PlayRecordsPrivate(ctx, q.AccessToken)
		if err != nil {
			return nil, err
		}
		if private {
			out.Visibility = VisibilityOwnerOnly
		}
		read = func(wq catalogv2.WorkRecordsQuery) (catalogv2.WorkRecordPage, error) {
			return l.catalog.MyWorkRecords(ctx, q.AccessToken, wq)
		}
	}

	cl := catalogContentLimit(q.ContentLimit)
	wq := catalogv2.WorkRecordsQuery{
		ContentLimit: cl,
		Cursor:       q.Cursor,
		Limit:        libraryPageSize,
		Facets:       q.Cursor == "" && q.State == "",
	}
	if q.State != "" {
		wq.States = []string{q.State}
	}
	page, err := read(wq)
	if catalogv2.IsPlayRecordsPrivate(err) {
		out.Visibility = VisibilityHidden
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	out.NextCursor, out.Total, out.Counts = page.NextCursor, page.Total, page.StateCounts
	if q.Cursor == "" && q.State != "" {
		all, err := read(catalogv2.WorkRecordsQuery{ContentLimit: cl, Limit: 1, Facets: true})
		if err != nil {
			return nil, err
		}
		out.Total, out.Counts = all.Total, all.StateCounts
	}
	if out.Items, err = l.items(ctx, page.Items, q.ContentLimit); err != nil {
		return nil, err
	}
	return out, nil
}

func (l *Library) items(ctx context.Context, records []catalogv2.ListedWorkRecord, contentLimit string) ([]LibraryItem, error) {
	items := make([]LibraryItem, 0, len(records))
	if len(records) == 0 {
		return items, nil
	}
	ids := make([]int, 0, len(records))
	for _, r := range records {
		ids = append(ids, int(r.WorkID))
	}
	briefs, err := l.works.GalgameCardBatch(ctx, ids, contentLimit)
	if err != nil {
		return nil, err
	}
	local, err := l.patches.PatchRows(ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[int]*galgameClient.GalgameBrief, len(briefs))
	for i := range briefs {
		byID[briefs[i].ID] = &briefs[i]
	}
	for _, r := range records {
		brief := byID[int(r.WorkID)]
		if brief == nil {
			continue
		}
		var patch *patchModel.Patch
		if p, ok := local[brief.ID]; ok {
			patch = &p
		}
		item := LibraryItem{
			GalgameCard: enricher.CardFromBriefAndPatch(brief, patch),
			PlayState:   fromCatalogState(r.State, r.Completion),
		}
		if r.Minutes >= playtimeFloor {
			item.Minutes = r.Minutes
		}
		if r.Rating != nil {
			overall := r.Rating.Overall
			item.Overall = &overall
		}
		items = append(items, item)
	}
	return items, nil
}

func (l *Library) Private(ctx context.Context, accessToken string) (bool, error) {
	return l.catalog.PlayRecordsPrivate(ctx, accessToken)
}

func (l *Library) SetPrivate(ctx context.Context, accessToken string, private bool) (bool, error) {
	return l.catalog.SetPlayRecordsPrivate(ctx, accessToken, private)
}
