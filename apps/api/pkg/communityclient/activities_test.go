package communityclient_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"kun-galgame-patch-api/pkg/communityclient"
	"kun-galgame-patch-api/pkg/upstream"
)

// The wire shapes below follow infra's dto/activity_dto.go, not a fake of our
// own: letmoe and kungal both shipped a follow face whose booleans were
// strings, and a fake written from the same misreading kept their suites green.
func TestWriteActivitiesSendsTombstonesBare(t *testing.T) {
	var body map[string][]map[string]any
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/activities" {
			t.Errorf("wrote to %s %s", r.Method, r.URL.Path)
		}
		raw, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Fatalf("body: %v", err)
		}
		envelope(w, map[string]any{"results": []any{
			map[string]any{"key": "patch_resource:1", "outcome": "created"},
			map[string]any{"key": "patch_resource:2", "outcome": "invalid", "reason": "url host"},
		}})
	})

	work := int64(42)
	at := time.Date(2026, 9, 26, 4, 0, 0, 123456000, time.UTC)
	res, err := c.WriteActivities(context.Background(), []communityclient.ActivityItem{
		{
			Key: "patch_resource:1", ActorID: 7, Revision: 1758859200123456, Verb: "publish",
			ObjectKind: "patch_resource", ObjectLabel: "Galgame 补丁", Title: "t", URL: "https://www.moyu.moe/resource/1",
			WorkID: &work, ContentLimit: "sfw", Notify: true, OccurredAt: &at,
		},
		{Key: "patch_resource:2", ActorID: 7, Revision: 1758859200123456, Removed: true},
	})
	if err != nil {
		t.Fatalf("WriteActivities: %v", err)
	}
	if len(res.Results) != 2 || res.Results[0].Outcome != communityclient.ActivityCreated ||
		res.Results[1].Outcome != communityclient.ActivityInvalid || res.Results[1].Reason != "url host" {
		t.Errorf("results = %+v", res.Results)
	}

	live, tomb := body["items"][0], body["items"][1]
	if live["notify"] != true || live["work_id"] != float64(42) || live["occurred_at"] != "2026-09-26T04:00:00.123456Z" {
		t.Errorf("live item = %v", live)
	}
	if _, ok := live["removed"]; ok {
		t.Errorf("a live item carried removed: %v", live)
	}
	for _, f := range []string{"verb", "title", "url", "work_id", "notify", "occurred_at", "content_limit"} {
		if _, ok := tomb[f]; ok {
			t.Errorf("tombstone carried %q: %v", f, tomb)
		}
	}
	if tomb["removed"] != true {
		t.Errorf("tombstone = %v", tomb)
	}
}

func TestWriteActivitiesSurfacesTheWholeBatch422(t *testing.T) {
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"code":7,"message":"items[0]: unknown field notfy"}`))
	})
	_, err := c.WriteActivities(context.Background(), []communityclient.ActivityItem{{Key: "k", ActorID: 1, Revision: 1}})
	e, ok := upstream.As(err)
	if !ok || e.Status != http.StatusUnprocessableEntity {
		t.Fatalf("err = %v", err)
	}
}

func TestListSiteActivitiesDecodesEveryStoredField(t *testing.T) {
	var gotQuery string
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/activities" {
			t.Errorf("path = %q", r.URL.Path)
		}
		gotQuery = r.URL.RawQuery
		envelope(w, map[string]any{
			"activities": []any{
				map[string]any{
					"id": 1, "key": "patch_resource:9", "actor_id": 7, "verb": "publish",
					"object_kind": "patch_resource", "object_label": "Galgame 补丁", "title": "t", "excerpt": "",
					"url": "https://www.moyu.moe/resource/9", "cover_image_hash": nil, "work_id": nil,
					"content_limit": "nsfw", "notify": true, "occurred_at": "2026-09-26T04:00:00.123456Z",
					"revision": 1758859200123456, "removed": false,
					"created_at": "2026-09-26T04:00:01Z", "updated_at": "2026-09-26T04:00:01Z", "removed_at": nil,
				},
				map[string]any{
					"id": 2, "key": "patch_resource:10", "actor_id": 7, "verb": "publish", "object_kind": "patch_resource",
					"object_label": "Galgame 补丁", "title": "", "excerpt": "", "url": "", "cover_image_hash": nil,
					"work_id": nil, "content_limit": "nsfw", "notify": false, "occurred_at": "2026-09-26T04:00:00Z",
					"revision": 1758859200999999, "removed": true,
					"created_at": "2026-09-26T04:00:01Z", "updated_at": "2026-09-26T05:00:01Z", "removed_at": "2026-09-26T05:00:01Z",
				},
			},
			"next_cursor": "cur_2",
		})
	})

	page, err := c.ListSiteActivities(context.Background(), "cur_1", 1000)
	if err != nil {
		t.Fatalf("ListSiteActivities: %v", err)
	}
	for _, want := range []string{"cursor=cur_1", "limit=1000"} {
		if !strings.Contains(gotQuery, want) {
			t.Errorf("query %q missing %q", gotQuery, want)
		}
	}
	a, b := page.Activities[0], page.Activities[1]
	if a.Revision != 1758859200123456 || !a.Notify || a.WorkID != nil || a.CoverImageHash != nil || a.Removed {
		t.Errorf("live = %+v", a)
	}
	if !a.OccurredAt.Equal(time.Date(2026, 9, 26, 4, 0, 0, 123456000, time.UTC)) {
		t.Errorf("occurred_at = %v", a.OccurredAt)
	}
	if !b.Removed || b.RemovedAt == nil || page.NextCursor != "cur_2" {
		t.Errorf("tombstone = %+v, next = %q", b, page.NextCursor)
	}
}

func TestFollowingActivitiesReadsGroups(t *testing.T) {
	var gotPath, gotQuery string
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.Path, r.URL.RawQuery
		envelope(w, map[string]any{
			"groups": []any{map[string]any{
				"id": 5, "site": "kungal", "actor_id": 7, "verb": "publish", "object_kind": "topic",
				"object_label": "话题", "day": "2026-09-26", "item_count": 2, "latest_at": "2026-09-26T04:00:00Z",
				"items": []any{map[string]any{
					"id": 11, "site": "kungal", "key": "topic_creation:1", "actor_id": 7, "verb": "publish",
					"object_kind": "topic", "object_label": "话题", "title": "t", "excerpt": "e",
					"url": "https://www.kungal.com/topic/1", "cover_image_hash": nil, "work_id": 42,
					"content_limit": "sfw", "occurred_at": "2026-09-26T04:00:00Z",
				}},
			}},
			"next_cursor": "",
		})
	})

	page, err := c.FollowingActivities(context.Background(), 3, "", 20, communityclient.FeedSFW)
	if err != nil {
		t.Fatalf("FollowingActivities: %v", err)
	}
	if gotPath != "/users/3/following/activities" || !strings.Contains(gotQuery, "content_limit=sfw") ||
		!strings.Contains(gotQuery, "limit=20") || strings.Contains(gotQuery, "cursor") {
		t.Errorf("request = %s?%s", gotPath, gotQuery)
	}
	g := page.Groups[0]
	if g.Day != "2026-09-26" || g.ItemCount != 2 || len(g.Items) != 1 || *g.Items[0].WorkID != 42 {
		t.Errorf("group = %+v", g)
	}
}

func TestMarkFollowingActivitiesSeenOmitsAnUnsetMark(t *testing.T) {
	var bodies []string
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/users/3/following/activities/seen" {
			t.Errorf("wrote to %s %s", r.Method, r.URL.Path)
		}
		raw, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(raw))
		envelope(w, map[string]any{"seen_at": "2026-09-26T04:00:00Z"})
	})
	if _, err := c.MarkFollowingActivitiesSeen(context.Background(), 3, nil); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 9, 26, 3, 0, 0, 0, time.UTC)
	if _, err := c.MarkFollowingActivitiesSeen(context.Background(), 3, &at); err != nil {
		t.Fatal(err)
	}
	if bodies[0] != "{}" || bodies[1] != `{"at":"2026-09-26T03:00:00Z"}` {
		t.Errorf("bodies = %q", bodies)
	}
}

func TestSetFollowNotifyNeverCreatesAFollow(t *testing.T) {
	var gotBody string
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch || r.URL.Path != "/users/3/following/9" {
			t.Errorf("wrote to %s %s", r.Method, r.URL.Path)
		}
		raw, _ := io.ReadAll(r.Body)
		gotBody = string(raw)
		if strings.Contains(gotBody, "feed") {
			envelope(w, map[string]any{"follower_id": 3, "followee_id": 9, "notify": "feed"})
			return
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":4,"message":"not following"}`))
	})

	res, err := c.SetFollowNotify(context.Background(), 3, 9, communityclient.FollowNotifyFeed)
	if err != nil || res.Notify != "feed" || gotBody != `{"notify":"feed"}` {
		t.Fatalf("res = %+v, err = %v, body = %s", res, err, gotBody)
	}
	_, err = c.SetFollowNotify(context.Background(), 3, 9, communityclient.FollowNotifyAll)
	if upstream.KindOf(err) != upstream.NotFound {
		t.Errorf("not following: err = %v", err)
	}
}

func TestNotificationFeedDecodesKind10(t *testing.T) {
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		envelope(w, map[string]any{
			"notifications": []any{
				map[string]any{
					"id": 1, "user_id": 3, "kind": 10, "thread_id": 0, "anchor_kind": 0, "anchor_id": "",
					"post_id": nil, "post_number": nil, "first_post_number": nil, "actor_id": 7,
					"actor_count": 1, "item_count": 2, "created_at": "2026-09-26T04:00:00Z",
					"updated_at": "2026-09-26T04:00:00Z", "seq": 90,
					"activity": map[string]any{
						"id": 11, "site": "moyu", "key": "patch_resource:9", "verb": "publish",
						"object_kind": "patch_resource", "object_label": "Galgame 补丁", "title": "t",
						"url": "https://www.moyu.moe/resource/9", "content_limit": "sfw",
						"occurred_at": "2026-09-26T04:00:00Z",
					},
				},
				map[string]any{
					"id": 2, "user_id": 3, "kind": 10, "thread_id": 0, "anchor_kind": 0, "anchor_id": "",
					"post_id": nil, "post_number": nil, "first_post_number": nil, "actor_id": 7,
					"actor_count": 1, "item_count": 0, "read_at": "2026-09-26T05:00:00Z",
					"created_at": "2026-09-26T04:00:00Z", "updated_at": "2026-09-26T05:00:00Z", "seq": 91,
				},
			},
			"next_after": 91,
		})
	})
	feed, err := c.NotificationFeed(context.Background(), 0, 500)
	if err != nil {
		t.Fatalf("NotificationFeed: %v", err)
	}
	live, gone := feed.Notifications[0], feed.Notifications[1]
	if live.Kind != communityclient.NotificationKindFolloweeActivity || live.Activity == nil ||
		live.Activity.ObjectLabel != "Galgame 补丁" || live.Seq != 90 || live.ItemCount != 2 {
		t.Errorf("live = %+v", live)
	}
	if gone.ItemCount != 0 || gone.Activity != nil || gone.ReadAt == "" {
		t.Errorf("retracted = %+v", gone)
	}
}

func TestFollowStatesDecodesViewerNotify(t *testing.T) {
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		envelope(w, map[string]any{"states": []any{
			map[string]any{"user_id": 8, "followers_count": 1, "following_count": 0,
				"viewer_follows": true, "follows_viewer": false, "viewer_notify": "feed"},
			map[string]any{"user_id": 9, "followers_count": 0, "following_count": 0,
				"viewer_follows": false, "follows_viewer": false, "viewer_notify": nil},
		}})
	})
	res, err := c.FollowStates(context.Background(), 3, []int64{8, 9})
	if err != nil {
		t.Fatal(err)
	}
	if n := res.States[0].ViewerNotify; n == nil || *n != "feed" {
		t.Errorf("following = %v", n)
	}
	if res.States[1].ViewerNotify != nil {
		t.Errorf("not following = %v", *res.States[1].ViewerNotify)
	}
}
