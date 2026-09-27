// Package following reads the network-wide following feed: groups of what the
// people a reader follows did on every NextMoe site, newest first.
package following

import (
	"context"
	"net/url"
	"strings"
	"time"

	patchModel "kun-galgame-patch-api/internal/patch/model"
	"kun-galgame-patch-api/pkg/communityclient"
	"kun-galgame-patch-api/pkg/userclient"
)

// thisSite is moyu's community tenant (oauth_clients.community_site). Items
// from it link inside the site; every other site's open in a new tab.
const thisSite = "moyu"

type Community interface {
	FollowingActivities(ctx context.Context, userID int64, cursor string, limit int, contentLimit string) (*communityclient.ActivityGroupPage, error)
	ActivityGroupItems(ctx context.Context, groupID int64, cursor string, limit int, contentLimit string) (*communityclient.ActivityItemPage, error)
	FollowingActivitiesUnseen(ctx context.Context, userID int64, contentLimit string) (*communityclient.ActivityUnseen, error)
	MarkFollowingActivitiesSeen(ctx context.Context, userID int64, at *time.Time) (*communityclient.ActivitySeen, error)
}

type Briefs func(ctx context.Context, ids []int) map[int]*userclient.Brief

type Service struct {
	community Community
	briefs    Briefs
}

func New(community Community, users *userclient.Client) *Service {
	return &Service{community: community, briefs: func(ctx context.Context, ids []int) map[int]*userclient.Brief {
		return userclient.BriefMapByInt(ctx, users, ids)
	}}
}

type Item struct {
	ID          int64  `json:"id"`
	Site        string `json:"site"`
	Verb        string `json:"verb"`
	ObjectKind  string `json:"object_kind"`
	ObjectLabel string `json:"object_label"`
	Title       string `json:"title"`
	Excerpt     string `json:"excerpt"`
	URL         string `json:"url"`
	// Path is the in-site route for this site's own items, null for another
	// site's, which the page opens in a new tab.
	Path           *string   `json:"path"`
	CoverImageHash string    `json:"cover_image_hash"`
	WorkID         *int64    `json:"work_id"`
	ContentLimit   string    `json:"content_limit"`
	OccurredAt     time.Time `json:"occurred_at"`
}

type Group struct {
	ID          int64                 `json:"id"`
	Site        string                `json:"site"`
	Actor       *patchModel.PatchUser `json:"actor"`
	Verb        string                `json:"verb"`
	ObjectKind  string                `json:"object_kind"`
	ObjectLabel string                `json:"object_label"`
	Day         string                `json:"day"`
	ItemCount   int                   `json:"item_count"`
	LatestAt    time.Time             `json:"latest_at"`
	Items       []Item                `json:"items"`
}

type Page struct {
	Groups     []Group `json:"groups"`
	NextCursor string  `json:"next_cursor"`
	// SeenMark is what the page sends back to mark the feed seen: the newest
	// group community answered, before this site dropped anything. Marking up to
	// the first group the page rendered instead would leave a newer group it did
	// not render unseen forever, and the red dot would never clear.
	SeenMark *time.Time `json:"seen_mark"`
}

type ItemPage struct {
	Items      []Item `json:"items"`
	NextCursor string `json:"next_cursor"`
}

// FeedLimit maps the reader's content preference onto the feed's all|sfw.
func FeedLimit(pref string) string {
	if pref == "nsfw" || pref == "all" {
		return communityclient.FeedAll
	}
	return communityclient.FeedSFW
}

func (s *Service) Groups(ctx context.Context, viewerID int, cursor string, limit int, contentLimit string) (*Page, error) {
	res, err := s.community.FollowingActivities(ctx, int64(viewerID), cursor, limit, contentLimit)
	if err != nil {
		return nil, err
	}
	page := &Page{Groups: make([]Group, 0, len(res.Groups)), NextCursor: res.NextCursor}
	if cursor == "" && len(res.Groups) > 0 {
		mark := res.Groups[0].LatestAt
		page.SeenMark = &mark
	}

	ids := make([]int, 0, len(res.Groups))
	for _, g := range res.Groups {
		ids = append(ids, int(g.ActorID))
	}
	briefs := s.briefs(ctx, ids)
	for _, g := range res.Groups {
		page.Groups = append(page.Groups, Group{
			ID: g.ID, Site: g.Site, Actor: patchModel.NewPatchUser(briefs[int(g.ActorID)]),
			Verb: g.Verb, ObjectKind: g.ObjectKind, ObjectLabel: g.ObjectLabel,
			Day: g.Day, ItemCount: g.ItemCount, LatestAt: g.LatestAt, Items: items(g.Items),
		})
	}
	return page, nil
}

func (s *Service) GroupItems(ctx context.Context, groupID int64, cursor string, limit int, contentLimit string) (*ItemPage, error) {
	res, err := s.community.ActivityGroupItems(ctx, groupID, cursor, limit, contentLimit)
	if err != nil {
		return nil, err
	}
	return &ItemPage{Items: items(res.Items), NextCursor: res.NextCursor}, nil
}

func (s *Service) Unseen(ctx context.Context, viewerID int, contentLimit string) (int, error) {
	res, err := s.community.FollowingActivitiesUnseen(ctx, int64(viewerID), contentLimit)
	if err != nil {
		return 0, err
	}
	return res.UnseenCount, nil
}

func (s *Service) MarkSeen(ctx context.Context, viewerID int, at *time.Time) (time.Time, error) {
	res, err := s.community.MarkFollowingActivitiesSeen(ctx, int64(viewerID), at)
	if err != nil {
		return time.Time{}, err
	}
	return res.SeenAt, nil
}

func items(in []communityclient.ActivityItemView) []Item {
	out := make([]Item, 0, len(in))
	for _, it := range in {
		item := Item{
			ID: it.ID, Site: it.Site, Verb: it.Verb, ObjectKind: it.ObjectKind, ObjectLabel: it.ObjectLabel,
			Title: it.Title, Excerpt: it.Excerpt, URL: it.URL, WorkID: it.WorkID,
			ContentLimit: it.ContentLimit, OccurredAt: it.OccurredAt,
		}
		if it.CoverImageHash != nil {
			item.CoverImageHash = *it.CoverImageHash
		}
		if it.Site == thisSite {
			if u, err := url.Parse(it.URL); err == nil && strings.HasPrefix(u.RequestURI(), "/") {
				path := u.RequestURI()
				item.Path = &path
			}
		}
		out = append(out, item)
	}
	return out
}
