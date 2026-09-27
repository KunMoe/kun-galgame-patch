package following

import (
	"context"
	"testing"
	"time"

	"kun-galgame-patch-api/pkg/communityclient"
	"kun-galgame-patch-api/pkg/userclient"
)

type fakeFeed struct {
	page      *communityclient.ActivityGroupPage
	gotLimit  string
	gotCursor string
}

func (f *fakeFeed) FollowingActivities(_ context.Context, _ int64, cursor string, _ int, contentLimit string) (*communityclient.ActivityGroupPage, error) {
	f.gotCursor, f.gotLimit = cursor, contentLimit
	return f.page, nil
}

func (f *fakeFeed) ActivityGroupItems(context.Context, int64, int64, string, int, string) (*communityclient.ActivityItemPage, error) {
	return &communityclient.ActivityItemPage{}, nil
}

func (f *fakeFeed) ActivitySetting(context.Context, int64) (*communityclient.ActivitySetting, error) {
	return &communityclient.ActivitySetting{}, nil
}

func (f *fakeFeed) SetActivityHidden(context.Context, int64, bool) (*communityclient.ActivitySetting, error) {
	return &communityclient.ActivitySetting{}, nil
}

func (f *fakeFeed) FollowingActivitiesUnseen(context.Context, int64, string) (*communityclient.ActivityUnseen, error) {
	return &communityclient.ActivityUnseen{}, nil
}

func (f *fakeFeed) MarkFollowingActivitiesSeen(context.Context, int64, *time.Time) (*communityclient.ActivitySeen, error) {
	return &communityclient.ActivitySeen{}, nil
}

func TestGroupsLinkOwnItemsInsideAndMarkTheFirstGroupCommunitySent(t *testing.T) {
	newest := time.Date(2026, 9, 26, 4, 0, 0, 0, time.UTC)
	work := int64(42)
	feed := &fakeFeed{page: &communityclient.ActivityGroupPage{
		Groups: []communityclient.ActivityGroupView{
			{ID: 1, Site: "kungal", ActorID: 7, Verb: "publish", ObjectLabel: "话题", Day: "2026-09-26", ItemCount: 1, LatestAt: newest,
				Items: []communityclient.ActivityItemView{{ID: 11, Site: "kungal", URL: "https://www.kungal.com/topic/1"}}},
			{ID: 2, Site: "moyu", ActorID: 8, Verb: "publish", ObjectLabel: "Galgame 补丁", ItemCount: 2, LatestAt: newest.Add(-time.Hour),
				Items: []communityclient.ActivityItemView{{ID: 12, Site: "moyu", URL: "https://www.moyu.moe/resource/9?x=1", WorkID: &work}}},
		},
		NextCursor: "cur_2",
	}}
	svc := &Service{community: feed, briefs: func(_ context.Context, ids []int) map[int]*userclient.Brief {
		return map[int]*userclient.Brief{8: {ID: 8, Name: "kun"}}
	}}

	page, err := svc.Groups(context.Background(), 3, "", 20, FeedLimit("nsfw"))
	if err != nil {
		t.Fatal(err)
	}
	if feed.gotLimit != "all" {
		t.Errorf("content_limit = %q", feed.gotLimit)
	}
	if page.SeenMark == nil || !page.SeenMark.Equal(newest) || page.NextCursor != "cur_2" {
		t.Errorf("seen mark = %v, next = %q", page.SeenMark, page.NextCursor)
	}
	if other := page.Groups[0].Items[0]; other.Path != nil || page.Groups[0].Actor != nil {
		t.Errorf("another site's item = %+v", other)
	}
	own := page.Groups[1]
	if own.Actor == nil || own.Actor.Name != "kun" || own.Items[0].Path == nil || *own.Items[0].Path != "/resource/9?x=1" {
		t.Errorf("own group = %+v", own)
	}

	page, _ = svc.Groups(context.Background(), 3, "cur_2", 20, FeedLimit(""))
	if page.SeenMark != nil || feed.gotLimit != "sfw" {
		t.Errorf("a later page carried a seen mark %v, limit %q", page.SeenMark, feed.gotLimit)
	}
}
