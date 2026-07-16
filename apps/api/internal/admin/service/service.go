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
	"kun-galgame-patch-api/internal/middleware"
	patchModel "kun-galgame-patch-api/internal/patch/model"
	patchService "kun-galgame-patch-api/internal/patch/service"
	settingService "kun-galgame-patch-api/internal/setting/service"
	"kun-galgame-patch-api/pkg/errors"

	"github.com/redis/go-redis/v9"
)

type AdminService struct {
	repo    *repository.AdminRepository
	rdb     *redis.Client // sessions only (user-purge revocation); settings now live in `setting`
	setting *settingService.Service
	// patch is the canonical owner of resource/comment mutations (delete handles
	// moemoepoint reconciliation + owner notification + count upkeep). The admin
	// panel delegates the ACTION to it and only layers the
	// admin_log audit on top — so a delete via the admin panel behaves exactly
	// like one via the public Option-B path (no divergent re-implementation).
	patch *patchService.PatchService
}

func New(repo *repository.AdminRepository, rdb *redis.Client, setting *settingService.Service, patch *patchService.PatchService) *AdminService {
	return &AdminService{repo: repo, rdb: rdb, setting: setting, patch: patch}
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
	// Pure delegate to the canonical privileged delete — it decrements
	// comment_count, notifies the author, AND writes the audit_log (via the
	// AuditLogger injected into patch-service). Deleting from the admin panel is
	// now identical to the game-detail page; no reason is captured here.
	return s.patch.DeleteComment(commentID, adminUID, true, "")
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

func (s *AdminService) DeleteResource(resourceID, adminUID int, reason string) error {
	// Pure delegate to the canonical privileged delete — it handles artifact +
	// history cleanup, the owner's -3 moemoepoint reversal, the owner
	// notification (carrying reason), AND the audit_log (via the AuditLogger).
	// The admin panel now collects a reason too, mirroring the game-detail flow.
	return s.patch.DeleteResource(resourceID, adminUID, true, reason)
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
		MiscTraces:          c.MiscTraces,
		CanDeleteUserRow:    c.OwnedPatches == 0 || includeOwnedPatches,
	}, nil
}

// PurgeUser wipes every moyu-side trace of the user (DB rows via CASCADE +
// explicit FK-less tables) and revokes active sessions. Legacy s3_key objects
// are left in the frozen backup bucket, not reclaimed. Returns a 400 AppError
// when the user owns patches but the force flag is off.
func (s *AdminService) PurgeUser(userID int, purgeOwnedPatches bool, adminUID int) (*dto.UserPurgeResult, error) {
	// Snapshot the user's live artifact_uuids BEFORE the CASCADE removes the rows.
	// Completed artifact blobs aren't auto-reclaimed, so they must be soft-deleted
	// or they leak; enumeration failure isn't fatal (a straggler can be swept
	// out-of-band). Legacy s3_key objects stay in the frozen backup bucket.
	uuids, uErr := s.repo.CollectUserArtifactUUIDs(userID, purgeOwnedPatches)
	if uErr != nil {
		slog.Warn("PurgeUser: failed to enumerate artifact_uuids for cleanup", "user_id", userID, "error", uErr)
		uuids = nil
	}

	if err := s.repo.PurgeUser(userID, purgeOwnedPatches); err != nil {
		if stderrors.Is(err, repository.ErrUserOwnsPatches) {
			return nil, errors.ErrBadRequest("该用户仍拥有补丁，必须勾选「强删该用户创建的补丁」才能删除其账号")
		}
		return nil, errors.ErrInternal("")
	}

	// Best-effort artifact soft-delete AFTER the DB purge (delegated to patch,
	// which owns the artifact client).
	if len(uuids) > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		s.patch.SoftDeleteArtifacts(ctx, uuids)
	}

	res := &dto.UserPurgeResult{UserID: userID, UserRowDeleted: true}

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
		"sessions_revoked":    res.SessionsRevoked,
	})
	return res, nil
}

// ===== All Patches =====

func (s *AdminService) GetAllPatches(search string, page, limit int) ([]patchModel.Patch, int64, error) {
	return s.repo.GetAllPatches(search, (page-1)*limit, limit)
}

// LookupPatchesByIDs makes AdminService satisfy enricher.PatchSummaryDB, so the
// handler can attach galgame name/banner summaries to comment/resource lists.
func (s *AdminService) LookupPatchesByIDs(ids []int) ([]patchModel.Patch, error) {
	return s.repo.LookupPatchesByIDs(ids)
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

// GetOrphanCandidateIDs returns ids of candidate orphans (cheap local filter);
// the handler verifies them against Wiki by id.
func (s *AdminService) GetOrphanCandidateIDs() ([]int, error) {
	return s.repo.GetOrphanCandidateIDs()
}

// GetOrphanPatches returns the list of true orphan patches. excludeIDs are
// candidates Wiki confirmed exist by id (not real orphans).
func (s *AdminService) GetOrphanPatches(page, limit int, excludeIDs []int) ([]patchModel.Patch, int64, error) {
	return s.repo.GetOrphanPatches((page-1)*limit, limit, excludeIDs)
}

// CountOrphanPatches returns true-orphan counts by category (excludeIDs excluded).
func (s *AdminService) CountOrphanPatches(excludeIDs []int) (pending, badVndb int64, err error) {
	return s.repo.CountOrphanPatches(excludeIDs)
}
