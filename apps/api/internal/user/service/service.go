package service

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"kun-galgame-patch-api/internal/favorite"
	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/internal/galgame/enricher"
	patchModel "kun-galgame-patch-api/internal/patch/model"
	"kun-galgame-patch-api/internal/user/dto"
	"kun-galgame-patch-api/internal/user/model"
	"kun-galgame-patch-api/internal/user/repository"
	"kun-galgame-patch-api/pkg/moemoepoint"
	"kun-galgame-patch-api/pkg/userclient"
	"kun-galgame-patch-api/pkg/utils"

	"gorm.io/gorm"
)

type UserService struct {
	repo    *repository.UserRepository
	users   *userclient.Client
	galgame *galgameClient.Client
	db      *gorm.DB
	mp      *moemoepoint.Awarder
}

func New(
	repo *repository.UserRepository,
	users *userclient.Client,
	galgame *galgameClient.Client,
	db *gorm.DB,
	mp *moemoepoint.Awarder,
) *UserService {
	return &UserService{repo: repo, users: users, galgame: galgame, db: db, mp: mp}
}

type patchSummaryFinder struct{ db *gorm.DB }

func (p patchSummaryFinder) LookupPatchesByIDs(ids []int) ([]patchModel.Patch, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var rows []patchModel.Patch
	err := p.db.Select("id", "vndb_id").Where("id IN ?", ids).Find(&rows).Error
	return rows, err
}

func (s *UserService) attachPatchSummaries(ctx context.Context, comments []patchModel.PatchComment, resources []patchModel.PatchResource) {
	if len(comments) == 0 && len(resources) == 0 {
		return
	}
	idSet := make(map[int]struct{}, len(comments)+len(resources))
	for _, c := range comments {
		idSet[c.GalgameID] = struct{}{}
	}
	for _, r := range resources {
		idSet[r.GalgameID] = struct{}{}
	}
	if len(idSet) == 0 {
		return
	}
	ids := make([]int, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}

	summaries := enricher.BuildPatchSummaryMap(ctx, s.galgame, patchSummaryFinder{db: s.db}, ids)
	for i := range comments {
		if sum, ok := summaries[comments[i].GalgameID]; ok {
			cp := sum
			comments[i].Patch = &cp
		}
	}
	for i := range resources {
		if sum, ok := summaries[resources[i].GalgameID]; ok {
			cp := sum
			resources[i].Patch = &cp
		}
	}
}

func (s *UserService) GetUserInfo(ctx context.Context, userID, currentUID int, token, contentLimit string) (*dto.UserInfoResponse, error) {
	user, err := s.repo.FindByID(userID)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}

	resp := &dto.UserInfoResponse{
		ID:             user.ID,
		Moemoepoint:    user.Moemoepoint,
		FollowerCount:  user.FollowerCount,
		FollowingCount: user.FollowingCount,
		RegisterTime:   user.Created.Format(time.RFC3339),
		PatchCount:     s.repo.CountUserPatches(userID),
		ResourceCount:  s.repo.CountUserResources(userID),
		CommentCount:   s.repo.CountUserComments(userID),
		FavoriteCount:  s.countFavorites(ctx, userID, token, currentUID == userID, contentLimit),
	}

	if s.users != nil {
		if b, _ := s.users.User(ctx, uint(userID)); b != nil {
			resp.Name = b.Name
			resp.Avatar = b.Avatar
			resp.Bio = b.Bio
			resp.Roles = b.Roles
			resp.SiteRoles = b.SiteRoles
		}
	}

	if currentUID > 0 && currentUID != userID {
		_, err := s.repo.FindFollow(currentUID, userID)
		resp.IsFollowed = err == nil
	}

	return resp, nil
}

// countFavorites is the number the 收藏 tab will show: the same folders, the
// same reader, the same gate. It used to count user_patch_favorite_relation,
// which the cutover froze — a profile could say 10 over a list the reader was
// not allowed to see a single row of. A catalog outage costs the number, not
// the profile, which is what the local count did on a failed query too.
func (s *UserService) countFavorites(ctx context.Context, userID int, token string, isOwner bool, contentLimit string) int64 {
	ids, err := s.favoritePatchIDs(ctx, userID, token, isOwner)
	if err == nil {
		var total int64
		total, err = s.repo.CountUserFavoritesByIDs(ids, true, contentLimit)
		if err == nil {
			return total
		}
	}
	slog.Warn("user profile count failed", "what", "favorites", "user_id", userID, "error", err)
	return 0
}

func (s *UserService) GetUserFloating(ctx context.Context, userID int) (*dto.UserInfoResponse, error) {
	return s.GetUserInfo(ctx, userID, 0, "", utils.ContentLimitSFW)
}

func (s *UserService) Follow(followerID, followingID int) error {
	if followerID == followingID {
		return fmt.Errorf("cannot follow yourself")
	}

	_, err := s.repo.FindFollow(followerID, followingID)
	if err == nil {
		return fmt.Errorf("already following this user")
	}

	if err := s.repo.CreateFollowAndIncrement(followerID, followingID); err != nil {
		if strings.Contains(err.Error(), "violates foreign key") || strings.Contains(err.Error(), "23503") {
			return fmt.Errorf("用户不存在")
		}
		return err
	}
	return nil
}

func (s *UserService) Unfollow(followerID, followingID int) error {
	affected, err := s.repo.DeleteFollowAndDecrement(followerID, followingID)
	if err != nil {
		return err
	}
	if affected == 0 {
		return fmt.Errorf("not following this user")
	}
	return nil
}

func (s *UserService) GetFollowers(ctx context.Context, userID, viewerID, page, limit int) ([]model.UserFollowItem, int64, error) {
	ids, total, err := s.repo.GetFollowerIDs(userID, (page-1)*limit, limit)
	if err != nil {
		return nil, 0, err
	}
	return s.briefsToFollowItems(ctx, ids, viewerID), total, nil
}

func (s *UserService) GetFollowing(ctx context.Context, userID, viewerID, page, limit int) ([]model.UserFollowItem, int64, error) {
	ids, total, err := s.repo.GetFollowingIDs(userID, (page-1)*limit, limit)
	if err != nil {
		return nil, 0, err
	}
	return s.briefsToFollowItems(ctx, ids, viewerID), total, nil
}

func (s *UserService) briefsToFollowItems(ctx context.Context, ids []int, viewerID int) []model.UserFollowItem {
	briefs := userclient.BriefMapByInt(ctx, s.users, ids)
	followed, _ := s.repo.WhichFollowed(viewerID, ids)
	out := make([]model.UserFollowItem, 0, len(ids))
	for _, id := range ids {
		if b := briefs[id]; b != nil {
			out = append(out, model.UserFollowItem{
				ID:         int(b.ID),
				Name:       b.Name,
				Avatar:     b.Avatar,
				IsFollowed: followed[int(b.ID)],
			})
		}
	}
	return out
}

func (s *UserService) SearchUsers(ctx context.Context, query string, limit int) ([]model.UserBasic, error) {
	if s.users == nil {
		return []model.UserBasic{}, nil
	}
	if limit <= 0 || limit > 50 {
		limit = 50
	}
	briefs, err := s.users.Search(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	out := make([]model.UserBasic, 0, len(briefs))
	for _, b := range briefs {
		out = append(out, model.UserBasic{ID: int(b.ID), Name: b.Name, Avatar: b.Avatar, AvatarImageHash: b.AvatarImageHash})
	}
	return out, nil
}

func (s *UserService) CheckIn(userID int) (int, error) {
	affected, err := s.repo.CheckIn(userID)
	if err != nil {
		return 0, err
	}
	if affected == 0 {
		return 0, fmt.Errorf("already checked in today")
	}

	points := rand.Intn(8)
	loc, lerr := time.LoadLocation("Asia/Shanghai")
	if lerr != nil || loc == nil {
		loc = time.Local
	}
	date := time.Now().In(loc).Format("2006-01-02")
	go s.mp.Award(context.Background(), userID, points, "daily_checkin", "",
		fmt.Sprintf("moyu:checkin:%d:%s", userID, date))
	return points, nil
}

func (s *UserService) GetMoemoepointLog(ctx context.Context, userID, limit int, beforeID int64, reason string) ([]moemoepoint.LogEntry, bool, error) {
	items, hasMore, err := s.mp.Log(ctx, userID, limit, beforeID, reason)
	if err != nil {
		return items, hasMore, err
	}
	s.attachMoemoepointLinks(items)
	return items, hasMore, nil
}

func (s *UserService) attachMoemoepointLinks(items []moemoepoint.LogEntry) {
	commentIDs := make([]int, 0)
	for i := range items {
		if !items[i].IsLocal {
			continue
		}
		if kind, id := parseRef(items[i].Ref); kind == "comment" && id > 0 {
			commentIDs = append(commentIDs, id)
		}
	}
	galgameByComment := map[int]int{}
	if len(commentIDs) > 0 {
		var rows []struct {
			ID        int
			GalgameID int
		}
		s.db.Model(&patchModel.PatchComment{}).
			Select("id", "galgame_id").
			Where("id IN ?", commentIDs).
			Scan(&rows)
		for _, r := range rows {
			galgameByComment[r.ID] = r.GalgameID
		}
	}
	for i := range items {
		if !items[i].IsLocal {
			continue
		}
		kind, id := parseRef(items[i].Ref)
		if id <= 0 {
			continue
		}
		switch kind {
		case "resource":
			items[i].Link = fmt.Sprintf("/resource/%d", id)
		case "galgame", "patch":
			items[i].Link = fmt.Sprintf("/patch/%d", id)
		case "comment":
			if gid := galgameByComment[id]; gid > 0 {
				items[i].Link = fmt.Sprintf("/patch/%d/comment#comment-%d", gid, id)
			}
		}
	}
}

func parseRef(ref string) (kind string, id int) {
	k, rest, ok := strings.Cut(ref, ":")
	if !ok {
		return "", 0
	}
	id, _ = strconv.Atoi(rest)
	return k, id
}

func (s *UserService) GetUserPatches(userID, page, limit int, includeEmpty bool, contentLimit string) ([]patchModel.Patch, int64, error) {
	return s.repo.GetUserPatches(userID, (page-1)*limit, limit, includeEmpty, contentLimit)
}

func (s *UserService) GetUserResources(ctx context.Context, userID, page, limit int) ([]patchModel.PatchResource, int64, error) {
	rs, total, err := s.repo.GetUserResources(userID, (page-1)*limit, limit)
	if err != nil {
		return rs, total, err
	}
	patchModel.RenderResourceNotes(rs)
	s.attachResourceUsers(ctx, rs)
	s.attachPatchSummaries(ctx, nil, rs)
	return rs, total, nil
}

// GetUserFavorites reads the person's catalog folders and turns the works they
// hold back into this site's patches. Their own shelf is read with their own
// token so private folders count; anybody else sees only what they published.
//
// Works this site does not carry are dropped rather than shown as holes: the
// folders are shared with the forum, where somebody can favourite a game that
// has no patch page here.
func (s *UserService) GetUserFavorites(ctx context.Context, userID int, token string, isOwner bool,
	page, limit int, includeEmpty bool, contentLimit string) ([]patchModel.Patch, int64, error) {

	ids, err := s.favoritePatchIDs(ctx, userID, token, isOwner)
	if err != nil {
		return nil, 0, err
	}
	return s.repo.GetUserFavoritesByIDs(ids, (page-1)*limit, limit, includeEmpty, contentLimit)
}

// favoritePatchIDs is the pages of this site that the reader's view of the
// person's folders resolves to. The profile counter and the tab both go
// through it so the number and the list cannot disagree again.
func (s *UserService) favoritePatchIDs(ctx context.Context, userID int, token string, isOwner bool) ([]int, error) {
	workIDs, err := favorite.WorkIDs(ctx, s.galgame, userID, token, isOwner)
	if err != nil {
		return nil, err
	}
	byWork, err := s.repo.PatchIDsByWorkIDs(workIDs)
	if err != nil {
		return nil, err
	}
	ids := make([]int, 0, len(workIDs))
	for _, w := range workIDs {
		if pid, ok := byWork[w]; ok {
			ids = append(ids, pid)
		}
	}
	return ids, nil
}

func (s *UserService) GetUserComments(ctx context.Context, userID, page, limit int) ([]patchModel.PatchComment, int64, error) {
	cs, total, err := s.repo.GetUserComments(userID, (page-1)*limit, limit)
	if err != nil {
		return cs, total, err
	}
	s.attachCommentUsers(ctx, cs)
	s.attachPatchSummaries(ctx, cs, nil)
	return cs, total, nil
}

func (s *UserService) GetUserContributions(userID, page, limit int, includeEmpty bool, contentLimit string) ([]patchModel.Patch, int64, error) {
	return s.repo.GetUserContributions(userID, (page-1)*limit, limit, includeEmpty, contentLimit)
}

func (s *UserService) attachResourceUsers(ctx context.Context, rs []patchModel.PatchResource) {
	uids := make([]int, 0, len(rs))
	for _, r := range rs {
		uids = append(uids, r.UserID)
	}
	briefs := userclient.BriefMapByInt(ctx, s.users, uids)
	for i := range rs {
		if b := briefs[rs[i].UserID]; b != nil {
			rs[i].User = &patchModel.PatchUser{ID: int(b.ID), Name: b.Name, Avatar: b.Avatar, AvatarImageHash: b.AvatarImageHash, Roles: b.Roles, SiteRoles: b.SiteRoles}
		}
	}
}

func (s *UserService) attachCommentUsers(ctx context.Context, cs []patchModel.PatchComment) {
	uids := make([]int, 0, len(cs))
	for _, c := range cs {
		uids = append(uids, c.UserID)
	}
	briefs := userclient.BriefMapByInt(ctx, s.users, uids)
	for i := range cs {
		if b := briefs[cs[i].UserID]; b != nil {
			cs[i].User = &patchModel.PatchUser{ID: int(b.ID), Name: b.Name, Avatar: b.Avatar, AvatarImageHash: b.AvatarImageHash, Roles: b.Roles, SiteRoles: b.SiteRoles}
		}
	}
}
