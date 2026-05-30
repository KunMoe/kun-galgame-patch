package service

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	authModel "kun-galgame-patch-api/internal/auth/model"
	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/internal/galgame/enricher"
	"kun-galgame-patch-api/internal/infrastructure/storage"
	patchModel "kun-galgame-patch-api/internal/patch/model"
	"kun-galgame-patch-api/internal/user/dto"
	"kun-galgame-patch-api/internal/user/model"
	"kun-galgame-patch-api/internal/user/repository"
	"kun-galgame-patch-api/pkg/imageutil"
	"kun-galgame-patch-api/pkg/moemoepoint"
	"kun-galgame-patch-api/pkg/userclient"

	"gorm.io/gorm"
)

// Daily personal image upload limit, aligned with KUN_PATCH_USER_DAILY_UPLOAD_IMAGE_LIMIT in apps/next-web/config/user.ts.
const DailyImageLimit = 20

type UserService struct {
	repo  *repository.UserRepository
	s3    *storage.S3Client
	users *userclient.Client
	wiki  *galgameClient.Client
	db    *gorm.DB
	mp    *moemoepoint.Awarder
}

func New(repo *repository.UserRepository, s3 *storage.S3Client, users *userclient.Client, wiki *galgameClient.Client, db *gorm.DB, mp *moemoepoint.Awarder) *UserService {
	return &UserService{repo: repo, s3: s3, users: users, wiki: wiki, db: db, mp: mp}
}

// patchSummaryFinder adapts *gorm.DB to enricher.patchSummaryDB so we can
// reuse the same Wiki-batch summary builder the global handlers use.
type patchSummaryFinder struct{ db *gorm.DB }

func (p patchSummaryFinder) LookupPatchesByIDs(ids []int) ([]patchModel.Patch, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var rows []patchModel.Patch
	err := p.db.Select("id", "vndb_id").Where("id IN ?", ids).Find(&rows).Error
	return rows, err
}

// attachPatchSummaries stamps the `Patch` field on each resource and each
// comment in one Wiki batch call (name + banner come from the Wiki Service).
// Either slice may be nil.
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

	summaries := enricher.BuildPatchSummaryMap(ctx, s.wiki, patchSummaryFinder{db: s.db}, ids)
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

// GetUserInfo composes the public user profile: site-local row (moemoepoint,
// follower/following counts, content counts) + OAuth brief (name/avatar/bio).
//
// On OAuth lookup failure name/avatar/bio come back empty -- the page still
// renders, just without display fields.
func (s *UserService) GetUserInfo(ctx context.Context, userID, currentUID int) (*dto.UserInfoResponse, error) {
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
		FavoriteCount:  s.repo.CountUserFavorites(userID),
	}

	if s.users != nil {
		if b, _ := s.users.User(ctx, uint(userID)); b != nil {
			resp.Name = b.Name
			resp.Avatar = b.Avatar
			resp.Bio = b.Bio
			resp.Roles = b.Roles
		}
	}

	if currentUID > 0 && currentUID != userID {
		_, err := s.repo.FindFollow(currentUID, userID)
		resp.IsFollowed = err == nil
	}

	return resp, nil
}

// GetUserFloating retrieves the floating-card view of a user.
func (s *UserService) GetUserFloating(ctx context.Context, userID int) (*dto.UserInfoResponse, error) {
	return s.GetUserInfo(ctx, userID, 0)
}

// Follow creates a follow relation and bumps the denormalized counts.
func (s *UserService) Follow(followerID, followingID int) error {
	if followerID == followingID {
		return fmt.Errorf("cannot follow yourself")
	}

	_, err := s.repo.FindFollow(followerID, followingID)
	if err == nil {
		return fmt.Errorf("already following this user")
	}

	// Relation insert + count bump commit atomically (audit F029).
	if err := s.repo.CreateFollowAndIncrement(followerID, followingID); err != nil {
		// The followee id comes from OAuth and may legitimately lack a local
		// `user` row; the following_id FK then rejects the insert. Map that to
		// a clean message instead of leaking the raw Postgres SQLSTATE string.
		if strings.Contains(err.Error(), "violates foreign key") || strings.Contains(err.Error(), "23503") {
			return fmt.Errorf("用户不存在")
		}
		return err
	}
	return nil
}

// Unfollow removes a follow relation and decrements counts ONLY when a
// relation actually existed. Without the rows-affected guard, calling unfollow
// on a user you never followed would still decrement their follower_count,
// letting anyone corrupt/harass another user's count (GREATEST clamps at 0 but
// can't prevent corrupting a legitimate positive count).
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

// GetFollowers returns follower user briefs, batch-resolved from OAuth.
// `viewerID` (0 for anonymous) is used to stamp each row with the viewer-
// relative is_followed flag so the FE can render per-row follow buttons
// without per-row round-trips.
func (s *UserService) GetFollowers(ctx context.Context, userID, viewerID, page, limit int) ([]model.UserFollowItem, int64, error) {
	ids, total, err := s.repo.GetFollowerIDs(userID, (page-1)*limit, limit)
	if err != nil {
		return nil, 0, err
	}
	return s.briefsToFollowItems(ctx, ids, viewerID), total, nil
}

// GetFollowing returns followee user briefs, batch-resolved from OAuth.
// See GetFollowers for `viewerID` semantics.
func (s *UserService) GetFollowing(ctx context.Context, userID, viewerID, page, limit int) ([]model.UserFollowItem, int64, error) {
	ids, total, err := s.repo.GetFollowingIDs(userID, (page-1)*limit, limit)
	if err != nil {
		return nil, 0, err
	}
	return s.briefsToFollowItems(ctx, ids, viewerID), total, nil
}

// briefsToFollowItems is briefsToUserBasic + per-row is_followed stamp.
// One follow-set lookup covers the whole page.
func (s *UserService) briefsToFollowItems(ctx context.Context, ids []int, viewerID int) []model.UserFollowItem {
	briefs := userclient.BriefMapByInt(ctx, s.users, ids)
	followed, _ := s.repo.WhichFollowed(viewerID, ids) // nil on error → empty map → all false
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

// SearchUsers proxies OAuth /users/search and returns the slim wire shape.
//
// limit is capped server-side at 50 (the OAuth API's max).
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
		out = append(out, model.UserBasic{ID: int(b.ID), Name: b.Name, Avatar: b.Avatar})
	}
	return out, nil
}

// briefsToUserBasic batches an id list through OAuth and returns the wire
// shape. IDs missing on OAuth are silently dropped from the result.
func (s *UserService) briefsToUserBasic(ctx context.Context, ids []int) []model.UserBasic {
	briefs := userclient.BriefMapByInt(ctx, s.users, ids)
	out := make([]model.UserBasic, 0, len(ids))
	for _, id := range ids {
		if b := briefs[id]; b != nil {
			out = append(out, model.UserBasic{ID: int(b.ID), Name: b.Name, Avatar: b.Avatar})
		}
	}
	return out
}

// CheckIn performs daily check-in.
func (s *UserService) CheckIn(userID int) (int, error) {
	// Atomic check-and-set (audit GPT-M04): only the request that actually
	// flips daily_check_in 0→1 proceeds. The previous read-then-write let two
	// concurrent requests both pass the "already checked in" guard and both
	// return a success + random reward.
	affected, err := s.repo.CheckIn(userID)
	if err != nil {
		return 0, err
	}
	if affected == 0 {
		return 0, fmt.Errorf("already checked in today")
	}

	points := rand.Intn(8) // 0-7
	// Award via OAuth (unified balance). Best-effort; the daily flag is already
	// set so a missed award doesn't let the user re-check. Key is per-user-per-
	// day, with the date pinned to Asia/Shanghai so it agrees with the daily-
	// reset cron's "day" boundary (audit F085) → replay-safe. points==0 is a
	// no-op (Awarder skips a zero delta, satisfying OAuth's non-zero rule).
	loc, lerr := time.LoadLocation("Asia/Shanghai")
	if lerr != nil || loc == nil {
		loc = time.Local
	}
	date := time.Now().In(loc).Format("2006-01-02")
	go s.mp.Award(context.Background(), userID, points, "daily_checkin", "",
		fmt.Sprintf("moyu:checkin:%d:%s", userID, date))
	return points, nil
}

// GetMoemoepointLog reads a page of the user's OWN moemoepoint ledger from OAuth
// (the unified source of truth — moyu stores no local ledger). Cursor paginated
// via beforeID (0 = newest page); reason is an optional filter.
func (s *UserService) GetMoemoepointLog(ctx context.Context, userID, limit int, beforeID int64, reason string) ([]moemoepoint.LogEntry, bool, error) {
	return s.mp.Log(ctx, userID, limit, beforeID, reason)
}

// GetUserPatches retrieves the user's patch list.
func (s *UserService) GetUserPatches(userID, page, limit int) ([]patchModel.Patch, int64, error) {
	return s.repo.GetUserPatches(userID, (page-1)*limit, limit)
}

// GetUserResources retrieves the user's resource list with each resource
// enriched with its owning patch's Wiki summary (name + banner) so the
// /user/:id/resource page can render the game thumbnail + title without an
// extra round-trip per row.
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

// GetUserFavorites retrieves the user's favorite list.
func (s *UserService) GetUserFavorites(userID, page, limit int) ([]patchModel.Patch, int64, error) {
	return s.repo.GetUserFavorites(userID, (page-1)*limit, limit)
}

// GetUserComments retrieves the user's comment list with each comment
// enriched with its owning patch's Wiki summary (name only — banner is not
// needed for the "评论在 <game>" link on the user-comments page).
func (s *UserService) GetUserComments(ctx context.Context, userID, page, limit int) ([]patchModel.PatchComment, int64, error) {
	cs, total, err := s.repo.GetUserComments(userID, (page-1)*limit, limit)
	if err != nil {
		return cs, total, err
	}
	s.attachCommentUsers(ctx, cs)
	s.attachPatchSummaries(ctx, cs, nil)
	return cs, total, nil
}

// GetUserContributions retrieves the user's contribution list.
func (s *UserService) GetUserContributions(userID, page, limit int) ([]patchModel.Patch, int64, error) {
	return s.repo.GetUserContributions(userID, (page-1)*limit, limit)
}

// GetUserByID retrieves the local user row (site-local fields only).
func (s *UserService) GetUserByID(userID int) (*authModel.User, error) {
	return s.repo.FindByID(userID)
}

// attachResourceUsers / attachCommentUsers stamp the User field on rows
// returned to the user-profile pages.
func (s *UserService) attachResourceUsers(ctx context.Context, rs []patchModel.PatchResource) {
	uids := make([]int, 0, len(rs))
	for _, r := range rs {
		uids = append(uids, r.UserID)
	}
	briefs := userclient.BriefMapByInt(ctx, s.users, uids)
	for i := range rs {
		if b := briefs[rs[i].UserID]; b != nil {
			rs[i].User = &patchModel.PatchUser{ID: int(b.ID), Name: b.Name, Avatar: b.Avatar, AvatarImageHash: b.AvatarImageHash, Roles: b.Roles}
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
			cs[i].User = &patchModel.PatchUser{ID: int(b.ID), Name: b.Name, Avatar: b.Avatar, AvatarImageHash: b.AvatarImageHash, Roles: b.Roles}
		}
	}
}

// ─── User image uploads ──────────────────────────────

// UploadUserImage uploads an image for the user's personal page (fit within 1920x1080, JPEG q=50).
// Rate-limited by daily_image_count (aligned with the original project's DailyImageLimit).
func (s *UserService) UploadUserImage(ctx context.Context, userID int, raw []byte) (string, error) {
	user, err := s.repo.FindByID(userID)
	if err != nil {
		return "", fmt.Errorf("用户不存在")
	}
	if user.DailyImageCount >= DailyImageLimit {
		return "", fmt.Errorf("今日上传图片数量已达 %d 张上限", DailyImageLimit)
	}

	jpg, err := imageutil.FitJPEG(raw, 1920, 1080, 50)
	if err != nil {
		return "", err
	}

	key := fmt.Sprintf("user_%d/image/%d-%d.jpg", userID, userID, time.Now().UnixMilli())
	if err := s.s3.PutObject(ctx, key, bytes.NewReader(jpg), int64(len(jpg)), "image/jpeg"); err != nil {
		return "", err
	}

	if err := s.repo.UpdateFields(userID, map[string]any{
		"daily_image_count": gorm.Expr("daily_image_count + 1"),
	}); err != nil {
		return "", err
	}
	return s.s3.PublicURL(key), nil
}
