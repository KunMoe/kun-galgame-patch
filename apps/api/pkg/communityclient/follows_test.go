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

func TestFollowUser(t *testing.T) {
	var gotMethod, gotPath string
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		envelope(w, map[string]any{
			"follower_id": 3, "followee_id": 8, "following": true, "created": true,
		})
	})

	res, err := c.FollowUser(context.Background(), 3, 8)
	if err != nil {
		t.Fatalf("FollowUser: %v", err)
	}
	if gotMethod != http.MethodPut || gotPath != "/users/3/following/8" {
		t.Errorf("wrote to %s %s", gotMethod, gotPath)
	}
	if res.FollowerID != 3 || res.FolloweeID != 8 || !res.Following || !res.Created {
		t.Errorf("result = %+v", res)
	}
}

func TestUnfollowUser(t *testing.T) {
	var gotMethod, gotPath string
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		envelope(w, map[string]any{
			"follower_id": 3, "followee_id": 8, "following": false, "deleted": true,
		})
	})

	res, err := c.UnfollowUser(context.Background(), 3, 8)
	if err != nil {
		t.Fatalf("UnfollowUser: %v", err)
	}
	if gotMethod != http.MethodDelete || gotPath != "/users/3/following/8" {
		t.Errorf("wrote to %s %s", gotMethod, gotPath)
	}
	if res.FollowerID != 3 || res.FolloweeID != 8 || res.Following || !res.Deleted {
		t.Errorf("result = %+v", res)
	}
}

func TestListFollowers(t *testing.T) {
	var gotPath, gotQuery string
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.Path, r.URL.RawQuery
		envelope(w, map[string]any{
			"users": []any{
				map[string]any{"user_id": 8, "followed_at": "2026-09-16T00:00:00Z"},
				map[string]any{"user_id": 9, "followed_at": nil},
			},
			"next_cursor": "cur_9",
		})
	})

	page, err := c.ListFollowers(context.Background(), 3, "cur_1", 20)
	if err != nil {
		t.Fatalf("ListFollowers: %v", err)
	}
	if gotPath != "/users/3/followers" {
		t.Errorf("path = %q", gotPath)
	}
	for _, want := range []string{"cursor=cur_1", "limit=20"} {
		if !strings.Contains(gotQuery, want) {
			t.Errorf("query %q missing %q", gotQuery, want)
		}
	}
	if page.NextCursor != "cur_9" || len(page.Users) != 2 || page.Users[0].UserID != 8 {
		t.Errorf("page = %+v", page)
	}
	if page.Users[0].FollowedAt == nil || page.Users[1].FollowedAt != nil {
		t.Errorf("followed_at = %v / %v", page.Users[0].FollowedAt, page.Users[1].FollowedAt)
	}
}

func TestListFollowing(t *testing.T) {
	var gotPath, gotQuery string
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.Path, r.URL.RawQuery
		envelope(w, map[string]any{
			"users":       []any{map[string]any{"user_id": 4}},
			"next_cursor": "",
		})
	})

	page, err := c.ListFollowing(context.Background(), 3, "", 24)
	if err != nil {
		t.Fatalf("ListFollowing: %v", err)
	}
	if gotPath != "/users/3/following" {
		t.Errorf("path = %q", gotPath)
	}
	if !strings.Contains(gotQuery, "limit=24") {
		t.Errorf("query %q missing limit", gotQuery)
	}
	if strings.Contains(gotQuery, "cursor=") {
		t.Errorf("empty cursor was sent: %q", gotQuery)
	}
	if page.NextCursor != "" || len(page.Users) != 1 || page.Users[0].UserID != 4 {
		t.Errorf("page = %+v", page)
	}
}

func TestFollowStates(t *testing.T) {
	var gotPath string
	var body communityclient.FollowStatesRequest
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if r.Method != http.MethodPost {
			t.Errorf("method = %s", r.Method)
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		envelope(w, map[string]any{"states": []any{
			map[string]any{
				"user_id": 8, "followers_count": 11, "following_count": 4,
				"viewer_follows": true, "follows_viewer": false,
			},
		}})
	})

	res, err := c.FollowStates(context.Background(), 3, []int64{8})
	if err != nil {
		t.Fatalf("FollowStates: %v", err)
	}
	if gotPath != "/follows/states" {
		t.Errorf("path = %q", gotPath)
	}
	if body.ViewerID != 3 || len(body.UserIDs) != 1 || body.UserIDs[0] != 8 {
		t.Errorf("body = %+v", body)
	}
	if len(res.States) != 1 || res.States[0].UserID != 8 || res.States[0].FollowersCount != 11 ||
		!res.States[0].ViewerFollows || res.States[0].FollowsViewer {
		t.Errorf("states = %+v", res.States)
	}
}

func TestFollowStatesOmitsAnonymousViewer(t *testing.T) {
	var raw map[string]any
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&raw)
		envelope(w, map[string]any{"states": []any{}})
	})

	if _, err := c.FollowStates(context.Background(), 0, []int64{8}); err != nil {
		t.Fatalf("FollowStates: %v", err)
	}
	if _, ok := raw["viewer_id"]; ok {
		t.Errorf("anonymous viewer_id was sent: %v", raw["viewer_id"])
	}
}

func TestFollowStatesSkipsTheCallOnAnEmptyList(t *testing.T) {
	called := false
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		envelope(w, map[string]any{"states": []any{}})
	})
	res, err := c.FollowStates(context.Background(), 3, nil)
	if err != nil {
		t.Fatalf("FollowStates: %v", err)
	}
	if called {
		t.Error("an empty id list still reached the network")
	}
	if len(res.States) != 0 {
		t.Errorf("states = %+v", res.States)
	}
}

func TestFollowStatesBatchesAt100AndKeepsOrder(t *testing.T) {
	var calls []int
	var viewers []int64
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		var body communityclient.FollowStatesRequest
		_ = json.NewDecoder(r.Body).Decode(&body)
		calls = append(calls, len(body.UserIDs))
		viewers = append(viewers, body.ViewerID)
		states := make([]map[string]any, len(body.UserIDs))
		for i, id := range body.UserIDs {
			states[i] = map[string]any{"user_id": id, "followers_count": id}
		}
		envelope(w, map[string]any{"states": states})
	})

	ids := make([]int64, 150)
	for i := range ids {
		ids[i] = int64(i + 1)
	}
	res, err := c.FollowStates(context.Background(), 7, ids)
	if err != nil {
		t.Fatalf("FollowStates: %v", err)
	}
	if len(calls) != 2 || calls[0] != 100 || calls[1] != 50 {
		t.Errorf("batch sizes = %v, want 100 then 50", calls)
	}
	if len(viewers) != 2 || viewers[0] != 7 || viewers[1] != 7 {
		t.Errorf("viewer_id = %v", viewers)
	}
	if len(res.States) != 150 || res.States[0].UserID != 1 || res.States[99].UserID != 100 ||
		res.States[100].UserID != 101 || res.States[149].UserID != 150 {
		t.Errorf("order lost: first=%d mid=%d last=%d n=%d",
			res.States[0].UserID, res.States[100].UserID, res.States[149].UserID, len(res.States))
	}
}

func TestFollowUserLimitIsRejected(t *testing.T) {
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 7, "message": "following limit reached (max 5000)", "data": nil,
		})
	})
	_, err := c.FollowUser(context.Background(), 3, 8)
	if upstream.KindOf(err) != upstream.Rejected {
		t.Errorf("kind = %v, want Rejected; err = %v", upstream.KindOf(err), err)
	}
	if communityclient.RefusalOf(err) != communityclient.RefusalFollowingLimit {
		t.Errorf("refusal = %v", communityclient.RefusalOf(err))
	}
}
