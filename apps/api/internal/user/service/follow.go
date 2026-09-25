package service

import (
	"context"
	"fmt"

	"kun-galgame-patch-api/internal/user/dto"
	"kun-galgame-patch-api/internal/user/model"
	"kun-galgame-patch-api/pkg/communityclient"
	"kun-galgame-patch-api/pkg/userclient"
)

var (
	ErrFollowSelf  = fmt.Errorf("cannot follow yourself")
	ErrUserMissing = fmt.Errorf("用户不存在")
)

type userPresence interface {
	Exists(id int) (bool, error)
}

type briefLookup interface {
	Briefs(ctx context.Context, ids []int) map[int]*userclient.Brief
}

type oauthBriefs struct{ c *userclient.Client }

func (o oauthBriefs) Briefs(ctx context.Context, ids []int) map[int]*userclient.Brief {
	return userclient.BriefMapByInt(ctx, o.c, ids)
}

func (s *UserService) Follow(ctx context.Context, followerID, followingID int) error {
	if followerID == followingID {
		return ErrFollowSelf
	}
	ok, err := s.presence.Exists(followingID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrUserMissing
	}
	_, err = s.community.FollowUser(ctx, int64(followerID), int64(followingID))
	return err
}

func (s *UserService) Unfollow(ctx context.Context, followerID, followingID int) error {
	ok, err := s.presence.Exists(followingID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrUserMissing
	}
	_, err = s.community.UnfollowUser(ctx, int64(followerID), int64(followingID))
	return err
}

func (s *UserService) GetFollowers(ctx context.Context, userID, viewerID int, cursor string, limit int) (*dto.FollowListResponse, error) {
	return s.followList(ctx, userID, viewerID, cursor, limit, true)
}

func (s *UserService) GetFollowing(ctx context.Context, userID, viewerID int, cursor string, limit int) (*dto.FollowListResponse, error) {
	return s.followList(ctx, userID, viewerID, cursor, limit, false)
}

func (s *UserService) followList(ctx context.Context, userID, viewerID int, cursor string, limit int, followers bool) (*dto.FollowListResponse, error) {
	var (
		page *communityclient.FollowListResponse
		err  error
	)
	if followers {
		page, err = s.community.ListFollowers(ctx, int64(userID), cursor, limit)
	} else {
		page, err = s.community.ListFollowing(ctx, int64(userID), cursor, limit)
	}
	if err != nil {
		return nil, err
	}

	ids := make([]int64, 0, len(page.Users)+1)
	ids = append(ids, int64(userID))
	seen := map[int64]struct{}{int64(userID): {}}
	itemIDs := make([]int, 0, len(page.Users))
	for _, u := range page.Users {
		itemIDs = append(itemIDs, int(u.UserID))
		if _, ok := seen[u.UserID]; ok {
			continue
		}
		seen[u.UserID] = struct{}{}
		ids = append(ids, u.UserID)
	}

	states, err := s.community.FollowStates(ctx, int64(viewerID), ids)
	if err != nil {
		return nil, err
	}
	byID := indexFollowStates(states.States)
	var total int64
	if st, ok := byID[int64(userID)]; ok {
		if followers {
			total = st.FollowersCount
		} else {
			total = st.FollowingCount
		}
	}

	return &dto.FollowListResponse{
		Items:      s.briefsToFollowItems(ctx, itemIDs, byID),
		NextCursor: page.NextCursor,
		Total:      total,
	}, nil
}

func (s *UserService) attachFollowState(ctx context.Context, resp *dto.UserInfoResponse, viewerID int) {
	if s.community == nil || !s.community.Configured() {
		return
	}
	res, err := s.community.FollowStates(ctx, int64(viewerID), []int64{int64(resp.ID)})
	if err != nil {
		communityclient.LogDegraded(ctx, "user follow states unavailable", err, "user_id", resp.ID)
		return
	}
	st, ok := indexFollowStates(res.States)[int64(resp.ID)]
	if !ok {
		return
	}
	followers := int(st.FollowersCount)
	following := int(st.FollowingCount)
	resp.FollowerCount = &followers
	resp.FollowingCount = &following
	resp.IsFollowed = st.ViewerFollows
}

func (s *UserService) briefsToFollowItems(ctx context.Context, ids []int, states map[int64]communityclient.FollowStateView) []model.UserFollowItem {
	briefs := s.briefLookup.Briefs(ctx, ids)
	out := make([]model.UserFollowItem, 0, len(ids))
	for _, id := range ids {
		if b := briefs[id]; b != nil {
			out = append(out, model.UserFollowItem{
				ID:         int(b.ID),
				Name:       b.Name,
				Avatar:     b.Avatar,
				Cosmetics:  b.Cosmetics,
				IsFollowed: states[int64(b.ID)].ViewerFollows,
			})
		}
	}
	return out
}

func indexFollowStates(states []communityclient.FollowStateView) map[int64]communityclient.FollowStateView {
	out := make(map[int64]communityclient.FollowStateView, len(states))
	for _, st := range states {
		out[st.UserID] = st
	}
	return out
}
