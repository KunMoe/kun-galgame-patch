package service

import (
	"context"
	"log/slog"
	"time"

	"kun-galgame-patch-api/internal/admin/dto"
	adminModel "kun-galgame-patch-api/internal/admin/model"
	"kun-galgame-patch-api/internal/admin/repository"
	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/internal/middleware"
	patchModel "kun-galgame-patch-api/internal/patch/model"
	patchService "kun-galgame-patch-api/internal/patch/service"
	settingService "kun-galgame-patch-api/internal/setting/service"
	"kun-galgame-patch-api/pkg/communityclient"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/upstream"

	"github.com/redis/go-redis/v9"
)

type AdminService struct {
	repo      *repository.AdminRepository
	rdb       *redis.Client
	setting   *settingService.Service
	patch     *patchService.PatchService
	galgame   *galgameClient.Client
	comments  CommentPurger
	community *communityclient.Client
}

// CommentPurger is the community primitive's side of erasing a user. Comments
// are not in this database any more, so a purge that only ran SQL would leave
// every comment the user wrote standing.
type CommentPurger interface {
	CountAuthorComments(ctx context.Context, userID int) int64
	PurgeAuthor(ctx context.Context, userID int) (int64, error)
}

func New(repo *repository.AdminRepository, rdb *redis.Client, setting *settingService.Service, patch *patchService.PatchService, galgame *galgameClient.Client, comments CommentPurger, community *communityclient.Client) *AdminService {
	return &AdminService{repo: repo, rdb: rdb, setting: setting, patch: patch, galgame: galgame, comments: comments, community: community}
}

func (s *AdminService) GetResources(search string, page, limit int) ([]patchModel.PatchResource, int64, error) {
	resources, total, err := s.repo.GetResources(search, (page-1)*limit, limit)
	if err == nil {
		patchModel.RenderResourceNotes(resources)
	}
	return resources, total, err
}

func (s *AdminService) UpdateResource(resourceID int, note string, adminUID int) error {
	if err := s.repo.UpdateResource(resourceID, note); err != nil {
		return err
	}
	s.repo.CreateLog(adminUID, "updateResource", map[string]any{"resource_id": resourceID})
	return nil
}

func (s *AdminService) DeleteResource(resourceID, adminUID int, reason string) error {
	return s.patch.DeleteResource(resourceID, adminUID, true, reason)
}

func (s *AdminService) PurgeUserPreview(ctx context.Context, userID int, includeOwnedPatches bool, token string) (*dto.UserPurgePreview, error) {
	c, err := s.repo.PurgePreview(userID, includeOwnedPatches)
	if err != nil {
		return nil, err
	}
	folders, items, folderErr := s.catalogFolders(ctx, userID, token)
	following, followers := s.followCounts(ctx, userID)
	return &dto.UserPurgePreview{
		UserID:              userID,
		UserExists:          c.UserExists,
		Comments:            s.comments.CountAuthorComments(ctx, userID),
		Resources:           c.Resources,
		ResourceLikes:       c.ResourceLikes,
		Contributes:         c.Contributes,
		Following:           following,
		Followers:           followers,
		ChatMemberships:     c.ChatMemberships,
		ChatMessages:        c.ChatMessages,
		PrivateMessages:     c.PrivateMessages,
		OwnedPatches:        c.OwnedPatches,
		OwnedPatchResources: c.OwnedPatchResources,
		MiscTraces:          c.MiscTraces,
		CatalogFolders:      folders,
		CatalogFolderItems:  items,
		CatalogFolderError:  folderErr,
	}, nil
}

// The folders are read and never deleted. The catalog has a face that would
// remove them all — DELETE /v2/moderation/users/{uid}/folders — and this purge
// deliberately does not call it: catalog_user_folder carries no site, so one
// shelf is the central account's and the forum imported 8283 of production's
// 11995 folders. Deleting the local account here promises kungal is untouched,
// and that call would take a forum user's whole collection with it. Removing a
// central account's folders belongs to whoever deletes the central account.
//
// Reading them needs the admin's own catalog moderation standing, which is a
// different grant from this site's admin role: a moyu admin who does not
// moderate in the catalog gets 403 and sees the reason rather than a zero.
func (s *AdminService) catalogFolders(ctx context.Context, userID int, token string) (int64, int64, string) {
	if s.galgame == nil || token == "" {
		return 0, 0, "未读取：当前会话没有 catalog 访问令牌"
	}
	folders, err := s.galgame.V2().UserFolders(ctx, token, int64(userID))
	if err != nil {
		if upstream.KindOf(err) == upstream.Rejected {
			return 0, 0, "未读取：当前管理员没有 catalog 审核权限"
		}
		slog.Warn("PurgeUserPreview: 读取 catalog 收藏夹失败", "user_id", userID, "error", err)
		return 0, 0, "读取 catalog 收藏夹失败"
	}
	var items int64
	for _, f := range folders {
		items += int64(f.ItemCount)
	}
	return int64(len(folders)), items, ""
}

func (s *AdminService) followCounts(ctx context.Context, userID int) (following, followers *int64) {
	if s.community == nil || !s.community.Configured() {
		return nil, nil
	}
	res, err := s.community.FollowStates(ctx, 0, []int64{int64(userID)})
	if err != nil {
		communityclient.LogDegraded(ctx, "purge preview follow counts unavailable", err, "user_id", userID)
		return nil, nil
	}
	for i := range res.States {
		if res.States[i].UserID == int64(userID) {
			g, f := res.States[i].FollowingCount, res.States[i].FollowersCount
			return &g, &f
		}
	}
	return nil, nil
}

func (s *AdminService) PurgeUser(ctx context.Context, userID int, purgeOwnedPatches bool, adminUID int) (*dto.UserPurgeResult, error) {
	// Every check that can refuse runs before the community purge. A refusal
	// used to run after it, so the admin was told 400 while every comment
	// the user wrote was already tombstoned upstream.
	if userID == adminUID {
		return nil, errors.ErrBadRequest("不能清除自己的账号")
	}

	uuids, uErr := s.repo.CollectUserArtifactUUIDs(userID, purgeOwnedPatches)
	if uErr != nil {
		slog.Warn("PurgeUser: failed to enumerate artifact_uuids for cleanup", "user_id", userID, "error", uErr)
		uuids = nil
	}

	// Comments first, and not best-effort: the local rows are about to go and
	// with them every trace of which posts were this user's, so a failure here
	// after the SQL purge would leave orphaned comments nothing can find again.
	commentsPurged, err := s.comments.PurgeAuthor(ctx, userID)
	if err != nil {
		return nil, err
	}

	res := &dto.UserPurgeResult{UserID: userID, CommentsPurged: commentsPurged}
	handedOver, err := s.repo.PurgeUser(userID, purgeOwnedPatches, adminUID)
	if err != nil {
		slog.Error("PurgeUser: local purge failed after the community purge", "user_id", userID, "error", err)
		res.Warning = "评论已清除，但本地数据清除失败，请重新执行清除"
		return res, nil
	}
	res.UserRowDeleted = true
	res.PatchesHandedOver = handedOver

	if len(uuids) > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		s.patch.SoftDeleteArtifacts(ctx, uuids)
	}

	if s.rdb != nil {
		if n, rerr := middleware.RevokeUserSessions(context.Background(), s.rdb, userID); rerr != nil {
			slog.Warn("PurgeUser: 撤销会话失败", "user_id", userID, "error", rerr)
		} else {
			res.SessionsRevoked = n
		}
	}

	s.repo.CreateLog(adminUID, "purgeUser", map[string]any{
		"target_user_id":      userID,
		"purge_owned_patches": purgeOwnedPatches,
		"sessions_revoked":    res.SessionsRevoked,
		"patches_handed_over": res.PatchesHandedOver,
	})
	return res, nil
}

func (s *AdminService) GetAllPatches(search string, page, limit int) ([]patchModel.Patch, int64, error) {
	return s.repo.GetAllPatches(search, (page-1)*limit, limit)
}

func (s *AdminService) LookupPatchesByIDs(ids []int) ([]patchModel.Patch, error) {
	return s.repo.LookupPatchesByIDs(ids)
}

func (s *AdminService) GetSetting(key string) bool {
	return s.setting.GetBool(key)
}

func (s *AdminService) SetSetting(key string, enabled bool, adminUID int) error {
	return s.setting.SetBool(key, enabled, adminUID)
}

func (s *AdminService) GetStats(days int) *dto.AdminStatsResponse {
	since := time.Now().AddDate(0, 0, -days)
	newUser, newActive, newGalgame, newResource := s.repo.GetStats(since)
	return &dto.AdminStatsResponse{
		NewUser:          newUser,
		NewActiveUser:    newActive,
		NewGalgame:       newGalgame,
		NewPatchResource: newResource,
	}
}

func (s *AdminService) GetStatsSum() *dto.AdminStatsSumResponse {
	u, g, r := s.repo.GetStatsSum()
	return &dto.AdminStatsSumResponse{
		UserCount:          u,
		GalgameCount:       g,
		PatchResourceCount: r,
	}
}

func (s *AdminService) GetLogs(page, limit int) ([]adminModel.AdminLog, int64, error) {
	return s.repo.GetLogs((page-1)*limit, limit)
}

func (s *AdminService) GetResourceFileHistory(
	resourceID, page, limit int,
) ([]patchModel.PatchResourceFileHistory, int64, error) {
	return s.repo.GetResourceFileHistory(resourceID, (page-1)*limit, limit)
}

func (s *AdminService) GetOrphanCandidateIDs() ([]int, error) {
	return s.repo.GetOrphanCandidateIDs()
}

func (s *AdminService) GetOrphanPatches(page, limit int, excludeIDs []int) ([]patchModel.Patch, int64, error) {
	return s.repo.GetOrphanPatches((page-1)*limit, limit, excludeIDs)
}

func (s *AdminService) CountOrphanPatches(excludeIDs []int) (pending, badVndb int64, err error) {
	return s.repo.CountOrphanPatches(excludeIDs)
}
