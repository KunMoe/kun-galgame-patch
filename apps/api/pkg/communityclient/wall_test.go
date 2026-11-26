package communityclient_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"kun-galgame-patch-api/pkg/communityclient"
	"kun-galgame-patch-api/pkg/upstream"
)

func TestGetCommentsSendsTheWallShape(t *testing.T) {
	var gotQuery string
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		envelope(w, map[string]any{
			"thread":         map[string]any{"id": 7},
			"pinned_posts":   []any{map[string]any{"id": 5, "pinned_at": "2026-11-26T00:00:00Z", "reply_count": 2}},
			"posts":          []any{map[string]any{"id": 5, "reply_count": 2}, map[string]any{"id": 6, "status": 2, "delete_source": "moderator"}},
			"reply_previews": []any{map[string]any{"id": 8, "root_post_id": 5}},
			"next_cursor":    "bjo2",
		})
	})

	page, err := c.GetComments(context.Background(), communityclient.AnchorSiteGame, "42", communityclient.CommentsQuery{
		Sort: communityclient.SortTop, Cursor: "dDoxOjU", Limit: 30, RootsOnly: true, ReplyPreview: 3,
	}, 0)
	if err != nil {
		t.Fatalf("GetComments: %v", err)
	}
	for _, want := range []string{"sort=top", "cursor=dDoxOjU", "limit=30", "roots_only=true", "reply_preview=3"} {
		if !strings.Contains(gotQuery, want) {
			t.Errorf("query %q missing %q", gotQuery, want)
		}
	}
	if strings.Contains(gotQuery, "after=") {
		t.Errorf("query %q sends after beside a cursor, which is a 422", gotQuery)
	}
	if len(page.PinnedPosts) != 1 || page.PinnedPosts[0].PinnedAt == "" || *page.PinnedPosts[0].ReplyCount != 2 {
		t.Errorf("pinned = %+v", page.PinnedPosts)
	}
	if page.Posts[1].DeleteSource != communityclient.DeletedByModerator {
		t.Errorf("delete_source = %q", page.Posts[1].DeleteSource)
	}
	if len(page.ReplyPreviews) != 1 || page.ReplyPreviews[0].RootPostID != 5 {
		t.Errorf("previews = %+v", page.ReplyPreviews)
	}
}

func TestPinUnpinAndRestoreFaces(t *testing.T) {
	type call struct{ method, path, query, actor string }
	var got []call
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		var body communityclient.PostActorRequest
		_ = json.NewDecoder(r.Body).Decode(&body)
		actor := ""
		if body.ActorID != 0 {
			actor = "body"
		}
		got = append(got, call{r.Method, r.URL.Path, r.URL.RawQuery, actor})
		envelope(w, map[string]any{"post": map[string]any{"id": 900}, "changed": true})
	})
	ctx := context.Background()
	for _, do := range []func() (*communityclient.PostChange, error){
		func() (*communityclient.PostChange, error) { return c.PinPost(ctx, 900, 9) },
		func() (*communityclient.PostChange, error) { return c.UnpinPost(ctx, 900, 9) },
		func() (*communityclient.PostChange, error) { return c.RestorePost(ctx, 900, 9) },
	} {
		res, err := do()
		if err != nil || !res.Changed || res.Post.ID != 900 {
			t.Fatalf("res = %+v, err = %v", res, err)
		}
	}
	want := []call{
		{http.MethodPut, "/posts/900/pin", "", "body"},
		{http.MethodDelete, "/posts/900/pin", "actor_id=9", ""},
		{http.MethodPost, "/posts/900/restore", "", "body"},
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("call %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestThePinLimitIsItsOwnRefusal(t *testing.T) {
	_, err := fake(t, answer{http.StatusConflict, `{"code":10,"message":"pin limit reached (max 3 per wall)"}`}).PinPost(context.Background(), 900, 9)
	if upstream.KindOf(err) != upstream.Conflict || communityclient.RefusalOf(err) != communityclient.RefusalPinLimit {
		t.Fatalf("err = %v, refusal = %v", err, communityclient.RefusalOf(err))
	}
	_, err = fake(t, answer{http.StatusConflict, `{"code":10,"message":"only a visible comment can be pinned"}`}).PinPost(context.Background(), 900, 9)
	if communityclient.RefusalOf(err) == communityclient.RefusalPinLimit {
		t.Fatal("a hidden post read as the pin limit")
	}
	_, err = fake(t, answer{http.StatusConflict, `{"code":10,"message":"the comment moved to another wall; retry"}`}).PinPost(context.Background(), 900, 9)
	if communityclient.RefusalOf(err) != communityclient.RefusalPinMoved {
		t.Fatalf("a merge race read as %v", communityclient.RefusalOf(err))
	}
}
