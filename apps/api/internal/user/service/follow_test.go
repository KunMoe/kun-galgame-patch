package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"kun-galgame-patch-api/internal/user/dto"
	"kun-galgame-patch-api/pkg/communityclient"
	"kun-galgame-patch-api/pkg/userclient"
)

func envelope(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "message": "成功", "data": data})
}

func newFollowClient(t *testing.T, h http.HandlerFunc) *communityclient.Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return communityclient.New(communityclient.Config{
		BaseURL: srv.URL, ClientID: "cid", ClientSecret: "secret",
	})
}

type fakePresence map[int]bool

func (f fakePresence) Exists(id int) (bool, error) {
	return f[id], nil
}

type fakeBriefs map[int]*userclient.Brief

func (f fakeBriefs) Briefs(_ context.Context, ids []int) map[int]*userclient.Brief {
	out := map[int]*userclient.Brief{}
	for _, id := range ids {
		if b := f[id]; b != nil {
			out[id] = b
		}
	}
	return out
}

func TestFollowRepeatSucceeds(t *testing.T) {
	var n int
	svc := &UserService{
		community: newFollowClient(t, func(w http.ResponseWriter, r *http.Request) {
			n++
			if r.Method != http.MethodPut || r.URL.Path != "/users/3/following/8" {
				t.Errorf("wrote to %s %s", r.Method, r.URL.Path)
			}
			envelope(w, map[string]any{
				"follower_id": 3, "followee_id": 8, "following": true, "created": n == 1,
			})
		}),
		presence: fakePresence{8: true},
	}
	if err := svc.Follow(context.Background(), 3, 8); err != nil {
		t.Fatalf("first Follow: %v", err)
	}
	if err := svc.Follow(context.Background(), 3, 8); err != nil {
		t.Fatalf("repeat Follow: %v", err)
	}
	if n != 2 {
		t.Errorf("calls = %d", n)
	}
}

func TestFollowSelf(t *testing.T) {
	called := false
	svc := &UserService{
		community: newFollowClient(t, func(w http.ResponseWriter, r *http.Request) {
			called = true
			envelope(w, map[string]any{})
		}),
		presence: fakePresence{3: true},
	}
	err := svc.Follow(context.Background(), 3, 3)
	if !errors.Is(err, ErrFollowSelf) {
		t.Errorf("err = %v", err)
	}
	if called {
		t.Error("self-follow reached community")
	}
}

func TestFollowUnknownTarget(t *testing.T) {
	called := false
	svc := &UserService{
		community: newFollowClient(t, func(w http.ResponseWriter, r *http.Request) {
			called = true
			envelope(w, map[string]any{})
		}),
		presence: fakePresence{},
	}
	err := svc.Follow(context.Background(), 3, 8)
	if !errors.Is(err, ErrUserMissing) {
		t.Errorf("err = %v", err)
	}
	if called {
		t.Error("unknown target reached community")
	}
}

func TestUnfollowRepeatSucceeds(t *testing.T) {
	var n int
	svc := &UserService{
		community: newFollowClient(t, func(w http.ResponseWriter, r *http.Request) {
			n++
			if r.Method != http.MethodDelete || r.URL.Path != "/users/3/following/8" {
				t.Errorf("wrote to %s %s", r.Method, r.URL.Path)
			}
			envelope(w, map[string]any{
				"follower_id": 3, "followee_id": 8, "following": false, "deleted": n == 1,
			})
		}),
		presence: fakePresence{8: true},
	}
	if err := svc.Unfollow(context.Background(), 3, 8); err != nil {
		t.Fatalf("first Unfollow: %v", err)
	}
	if err := svc.Unfollow(context.Background(), 3, 8); err != nil {
		t.Fatalf("repeat Unfollow: %v", err)
	}
}

func TestUnfollowUnknownTarget(t *testing.T) {
	called := false
	svc := &UserService{
		community: newFollowClient(t, func(w http.ResponseWriter, r *http.Request) {
			called = true
			envelope(w, map[string]any{})
		}),
		presence: fakePresence{},
	}
	err := svc.Unfollow(context.Background(), 3, 8)
	if !errors.Is(err, ErrUserMissing) {
		t.Errorf("err = %v", err)
	}
	if called {
		t.Error("unknown target reached community")
	}
}

func TestFollowListCursorTotalDropsAndIsFollowed(t *testing.T) {
	svc := &UserService{
		community: newFollowClient(t, func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/users/2/followers":
				if !strings.Contains(r.URL.RawQuery, "cursor=cur_1") || !strings.Contains(r.URL.RawQuery, "limit=20") {
					t.Errorf("query = %q", r.URL.RawQuery)
				}
				envelope(w, map[string]any{
					"users": []any{
						map[string]any{"user_id": 3},
						map[string]any{"user_id": 4},
						map[string]any{"user_id": 5},
					},
					"next_cursor": "cur_9",
				})
			case r.Method == http.MethodPost && r.URL.Path == "/follows/states":
				var body communityclient.FollowStatesRequest
				_ = json.NewDecoder(r.Body).Decode(&body)
				if body.ViewerID != 9 {
					t.Errorf("viewer_id = %d", body.ViewerID)
				}
				envelope(w, map[string]any{"states": []any{
					map[string]any{"user_id": 2, "followers_count": 10, "following_count": 1},
					map[string]any{"user_id": 3, "viewer_follows": true},
					map[string]any{"user_id": 4, "viewer_follows": false},
					map[string]any{"user_id": 5, "viewer_follows": true},
				}})
			default:
				t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
				envelope(w, map[string]any{})
			}
		}),
		briefLookup: fakeBriefs{
			3: {ID: 3, Name: "a"},
			4: {ID: 4, Name: "b"},
		},
	}

	page, err := svc.GetFollowers(context.Background(), 2, 9, "cur_1", 20)
	if err != nil {
		t.Fatalf("GetFollowers: %v", err)
	}
	if page.NextCursor != "cur_9" || page.Total != 10 {
		t.Errorf("cursor/total = %q / %d", page.NextCursor, page.Total)
	}
	if len(page.Items) != 2 || page.Items[0].ID != 3 || page.Items[1].ID != 4 {
		t.Errorf("items = %+v", page.Items)
	}
	if !page.Items[0].IsFollowed || page.Items[1].IsFollowed {
		t.Errorf("is_followed = %v / %v", page.Items[0].IsFollowed, page.Items[1].IsFollowed)
	}
}

func TestFollowListAnonymousViewerIsNotFollowed(t *testing.T) {
	svc := &UserService{
		community: newFollowClient(t, func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.URL.Path == "/users/2/following":
				envelope(w, map[string]any{
					"users": []any{map[string]any{"user_id": 3}},
				})
			case r.URL.Path == "/follows/states":
				var raw map[string]any
				_ = json.NewDecoder(r.Body).Decode(&raw)
				if _, ok := raw["viewer_id"]; ok {
					t.Errorf("anonymous viewer_id was sent: %v", raw["viewer_id"])
				}
				envelope(w, map[string]any{"states": []any{
					map[string]any{"user_id": 2, "following_count": 4},
					map[string]any{"user_id": 3, "viewer_follows": false},
				}})
			default:
				t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
				envelope(w, map[string]any{})
			}
		}),
		briefLookup: fakeBriefs{3: {ID: 3, Name: "a"}},
	}

	page, err := svc.GetFollowing(context.Background(), 2, 0, "", 20)
	if err != nil {
		t.Fatalf("GetFollowing: %v", err)
	}
	if page.Total != 4 || len(page.Items) != 1 || page.Items[0].IsFollowed {
		t.Errorf("page = %+v", page)
	}
}

func TestProfileOmitsCountsWhenFollowStatesFails(t *testing.T) {
	svc := &UserService{
		community: newFollowClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 1, "message": "down"})
		}),
	}
	resp := &dto.UserInfoResponse{ID: 2, IsFollowed: true}
	svc.attachFollowState(context.Background(), resp, 9)
	if resp.FollowerCount != nil || resp.FollowingCount != nil {
		t.Errorf("counts = %v / %v", resp.FollowerCount, resp.FollowingCount)
	}
	if !resp.IsFollowed {
		t.Error("failed followStates overwrote is_followed")
	}
}

func TestProfileFillsCountsAndIsFollowed(t *testing.T) {
	svc := &UserService{
		community: newFollowClient(t, func(w http.ResponseWriter, r *http.Request) {
			var body communityclient.FollowStatesRequest
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body.ViewerID != 9 || len(body.UserIDs) != 1 || body.UserIDs[0] != 2 {
				t.Errorf("body = %+v", body)
			}
			envelope(w, map[string]any{"states": []any{
				map[string]any{
					"user_id": 2, "followers_count": 11, "following_count": 4,
					"viewer_follows": true,
				},
			}})
		}),
	}
	resp := &dto.UserInfoResponse{ID: 2}
	svc.attachFollowState(context.Background(), resp, 9)
	if resp.FollowerCount == nil || *resp.FollowerCount != 11 {
		t.Errorf("follower_count = %v", resp.FollowerCount)
	}
	if resp.FollowingCount == nil || *resp.FollowingCount != 4 {
		t.Errorf("following_count = %v", resp.FollowingCount)
	}
	if !resp.IsFollowed {
		t.Error("is_followed stayed false")
	}
}
