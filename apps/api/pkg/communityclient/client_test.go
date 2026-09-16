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
	})
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

// The TL0 sandbox answers 429, and it has to stay distinguishable from every
// other 4xx: the caller turns it into 发表过于频繁 rather than a generic failure.
func TestRateLimitIsItsOwnError(t *testing.T) {
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	})
	_, err := c.CommentOnAnchor(context.Background(), communityclient.CommentRequest{})
	if err != communityclient.ErrRateLimited {
		t.Errorf("err = %v, want ErrRateLimited", err)
	}
}

// A client with no site binding is refused on every write. That is a deployment
// fault, not a user one, so it does not reach the reader as a 400.
func TestForbiddenIsItsOwnError(t *testing.T) {
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})
	if _, err := c.GetComments(context.Background(), 1, "1", "", "", 0); err != communityclient.ErrForbidden {
		t.Errorf("err = %v, want ErrForbidden", err)
	}
}

func TestUnconfiguredClientNeverDials(t *testing.T) {
	c := communityclient.New(communityclient.Config{})
	if c.Configured() {
		t.Fatal("Configured() is true with no base URL")
	}
	if _, err := c.GetComments(context.Background(), 1, "1", "", "", 0); err != communityclient.ErrNotConfigured {
		t.Errorf("err = %v, want ErrNotConfigured", err)
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

// A non-zero envelope code is an error even under HTTP 200: the house envelope
// carries the failure, not the status line.
func TestNonZeroEnvelopeCodeIsAnError(t *testing.T) {
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 40001, "message": "bad anchor"})
	})
	_, err := c.GetComments(context.Background(), 1, "1", "", "", 0)
	var apiErr *communityclient.APIError
	if !errorsAs(err, &apiErr) || apiErr.Code != 40001 {
		t.Fatalf("err = %v, want APIError code 40001", err)
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

func errorsAs(err error, target **communityclient.APIError) bool {
	e, ok := err.(*communityclient.APIError)
	if ok {
		*target = e
	}
	return ok
}
