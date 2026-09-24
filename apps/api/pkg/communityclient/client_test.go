package communityclient_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"kun-galgame-patch-api/pkg/communityclient"
)

func envelope(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "message": "成功", "data": data})
}

func newClient(t *testing.T, h http.HandlerFunc) (*communityclient.Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return communityclient.New(communityclient.Config{
		BaseURL: srv.URL, ClientID: "cid", ClientSecret: "secret",
	}), srv
}

// An anchor nobody has commented on has no thread at all. Every caller has to
// read that as an empty wall rather than dereferencing a nil.
func TestGetCommentsAnswersNoThreadBeforeTheFirstComment(t *testing.T) {
	var gotQuery string
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		if r.Header.Get("Authorization") == "" {
			t.Error("request carried no Basic credential")
		}
		envelope(w, map[string]any{"posts": []any{}, "next_cursor": ""})
	})

	page, err := c.GetComments(context.Background(), communityclient.AnchorSiteGame, "42", "", "30", 7)
	if err != nil {
		t.Fatalf("GetComments: %v", err)
	}
	if page.Thread != nil {
		t.Errorf("thread = %+v, want nil", page.Thread)
	}
	if len(page.Posts) != 0 {
		t.Errorf("posts = %d, want 0", len(page.Posts))
	}
	for _, want := range []string{"anchor_kind=1", "anchor_id=42", "limit=30", "viewer_id=7"} {
		if !strings.Contains(gotQuery, want) {
			t.Errorf("query %q missing %q", gotQuery, want)
		}
	}
}

func TestCommentOnAnchorReportsTheThreadItCreated(t *testing.T) {
	var body communityclient.CommentRequest
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/comments" {
			t.Errorf("wrote to %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		envelope(w, map[string]any{
			"thread": map[string]any{"id": 7, "anchor_kind": 1, "anchor_id": "42", "posts_count": 1},
			"post":   map[string]any{"id": 900, "thread_id": 7, "post_number": 1, "author_id": 3},
		})
	})

	res, err := c.CommentOnAnchor(context.Background(), communityclient.CommentRequest{
		AnchorKind: communityclient.AnchorSiteGame, AnchorID: "42",
		AuthorID: 3, Body: "hi",
	}, "")
	if err != nil {
		t.Fatalf("CommentOnAnchor: %v", err)
	}
	if res.Thread.ID != 7 || res.Post.ID != 900 {
		t.Errorf("got thread %d post %d", res.Thread.ID, res.Post.ID)
	}
	if body.AnchorID != "42" {
		t.Errorf("anchor_id = %q", body.AnchorID)
	}
}

// The site feed and search both need an explicit "every kind" — a zero Kind is
// 0=topic upstream, which would answer no comments at all.
func TestListSitePostsSendsExplicitAnyKind(t *testing.T) {
	var gotQuery string
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		envelope(w, map[string]any{"posts": []any{}, "next_cursor": ""})
	})

	_, err := c.ListSitePosts(context.Background(), communityclient.SitePostsQuery{
		Kind: communityclient.KindComments, AnchorKind: communityclient.AnyKind, Limit: 50,
	})
	if err != nil {
		t.Fatalf("ListSitePosts: %v", err)
	}
	for _, want := range []string{"kind=1", "anchor_kind=-1", "limit=50"} {
		if !strings.Contains(gotQuery, want) {
			t.Errorf("query %q missing %q", gotQuery, want)
		}
	}
}

func TestSearchPostsCarriesTheQuery(t *testing.T) {
	var gotPath, gotQuery string
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.Path, r.URL.RawQuery
		envelope(w, map[string]any{"posts": []any{}, "next_cursor": "cur_2"})
	})

	page, err := c.SearchPosts(context.Background(), "汉化", communityclient.KindComments, "", 24)
	if err != nil {
		t.Fatalf("SearchPosts: %v", err)
	}
	if gotPath != "/search/posts" {
		t.Errorf("path = %q", gotPath)
	}
	if !strings.Contains(gotQuery, "q=%E6%B1%89%E5%8C%96") {
		t.Errorf("query %q lost the term", gotQuery)
	}
	if page.NextCursor != "cur_2" {
		t.Errorf("next_cursor = %q", page.NextCursor)
	}
}

// MaxInt32 is how "all of it" is expressed: community clamps to the thread's
// highest post number, so the caller never has to know what that number is.
func TestMarkThreadReadSendsTheHighWaterMark(t *testing.T) {
	var body communityclient.ThreadReadRequest
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/threads/7/read" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		envelope(w, map[string]any{
			"thread_id": 7, "user_id": 3, "last_read_post_number": 12,
			"highest_post_number": 12, "unread_count": 0, "notification_level": 3,
		})
	})

	view, err := c.MarkThreadRead(context.Background(), 7, 3, 1<<31-1)
	if err != nil {
		t.Fatalf("MarkThreadRead: %v", err)
	}
	if body.LastReadPostNumber != 1<<31-1 {
		t.Errorf("last_read_post_number = %d", body.LastReadPostNumber)
	}
	if view.UnreadCount != 0 || view.NotificationLevel != communityclient.NotificationWatching {
		t.Errorf("view = %+v", view)
	}
}

// A thread the user never touched carries no row upstream and is simply absent
// from the answer — it is NOT reported as entirely unread.
func TestThreadStatesOmitsUntouchedThreads(t *testing.T) {
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/threads/states" {
			t.Errorf("path = %q", r.URL.Path)
		}
		envelope(w, map[string]any{"states": []any{
			map[string]any{"thread_id": 7, "user_id": 3, "unread_count": 2, "notification_level": 3},
		}})
	})

	res, err := c.ThreadStates(context.Background(), 3, []int64{7, 8, 9})
	if err != nil {
		t.Fatalf("ThreadStates: %v", err)
	}
	if len(res.States) != 1 || res.States[0].ThreadID != 7 {
		t.Errorf("states = %+v", res.States)
	}
}

func TestThreadStatesSkipsTheCallOnAnEmptyList(t *testing.T) {
	called := false
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		envelope(w, map[string]any{"states": []any{}})
	})
	if _, err := c.ThreadStates(context.Background(), 3, nil); err != nil {
		t.Fatalf("ThreadStates: %v", err)
	}
	if called {
		t.Error("an empty id list still reached the network")
	}
}

func TestDeletePostPassesAuthorAndModeratorFlag(t *testing.T) {
	var gotQuery string
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %s", r.Method)
		}
		gotQuery = r.URL.RawQuery
		envelope(w, nil)
	})

	if err := c.DeletePost(context.Background(), 900, 3, true); err != nil {
		t.Fatalf("DeletePost: %v", err)
	}
	for _, want := range []string{"author_id=3", "as_moderator=true"} {
		if !strings.Contains(gotQuery, want) {
			t.Errorf("query %q missing %q", gotQuery, want)
		}
	}
}

// A signed-out reader has no viewer, and 0 would be sent as a user id. The
// faces read it as "no viewer" either way, but only the absent parameter says so.
func TestSignedOutReadSendsNoViewer(t *testing.T) {
	var gotQuery string
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		envelope(w, map[string]any{
			"thread": map[string]any{"id": 7},
			"posts": []any{map[string]any{
				"id": 900, "thread_id": 7, "reaction_count": 4, "viewer_reacted": false,
			}},
		})
	})

	page, err := c.GetComments(context.Background(), communityclient.AnchorSiteGame, "42", "", "30", 0)
	if err != nil {
		t.Fatalf("GetComments: %v", err)
	}
	if strings.Contains(gotQuery, "viewer_id") {
		t.Errorf("query %q names a viewer", gotQuery)
	}
	// The like count is the primitive's now, not a mirror table's.
	if page.Posts[0].ReactionCount != 4 || page.Posts[0].ViewerReacted {
		t.Errorf("post = %+v", page.Posts[0])
	}
}

func TestTopAuthorsRanksBySite(t *testing.T) {
	var gotPath, gotQuery string
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.Path, r.URL.RawQuery
		envelope(w, map[string]any{"stats": []any{
			map[string]any{"author_id": 3, "visible_posts": 91},
			map[string]any{"author_id": 8, "visible_posts": 40},
		}})
	})

	res, err := c.TopAuthors(context.Background(), communityclient.KindComments, communityclient.AnyKind, 60)
	if err != nil {
		t.Fatalf("TopAuthors: %v", err)
	}
	if gotPath != "/authors/top" {
		t.Errorf("path = %q", gotPath)
	}
	for _, want := range []string{"kind=1", "anchor_kind=-1", "limit=60"} {
		if !strings.Contains(gotQuery, want) {
			t.Errorf("query %q missing %q", gotQuery, want)
		}
	}
	if len(res.Stats) != 2 || res.Stats[0].AuthorID != 3 {
		t.Errorf("stats = %+v", res.Stats)
	}
}

func TestNotificationFeedDecodesNullAndAbsentFields(t *testing.T) {
	var gotQuery string
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/notifications/feed" {
			t.Errorf("path = %q", r.URL.Path)
		}
		gotQuery = r.URL.RawQuery
		envelope(w, map[string]any{
			"notifications": []any{
				map[string]any{
					"id": 11, "user_id": 3, "kind": 1, "thread_id": 7,
					"anchor_kind": 1, "anchor_id": "42",
					"actor_id": nil, "post_id": nil, "read_at": nil,
					"actor_count": 1, "item_count": 1, "seq": 4,
					"created_at": "2026-09-16T00:00:00Z",
					"updated_at": "2026-09-16T00:00:00Z",
				},
				map[string]any{
					"id": 12, "user_id": 3, "kind": 5, "thread_id": 7,
					"anchor_kind": 1, "anchor_id": "42",
					"actor_count": 2, "item_count": 2, "seq": 5,
					"created_at": "2026-09-16T00:00:00Z",
					"updated_at": "2026-09-16T00:00:00Z",
				},
			},
			"next_after": 5,
		})
	})

	page, err := c.NotificationFeed(context.Background(), 0, 500)
	if err != nil {
		t.Fatalf("NotificationFeed: %v", err)
	}
	for _, want := range []string{"after=0", "limit=500"} {
		if !strings.Contains(gotQuery, want) {
			t.Errorf("query %q missing %q", gotQuery, want)
		}
	}
	if page.NextAfter != 5 || len(page.Notifications) != 2 {
		t.Fatalf("page = %+v", page)
	}
	first := page.Notifications[0]
	if first.ActorID != nil || first.PostID != nil || first.ReadAt != "" {
		t.Errorf("null fields decoded as %+v", first)
	}
	second := page.Notifications[1]
	if second.ActorID != nil || second.PostID != nil || second.ReadAt != "" {
		t.Errorf("absent fields decoded as %+v", second)
	}
}

func TestAnchorStatesSendsTheDocumentedBody(t *testing.T) {
	var body communityclient.AnchorStatesRequest
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/anchors/states" {
			t.Errorf("wrote to %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		envelope(w, map[string]any{"states": []any{
			map[string]any{"user_id": 3, "anchor_kind": 1, "anchor_id": "42", "notification_level": 3},
		}})
	})

	res, err := c.AnchorStates(context.Background(), 3, []communityclient.AnchorRef{
		{AnchorKind: communityclient.AnchorSiteGame, AnchorID: "42"},
	})
	if err != nil {
		t.Fatalf("AnchorStates: %v", err)
	}
	if body.UserID != 3 || len(body.Anchors) != 1 || body.Anchors[0].AnchorID != "42" {
		t.Errorf("body = %+v", body)
	}
	if len(res.States) != 1 || res.States[0].NotificationLevel != communityclient.NotificationWatching {
		t.Errorf("states = %+v", res.States)
	}
}

func TestAnchorStatesSkipsTheCallOnAnEmptyList(t *testing.T) {
	called := false
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		envelope(w, map[string]any{"states": []any{}})
	})
	if _, err := c.AnchorStates(context.Background(), 3, nil); err != nil {
		t.Fatalf("AnchorStates: %v", err)
	}
	if called {
		t.Error("an empty anchor list still reached the network")
	}
}

func TestSetAnchorNotificationSendsTheDocumentedBody(t *testing.T) {
	var body communityclient.AnchorNotificationRequest
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/anchors/notification" {
			t.Errorf("wrote to %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		envelope(w, map[string]any{
			"user_id": 3, "anchor_kind": 2, "anchor_id": "678", "notification_level": 3,
		})
	})

	view, err := c.SetAnchorNotification(context.Background(), 3, communityclient.AnchorSiteResource, "678", communityclient.NotificationWatching)
	if err != nil {
		t.Fatalf("SetAnchorNotification: %v", err)
	}
	if body.UserID != 3 || body.AnchorKind != 2 || body.AnchorID != "678" || body.Level != 3 {
		t.Errorf("body = %+v", body)
	}
	if view.NotificationLevel != communityclient.NotificationWatching {
		t.Errorf("view = %+v", view)
	}
}

func TestMarkNotificationsReadSendsIDs(t *testing.T) {
	var gotPath string
	var body communityclient.MarkNotificationsReadRequest
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&body)
		envelope(w, map[string]any{"marked": 2, "unread_count": 4})
	})

	res, err := c.MarkNotificationsRead(context.Background(), 3, []int64{11, 12})
	if err != nil {
		t.Fatalf("MarkNotificationsRead: %v", err)
	}
	if gotPath != "/users/3/notifications/read" {
		t.Errorf("path = %q", gotPath)
	}
	if len(body.IDs) != 2 || body.IDs[0] != 11 || body.IDs[1] != 12 {
		t.Errorf("body = %+v", body)
	}
	if res.Marked != 2 || res.UnreadCount != 4 {
		t.Errorf("result = %+v", res)
	}
}

func TestCommentOnAnchorSerialisesMentionUserIDs(t *testing.T) {
	var raw map[string]any
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&raw)
		envelope(w, map[string]any{
			"thread": map[string]any{"id": 7},
			"post":   map[string]any{"id": 900},
		})
	})

	if _, err := c.CommentOnAnchor(context.Background(), communityclient.CommentRequest{
		AnchorKind: communityclient.AnchorSiteGame, AnchorID: "42",
		AuthorID: 3, Body: "hi", MentionUserIDs: []int64{2, 5},
	}, ""); err != nil {
		t.Fatalf("CommentOnAnchor: %v", err)
	}
	ids, _ := raw["mention_user_ids"].([]any)
	if len(ids) != 2 || ids[0] != float64(2) || ids[1] != float64(5) {
		t.Errorf("mention_user_ids = %v", raw["mention_user_ids"])
	}
}

func TestCommentOnAnchorOmitsEmptyMentionUserIDs(t *testing.T) {
	var raw map[string]any
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&raw)
		envelope(w, map[string]any{
			"thread": map[string]any{"id": 7},
			"post":   map[string]any{"id": 900},
		})
	})

	if _, err := c.CommentOnAnchor(context.Background(), communityclient.CommentRequest{
		AnchorKind: communityclient.AnchorSiteGame, AnchorID: "42",
		AuthorID: 3, Body: "hi",
	}, ""); err != nil {
		t.Fatalf("CommentOnAnchor: %v", err)
	}
	if _, ok := raw["mention_user_ids"]; ok {
		t.Errorf("empty mention_user_ids was sent: %v", raw["mention_user_ids"])
	}
}

// PUT and DELETE /posts/{id}/reaction (infra #302, handler/s2s.go
// setReaction / unsetReaction) answer the toggle's shape plus `changed`, false
// when the reaction was already as asked, which is what a retry sees.
func TestSetAndUnsetReaction(t *testing.T) {
	var gotMethod, gotQuery string
	var body map[string]any
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/posts/900/reaction" {
			t.Errorf("path = %q", r.URL.Path)
		}
		gotMethod, gotQuery, body = r.Method, r.URL.RawQuery, nil
		_ = json.NewDecoder(r.Body).Decode(&body)
		envelope(w, map[string]any{
			"added": r.Method == http.MethodPut, "changed": false, "reaction_count": 5,
			"author_id": 3, "thread_id": 7, "anchor_kind": 1, "anchor_id": "42",
		})
	})

	set, err := c.SetReaction(context.Background(), 900, 8, communityclient.ReactionLike)
	if err != nil {
		t.Fatalf("SetReaction: %v", err)
	}
	if gotMethod != http.MethodPut || body["user_id"] != float64(8) || body["kind"] != float64(0) {
		t.Errorf("sent %s %v", gotMethod, body)
	}
	if !set.Added || set.Changed || set.ReactionCount != 5 || set.AuthorID != 3 {
		t.Errorf("set = %+v", set)
	}

	unset, err := c.UnsetReaction(context.Background(), 900, 8, communityclient.ReactionLike)
	if err != nil {
		t.Fatalf("UnsetReaction: %v", err)
	}
	if gotMethod != http.MethodDelete || gotQuery != "kind=0&user_id=8" || body != nil {
		t.Errorf("sent %s ?%s %v; the unset face is body-free", gotMethod, gotQuery, body)
	}
	if unset.Added || unset.Changed {
		t.Errorf("unset = %+v", unset)
	}
}
