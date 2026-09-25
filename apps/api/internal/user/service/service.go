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
	"kun-galgame-patch-api/pkg/communityclient"
	"kun-galgame-patch-api/pkg/moemoepoint"
	"kun-galgame-patch-api/pkg/userclient"

	"gorm.io/gorm"
)

type UserService struct {
	repo        *repository.UserRepository
	users       *userclient.Client
	galgame     *galgameClient.Client
	favorites   *favorite.Service
	db          *gorm.DB
	mp          *moemoepoint.Awarder
	comments    CommentStats
	community   *communityclient.Client
	presence    userPresence
	briefLookup briefLookup
}

// CommentStats is what the profile still needs to know about comments now that
// they live in the community primitive: how many a user has written, and which
// game page a moemoepoint `comment:<id>` reference points at.
type CommentStats interface {
	AuthorCounts(ctx context.Context, userIDs []int) map[int]int64
	PatchIDsForRefs(ctx context.Context, ids []int) map[int]int
}

func New(
	repo *repository.UserRepository,
	users *userclient.Client,
	galgame *galgameClient.Client,
	favorites *favorite.Service,
	db *gorm.DB,
	mp *moemoepoint.Awarder,
	comments CommentStats,
	community *communityclient.Client,
) *UserService {
	return &UserService{
		repo: repo, users: users, galgame: galgame, favorites: favorites, db: db, mp: mp, comments: comments,
		community: community, presence: repo, briefLookup: oauthBriefs{users},
	}
}

func (s *UserService) commentCount(ctx context.Context, userID int) int64 {
	if s.comments == nil {
		return 0
	}
	return s.comments.AuthorCounts(ctx, []int{userID})[userID]
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

func (s *UserService) attachPatchSummaries(ctx context.Context, resources []patchModel.PatchResource) {
	if len(resources) == 0 {
		return
	}
	idSet := make(map[int]struct{}, len(resources))
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
	for i := range resources {
		if sum, ok := summaries[resources[i].GalgameID]; ok {
			cp := sum
			resources[i].Patch = &cp
		}
	}
}

func (s *UserService) cardInfo(ctx context.Context, userID int) (*dto.UserInfoResponse, error) {
	user, err := s.repo.FindByID(userID)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}

	resp := &dto.UserInfoResponse{
		ID:            user.ID,
		Moemoepoint:   user.Moemoepoint,
		RegisterTime:  user.Created.Format(time.RFC3339),
		PatchCount:    s.repo.CountUserPatches(userID),
		ResourceCount: s.repo.CountUserResources(userID),
	}

	if b := userclient.BriefMapByInt(ctx, s.users, []int{userID})[userID]; b != nil {
		resp.Name = b.Name
		resp.Avatar = b.Avatar
		resp.Cosmetics = b.Cosmetics
		resp.Bio = b.Bio
		resp.Roles = b.Roles
		resp.SiteRoles = b.SiteRoles
	}

	return resp, nil
}

func (s *UserService) GetUserInfo(ctx context.Context, userID, currentUID int, token, contentLimit string) (*dto.UserInfoResponse, error) {
	resp, err := s.cardInfo(ctx, userID)
	if err != nil {
		return nil, err
	}

	resp.CommentCount = s.commentCount(ctx, userID)
	resp.FavoriteCount = s.countFavorites(ctx, userID, token, currentUID == userID, contentLimit)
	s.attachFollowState(ctx, resp, currentUID)

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

// GetUserFloating makes no catalog or community call because the game page
// calls it on every view, so follow counts are omitted.
func (s *UserService) GetUserFloating(ctx context.Context, userID int) (*dto.UserInfoResponse, error) {
	return s.cardInfo(ctx, userID)
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
	s.attachMoemoepointLinks(ctx, items)
	return items, hasMore, nil
}

func (s *UserService) attachMoemoepointLinks(ctx context.Context, items []moemoepoint.LogEntry) {
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
	if len(commentIDs) > 0 && s.comments != nil {
		galgameByComment = s.comments.PatchIDsForRefs(ctx, commentIDs)
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
			items[i].Link = fmt.Sprintf("/galgame/%d", id)
		case "comment":
			if gid := galgameByComment[id]; gid > 0 {
				items[i].Link = fmt.Sprintf("/galgame/%d?tab=comment#comment-%d", gid, id)
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
	s.attachPatchSummaries(ctx, rs)
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
	workIDs, err := s.favorites.WorkIDs(ctx, userID, token, isOwner)
	if err != nil {
		return nil, err
	}
	candidates := make([]int, 0, len(workIDs))
	for _, w := range workIDs {
		if w > 0 {
			candidates = append(candidates, int(w))
		}
	}
	here, err := s.repo.ExistingPatchIDs(candidates)
	if err != nil {
		return nil, err
	}
	ids := make([]int, 0, len(candidates))
	for _, pid := range candidates {
		if here[pid] {
			ids = append(ids, pid)
		}
	}
	return ids, nil
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
			rs[i].User = patchModel.NewPatchUser(b)
		}
	}
}
