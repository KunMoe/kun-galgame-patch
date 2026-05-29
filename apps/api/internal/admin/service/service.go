package service

import (
	"context"
	stderrors "errors"
	"log/slog"
	"time"

	"kun-galgame-patch-api/internal/admin/dto"
	adminModel "kun-galgame-patch-api/internal/admin/model"
	"kun-galgame-patch-api/internal/admin/repository"
	"kun-galgame-patch-api/internal/infrastructure/markdown"
	"kun-galgame-patch-api/internal/infrastructure/storage"
	"kun-galgame-patch-api/internal/middleware"
	patchModel "kun-galgame-patch-api/internal/patch/model"
	settingService "kun-galgame-patch-api/internal/setting/service"
	"kun-galgame-patch-api/pkg/errors"

	"github.com/redis/go-redis/v9"
)

type AdminService struct {
	repo    *repository.AdminRepository
	rdb     *redis.Client // sessions only (user-purge revocation); settings now live in `setting`
	setting *settingService.Service
	s3      *storage.S3Client
}

func New(repo *repository.AdminRepository, rdb *redis.Client, setting *settingService.Service, s3 *storage.S3Client) *AdminService {
	return &AdminService{repo: repo, rdb: rdb, setting: setting, s3: s3}
}

// ===== Comments =====

func (s *AdminService) GetComments(search, status string, page, limit int) ([]patchModel.PatchComment, int64, error) {
	comments, total, err := s.repo.GetComments(search, status, (page-1)*limit, limit)
	if err == nil {
		for i := range comments {
			comments[i].ContentHTML = markdown.MustRender(comments[i].Content)
		}
	}
	return comments, total, err
}

func (s *AdminService) UpdateComment(commentID int, content string, adminUID int) error {
	if err := s.repo.UpdateComment(commentID, content); err != nil {
		return err
	}
	s.repo.CreateLog(adminUID, "updateComment", map[string]any{"comment_id": commentID})
	return nil
}

func (s *AdminService) DeleteComment(commentID, adminUID int) error {
	if err := s.repo.DeleteComment(commentID); err != nil {
		return err
	}
	s.repo.CreateLog(adminUID, "deleteComment", map[string]any{"comment_id": commentID})
	return nil
}

// ===== Resources =====

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

func (s *AdminService) DeleteResource(resourceID, adminUID int) error {
	// Snapshot Storage + S3Key AND the file_history's old_s3_keys BEFORE
	// deleting the row so we can clean up the S3 objects afterwards. Read
	// failures aren't fatal — fall through and let the DB delete still
	// happen (matches the patch-side DeleteResource: deleting the row is
	// the primary operation, S3 cleanup is best-effort).
	resource, readErr := s.repo.GetResourceByID(resourceID)
	historyKeys, hErr := s.repo.GetResourceFileHistoryS3Keys(resourceID)
	if hErr != nil {
		slog.Warn("admin DeleteResource: failed to enumerate history old_s3_keys for cleanup", "resource_id", resourceID, "error", hErr)
		historyKeys = nil
	}

	if err := s.repo.DeleteResource(resourceID); err != nil {
		return err
	}

	// Drain both sources. The two key sets are disjoint by construction —
	// history rows only record keys that have already been replaced by a
	// newer one, so they never overlap with the live resource.S3Key.
	if s.s3 != nil && s.s3.Ready() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if readErr == nil && resource.Storage == "s3" && resource.S3Key != "" {
			if err := s.s3.DeleteObject(ctx, resource.S3Key); err != nil {
				slog.Warn("admin DeleteResource: 删除 S3 对象失败", "s3_key", resource.S3Key, "resource_id", resourceID, "error", err)
			}
		}
		for _, key := range historyKeys {
			if err := s.s3.DeleteObject(ctx, key); err != nil {
				slog.Warn("admin DeleteResource: 删除 S3 历史对象失败", "s3_key", key, "resource_id", resourceID, "error", err)
			}
		}
	}

	s.repo.CreateLog(adminUID, "deleteResource", map[string]any{"resource_id": resourceID})
	return nil
}

// User management (GetUsers / UpdateUser / DeleteUser) was removed when
// identity moved to OAuth: name / email / role / status / bans are all owned
// by the OAuth admin console, not this site.
//
// Creator-application flow was removed alongside the creator role itself
// (decision: creator role = 2 was deleted in the OAuth migration).

// ===== User purge (anti-spam) =====

// PurgeUserPreview returns the dry-run count breakdown. includeOwnedPatches
// mirrors the execute-time force flag so the collateral counts shown match
// what an execute would remove.
func (s *AdminService) PurgeUserPreview(userID int, includeOwnedPatches bool) (*dto.UserPurgePreview, error) {
	c, err := s.repo.PurgePreview(userID, includeOwnedPatches)
	if err != nil {
		return nil, err
	}
	return &dto.UserPurgePreview{
		UserID:              userID,
		UserExists:          c.UserExists,
		Comments:            c.Comments,
		Resources:           c.Resources,
		ResourceS3Files:     c.ResourceS3Files,
		CommentLikes:        c.CommentLikes,
		ResourceLikes:       c.ResourceLikes,
		Favorites:           c.Favorites,
		Contributes:         c.Contributes,
		Following:           c.Following,
		Followers:           c.Followers,
		ChatMemberships:     c.ChatMemberships,
		ChatMessages:        c.ChatMessages,
		PrivateMessages:     c.PrivateMessages,
		OwnedPatches:        c.OwnedPatches,
		OwnedPatchResources: c.OwnedPatchResources,
		OwnedPatchComments:  c.OwnedPatchComments,
		OwnedPatchS3Files:   c.OwnedPatchS3Files,
		MiscTraces:          c.MiscTraces,
		CanDeleteUserRow:    c.OwnedPatches == 0 || includeOwnedPatches,
	}, nil
}

// PurgeUser wipes every moyu-side trace of the user, then best-effort deletes
// the orphaned S3 objects (same philosophy as DeleteResource / DeletePatch: the
// DB transaction is the primary op; S3 cleanup only WARNs on failure). Returns
// a 400 AppError when the user owns patches but the force flag is off.
func (s *AdminService) PurgeUser(userID int, purgeOwnedPatches bool, adminUID int) (*dto.UserPurgeResult, error) {
	// Snapshot S3 keys BEFORE the DB delete — the rows pointing at them are
	// about to vanish. Enumeration failure isn't fatal (a periodic offline
	// scrub can sweep stragglers); fall through and still purge the DB.
	keys, kerr := s.repo.CollectUserS3Keys(userID, purgeOwnedPatches)
	if kerr != nil {
		slog.Warn("PurgeUser: failed to enumerate s3 keys for cleanup", "user_id", userID, "error", kerr)
		keys = nil
	}

	if err := s.repo.PurgeUser(userID, purgeOwnedPatches); err != nil {
		if stderrors.Is(err, repository.ErrUserOwnsPatches) {
			return nil, errors.ErrBadRequest("该用户仍拥有补丁，必须勾选「强删该用户创建的补丁」才能删除其账号")
		}
		return nil, errors.ErrInternal("")
	}

	res := &dto.UserPurgeResult{UserID: userID, UserRowDeleted: true}
	if s.s3 != nil && s.s3.Ready() && len(keys) > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		for _, k := range keys {
			if err := s.s3.DeleteObject(ctx, k); err != nil {
				slog.Warn("PurgeUser: 删除 S3 对象失败", "s3_key", k, "user_id", userID, "error", err)
				res.S3Failed++
			} else {
				res.S3Deleted++
			}
		}
	}

	// Revoke active Redis sessions so the purged user's existing cookie can't
	// keep authenticating (the request path reads identity from the session
	// blob, not the now-deleted DB row). Best-effort: a SCAN failure only WARNs.
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
		"s3_deleted":          res.S3Deleted,
		"s3_failed":           res.S3Failed,
		"sessions_revoked":    res.SessionsRevoked,
	})
	return res, nil
}

// ===== All Patches =====

func (s *AdminService) GetAllPatches(search string, page, limit int) ([]patchModel.Patch, int64, error) {
	return s.repo.GetAllPatches(search, (page-1)*limit, limit)
}

// ===== Settings =====
//
// Source of truth is the site_setting table via settingService (durable +
// audited), NOT Redis. SetSetting records the acting admin for the audit trail.

func (s *AdminService) GetSetting(key string) bool {
	return s.setting.GetBool(key)
}

func (s *AdminService) SetSetting(key string, enabled bool, adminUID int) error {
	return s.setting.SetBool(key, enabled, adminUID)
}

// ===== Stats =====

func (s *AdminService) GetStats(days int) *dto.AdminStatsResponse {
	since := time.Now().AddDate(0, 0, -days)
	newUser, newActive, newGalgame, newResource, newComment := s.repo.GetStats(since)
	return &dto.AdminStatsResponse{
		NewUser:          newUser,
		NewActiveUser:    newActive,
		NewGalgame:       newGalgame,
		NewPatchResource: newResource,
		NewComment:       newComment,
	}
}

func (s *AdminService) GetStatsSum() *dto.AdminStatsSumResponse {
	u, g, r, c := s.repo.GetStatsSum()
	return &dto.AdminStatsSumResponse{
		UserCount:          u,
		GalgameCount:       g,
		PatchResourceCount: r,
		PatchCommentCount:  c,
	}
}

// ===== Logs =====

func (s *AdminService) GetLogs(page, limit int) ([]adminModel.AdminLog, int64, error) {
	return s.repo.GetLogs((page-1)*limit, limit)
}

// ===== Resource file history (MOYU-PR5 / M3) =====

func (s *AdminService) GetResourceFileHistory(
	resourceID, page, limit int,
) ([]patchModel.PatchResourceFileHistory, int64, error) {
	return s.repo.GetResourceFileHistory(resourceID, (page-1)*limit, limit)
}

// ===== Orphan Patches (D12) =====

// GetOrphanPatches returns the list of orphan patches with galgame_id=0.
func (s *AdminService) GetOrphanPatches(page, limit int) ([]patchModel.Patch, int64, error) {
	return s.repo.GetOrphanPatches((page-1)*limit, limit)
}

// CountOrphanPatches returns orphan patch counts by category.
func (s *AdminService) CountOrphanPatches() (pending, badVndb int64, err error) {
	return s.repo.CountOrphanPatches()
}
