package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/internal/infrastructure/markdown"
	"kun-galgame-patch-api/internal/infrastructure/storage"
	"kun-galgame-patch-api/internal/patch/model"
	"kun-galgame-patch-api/internal/patch/repository"
	"kun-galgame-patch-api/pkg/userclient"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// ErrWikiGalgameMissing is returned by CreatePatch when the supplied
// vndb_id has no corresponding row on the Galgame Wiki yet. The handler
// translates this into the typed AppError so the frontend can pick it up
// via code = 44001 and render a "前往 Wiki 创建" CTA.
var ErrWikiGalgameMissing = errors.New("wiki galgame missing for vndb_id")

type PatchService struct {
	repo  *repository.PatchRepository
	rdb   *redis.Client
	db    *gorm.DB
	s3    *storage.S3Client
	wiki  *galgameClient.Client
	users *userclient.Client
}

func New(repo *repository.PatchRepository, rdb *redis.Client, db *gorm.DB, s3 *storage.S3Client, wiki *galgameClient.Client, users *userclient.Client) *PatchService {
	return &PatchService{repo: repo, rdb: rdb, db: db, s3: s3, wiki: wiki, users: users}
}

// ===== Patch =====

// CreatePatch handles POST /api/patch (D12, 2026-04-21).
//
// Strict policy: vndb_id MUST already exist on the Galgame Wiki. We do not
// POST /galgame on behalf of the user -- galgame metadata curation is
// pushed to the Wiki frontend (which has the search-and-pick UI for
// tag/official/engine that we don't want to re-implement here).
//
// When Wiki returns "not found" we surface ErrWikiGalgameMissing so the
// handler can map to AppError 44001 and the frontend renders a "前往 Wiki
// 创建" CTA with the vndb_id pre-filled.
//
// Steps:
//  1. Wiki /galgame/check?vndb_id=... -> exists + galgame_id (or 44001)
//  2. Local dedup on vndb_id
//  3. One transaction: insert patch with id=galgame_id, +3 moemoepoint,
//     register contributor.
func (s *PatchService) CreatePatch(ctx context.Context, uid int, vndbID string) (int, error) {
	// 1. Check with Wiki: must exist, and get galgame_id
	exists, galgameID, err := s.wiki.CheckGalgameByVndbID(ctx, vndbID)
	if err != nil {
		return 0, fmt.Errorf("调用 Wiki 校验 vndb_id 失败: %w", err)
	}
	if !exists {
		// Sentinel error so the handler can map this to 44001 (typed AppError).
		return 0, ErrWikiGalgameMissing
	}

	// 2. Local dedup
	if existing, _ := s.repo.FindPatchByVndbID(vndbID); existing != nil && existing.ID != 0 {
		return 0, fmt.Errorf("该 VNDB ID 已经存在对应的补丁")
	}

	// 3. Transaction
	//
	// D13: patch.id IS the Wiki galgame_id. We assign it explicitly here
	// rather than relying on the autoincrement sequence. If a row with
	// id=galgameID already exists (race / re-publish), the unique vndb_id
	// constraint check above would normally have caught it; the INSERT will
	// fail with a FK / pkey violation as a safety net.
	var patchID int
	txErr := s.db.Transaction(func(tx *gorm.DB) error {
		p := &model.Patch{
			ID:     galgameID,
			VndbID: vndbID,
			UserID: uid,
		}
		if err := tx.Create(p).Error; err != nil {
			return fmt.Errorf("创建 patch 失败: %w", err)
		}
		patchID = p.ID

		// User reward: +3 moemoepoint
		if err := tx.Table("user").Where("id = ?", uid).
			UpdateColumn("moemoepoint", gorm.Expr("moemoepoint + 3")).Error; err != nil {
			return fmt.Errorf("更新用户积分失败: %w", err)
		}

		// Register contributor
		if err := tx.Create(&model.UserPatchContributeRelation{
			UserID: uid, GalgameID: p.ID,
		}).Error; err != nil {
			return fmt.Errorf("登记 contributor 失败: %w", err)
		}
		if err := tx.Model(&model.Patch{}).Where("id = ?", p.ID).
			UpdateColumn("contribute_count", gorm.Expr("contribute_count + 1")).Error; err != nil {
			return fmt.Errorf("更新 contribute_count 失败: %w", err)
		}
		return nil
	})
	if txErr != nil {
		return 0, txErr
	}
	return patchID, nil
}

func (s *PatchService) GetPatch(id int) (*model.Patch, error) {
	return s.repo.GetPatchByID(id)
}

func (s *PatchService) GetPatchDetail(id int) (*model.Patch, error) {
	return s.repo.GetPatchDetail(id)
}

// RegisterClaimedGalgame creates the local patch row for a galgame the user
// just claimed on Wiki (status 2 → 0), awarding +3 moemoepoint and
// registering the contributor — all in one transaction.
//
// Per docs/galgame_wiki/00-handbook-for-downstream.md §9 the local
// side-effects for "Claim" are exactly: INSERT patch(zeros) + moemoepoint+=3.
// We deliberately do NOT call Wiki /galgame/check here (the caller just
// claimed it, so it exists and is published).
//
// Idempotent: if the patch row already exists (the galgame was interacted
// with before, or a double-submit), we return its id without re-rewarding.
// This is the single source of the claim reward — the handler must NOT also
// call a separate reward path (that was the prior double-+3 bug).
func (s *PatchService) RegisterClaimedGalgame(uid, galgameID int, vndbID string) (int, error) {
	if galgameID <= 0 {
		return 0, fmt.Errorf("invalid galgame id")
	}
	if existing, _ := s.repo.GetPatchByID(galgameID); existing != nil && existing.ID != 0 {
		// Already registered — no reward, just hand back the id so the
		// frontend can navigate. (Covers re-claim races / retries.)
		return existing.ID, nil
	}

	var patchID int
	txErr := s.db.Transaction(func(tx *gorm.DB) error {
		p := &model.Patch{ID: galgameID, VndbID: vndbID, UserID: uid}
		if err := tx.Create(p).Error; err != nil {
			return fmt.Errorf("创建 patch 失败: %w", err)
		}
		patchID = p.ID

		if err := tx.Table("user").Where("id = ?", uid).
			UpdateColumn("moemoepoint", gorm.Expr("moemoepoint + 3")).Error; err != nil {
			return fmt.Errorf("更新用户积分失败: %w", err)
		}

		if err := tx.Create(&model.UserPatchContributeRelation{
			UserID: uid, GalgameID: p.ID,
		}).Error; err != nil {
			return fmt.Errorf("登记 contributor 失败: %w", err)
		}
		if err := tx.Model(&model.Patch{}).Where("id = ?", p.ID).
			UpdateColumn("contribute_count", gorm.Expr("contribute_count + 1")).Error; err != nil {
			return fmt.Errorf("更新 contribute_count 失败: %w", err)
		}
		return nil
	})
	if txErr != nil {
		return 0, txErr
	}
	return patchID, nil
}

// DB exposes the underlying *gorm.DB so a few thin "no-business-logic" handler
// endpoints (the wiki messages read-state shims) can do single-table reads /
// upserts without round-tripping through a dedicated repo + service layer.
// Anything with real business logic should still live in a service method.
func (s *PatchService) DB() *gorm.DB { return s.db }

// UpdatePatch: after D13, patch.id IS the Wiki galgame_id, so changing vndb_id
// to one that resolves to a different galgame_id would require remapping
// patch.id (and every FK in child tables) — that is the job of the
// cmd/remap-patch-ids migration script, not a per-request handler.
//
// Here we accept rebinding only when the new vndb_id resolves to the same
// galgame_id we already have (i.e. Wiki updated the metadata for an existing
// galgame). Anything else is rejected with a clear hint.
func (s *PatchService) UpdatePatch(ctx context.Context, id, userID int, isPrivileged bool, vndbID string) error {
	existing, err := s.repo.GetPatchByID(id)
	if err != nil {
		return fmt.Errorf("patch not found")
	}
	if existing.UserID != userID && !isPrivileged {
		return fmt.Errorf("no permission to modify this patch")
	}

	exists, galgameID, err := s.wiki.CheckGalgameByVndbID(ctx, vndbID)
	if err != nil {
		return fmt.Errorf("调用 Wiki 校验 vndb_id 失败: %w", err)
	}
	if !exists {
		return fmt.Errorf("Galgame Wiki 中不存在 vndb_id=%s 的游戏", vndbID)
	}
	if galgameID != existing.ID {
		return fmt.Errorf("不允许把 patch (id=%d) 重绑到不同的 galgame (id=%d) — 请运行 cmd/remap-patch-ids 完整迁移", existing.ID, galgameID)
	}

	return s.db.Model(&model.Patch{}).Where("id = ?", id).
		Update("vndb_id", vndbID).Error
}

func (s *PatchService) DeletePatch(id, userID int, isAdmin bool) error {
	patch, err := s.repo.GetPatchByID(id)
	if err != nil {
		return fmt.Errorf("patch not found")
	}
	if patch.UserID != userID && !isAdmin {
		return fmt.Errorf("no permission to delete this patch")
	}
	return s.repo.DeletePatch(id)
}

func (s *PatchService) CheckDuplicate(vndbID string) (bool, error) {
	_, err := s.repo.FindPatchByVndbID(vndbID)
	if err == gorm.ErrRecordNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *PatchService) IncrementView(id int) error {
	return s.repo.IncrementView(id)
}

func (s *PatchService) GetRandomPatchID() (int, error) {
	return s.repo.GetRandomPatchID()
}

// ===== Comments =====

// GetComments returns a page of top-level comments (plus their replies),
// renders content_html, attaches publisher briefs from OAuth /users/batch,
// and marks is_liked for the given currentUID (0 = anonymous, no like marks).
func (s *PatchService) GetComments(ctx context.Context, patchID, currentUID, page, limit int) ([]model.PatchComment, int64, error) {
	offset := (page - 1) * limit
	comments, total, err := s.repo.GetComments(patchID, offset, limit)
	if err != nil {
		return comments, total, err
	}

	// Render content_html for every top-level comment and each reply. Done
	// here so all consumers of GetComments share the same rendered output.
	for i := range comments {
		comments[i].ContentHTML = markdown.MustRender(comments[i].Content)
		for j := range comments[i].Replies {
			comments[i].Replies[j].ContentHTML = markdown.MustRender(comments[i].Replies[j].Content)
		}
	}

	// Batch-fetch publisher briefs for top-level + replies in one OAuth call.
	uids := make([]int, 0, len(comments)*2)
	for i := range comments {
		uids = append(uids, comments[i].UserID)
		for j := range comments[i].Replies {
			uids = append(uids, comments[i].Replies[j].UserID)
		}
	}
	briefs := userclient.BriefMapByInt(ctx, s.users, uids)
	for i := range comments {
		comments[i].User = briefToPatchUser(briefs[comments[i].UserID])
		for j := range comments[i].Replies {
			comments[i].Replies[j].User = briefToPatchUser(briefs[comments[i].Replies[j].UserID])
		}
	}

	if currentUID == 0 || len(comments) == 0 {
		return comments, total, nil
	}

	// Collect all comment IDs (top-level + replies) for the like-marking query.
	ids := make([]int, 0, len(comments))
	for i := range comments {
		ids = append(ids, comments[i].ID)
		for j := range comments[i].Replies {
			ids = append(ids, comments[i].Replies[j].ID)
		}
	}
	liked, err := s.repo.GetLikedCommentIDs(currentUID, ids)
	if err != nil {
		return comments, total, nil
	}
	likedSet := make(map[int]bool, len(liked))
	for _, id := range liked {
		likedSet[id] = true
	}
	for i := range comments {
		comments[i].IsLiked = likedSet[comments[i].ID]
		for j := range comments[i].Replies {
			comments[i].Replies[j].IsLiked = likedSet[comments[i].Replies[j].ID]
		}
	}
	return comments, total, nil
}

// briefToPatchUser is the small adapter from OAuth /users/batch shape to the
// embedded PatchUser wire shape ({id, name, avatar}).
func briefToPatchUser(b *userclient.Brief) *model.PatchUser {
	if b == nil {
		return nil
	}
	return &model.PatchUser{ID: int(b.ID), Name: b.Name, Avatar: b.Avatar, AvatarImageHash: b.AvatarImageHash, Roles: b.Roles}
}

func (s *PatchService) CreateComment(patchID, userID int, content string, parentID *int) (*model.PatchComment, error) {
	comment := &model.PatchComment{
		GalgameID: patchID,
		UserID:   userID,
		Content:  content,
		ParentID: parentID,
	}
	if err := s.repo.CreateComment(comment); err != nil {
		return nil, err
	}

	// Update patch comment count
	s.repo.UpdateCount(patchID, "comment_count", 1)

	// Award moemoepoint to patch creator
	patch, _ := s.repo.GetPatchByID(patchID)
	if patch != nil && patch.UserID != userID {
		s.repo.UpdateMoemoepoint(patch.UserID, 1)
	}

	// Ensure contributor
	s.repo.EnsureContributor(userID, patchID)

	// Pre-render content_html so the immediate POST response can be appended
	// directly into the comment list on the frontend without a second fetch.
	comment.ContentHTML = markdown.MustRender(comment.Content)

	return comment, nil
}

func (s *PatchService) UpdateComment(commentID, userID int, content string) error {
	comment, err := s.repo.GetCommentByID(commentID)
	if err != nil {
		return fmt.Errorf("comment not found")
	}
	if comment.UserID != userID {
		return fmt.Errorf("can only edit your own comments")
	}
	comment.Content = content
	comment.Edit = time.Now().Format(time.RFC3339)
	return s.repo.UpdateComment(comment)
}

func (s *PatchService) DeleteComment(commentID, userID int, isPrivileged bool) error {
	comment, err := s.repo.GetCommentByID(commentID)
	if err != nil {
		return fmt.Errorf("comment not found")
	}
	if comment.UserID != userID && !isPrivileged {
		return fmt.Errorf("no permission to delete this comment")
	}

	count, _ := s.repo.CountCommentAndReplies(commentID)
	if err := s.repo.DeleteComment(commentID); err != nil {
		return err
	}
	s.repo.UpdateCount(comment.GalgameID, "comment_count", -int(count))
	return nil
}

func (s *PatchService) ToggleCommentLike(commentID, userID int) (bool, error) {
	comment, err := s.repo.GetCommentByID(commentID)
	if err != nil {
		return false, fmt.Errorf("comment not found")
	}

	existing, err := s.repo.FindCommentLike(userID, commentID)
	if err == nil {
		// Unlike
		s.repo.DeleteCommentLike(existing.ID)
		s.db.Model(&model.PatchComment{}).Where("id = ?", commentID).
			UpdateColumn("like_count", gorm.Expr("GREATEST(like_count - 1, 0)"))
		if comment.UserID != userID {
			s.repo.UpdateMoemoepoint(comment.UserID, -1)
		}
		return false, nil
	}

	// Like
	rel := &model.UserPatchCommentLikeRelation{UserID: userID, CommentID: commentID}
	s.repo.CreateCommentLike(rel)
	s.db.Model(&model.PatchComment{}).Where("id = ?", commentID).
		UpdateColumn("like_count", gorm.Expr("like_count + 1"))
	if comment.UserID != userID {
		s.repo.UpdateMoemoepoint(comment.UserID, 1)
	}
	return true, nil
}

func (s *PatchService) GetCommentMarkdown(commentID int) (string, error) {
	return s.repo.GetCommentMarkdown(commentID)
}

// ===== Resources =====

func (s *PatchService) GetResources(ctx context.Context, patchID, currentUID int) ([]model.PatchResource, error) {
	resources, err := s.repo.GetResources(patchID)
	if err != nil {
		return resources, err
	}
	model.RenderResourceNotes(resources)
	attachUsersToResources(ctx, s.users, resources)
	s.markResourceLiked(currentUID, resources)
	return resources, nil
}

// markResourceLiked stamps is_liked on each resource for the given currentUID.
// Anonymous (currentUID == 0) leaves is_liked false everywhere.
func (s *PatchService) markResourceLiked(currentUID int, rs []model.PatchResource) {
	if currentUID == 0 || len(rs) == 0 {
		return
	}
	ids := make([]int, 0, len(rs))
	for _, r := range rs {
		ids = append(ids, r.ID)
	}
	liked, err := s.repo.GetLikedResourceIDs(currentUID, ids)
	if err != nil {
		return
	}
	likedSet := make(map[int]bool, len(liked))
	for _, id := range liked {
		likedSet[id] = true
	}
	for i := range rs {
		rs[i].IsLiked = likedSet[rs[i].ID]
	}
}

// attachUsersToResources batch-fetches publisher briefs from OAuth and
// stamps the User field on each resource row.
func attachUsersToResources(ctx context.Context, users *userclient.Client, rs []model.PatchResource) {
	if users == nil || len(rs) == 0 {
		return
	}
	uids := make([]int, 0, len(rs))
	for _, r := range rs {
		uids = append(uids, r.UserID)
	}
	briefs := userclient.BriefMapByInt(ctx, users, uids)
	for i := range rs {
		rs[i].User = briefToPatchUser(briefs[rs[i].UserID])
	}
}

func (s *PatchService) CreateResource(resource *model.PatchResource, userID int) error {
	resource.UserID = userID

	if err := s.repo.CreateResource(resource); err != nil {
		return err
	}

	// Update aggregates
	s.repo.UpdateCount(resource.GalgameID, "resource_count", 1)
	s.repo.RecalculatePatchAggregates(resource.GalgameID)

	// Update resource_update_time
	s.db.Model(&model.Patch{}).Where("id = ?", resource.GalgameID).
		Update("resource_update_time", time.Now())

	// Moemoepoint +3
	s.repo.UpdateMoemoepoint(userID, 3)

	// Ensure contributor
	s.repo.EnsureContributor(userID, resource.GalgameID)

	// Notify favorited users
	s.notifyFavoritedUsers(resource.GalgameID, userID)

	// Pre-render note_html for the immediate POST response.
	resource.NoteHTML = markdown.MustRender(resource.Note)

	return nil
}

func (s *PatchService) UpdateResource(resourceID, userID int, update *model.PatchResource) error {
	existing, err := s.repo.GetResourceByID(resourceID)
	if err != nil {
		return fmt.Errorf("resource not found")
	}
	if existing.UserID != userID {
		return fmt.Errorf("can only edit your own resources")
	}

	existing.Storage = update.Storage
	existing.Name = update.Name
	existing.ModelName = update.ModelName
	existing.Size = update.Size
	existing.Code = update.Code
	existing.Password = update.Password
	existing.Note = update.Note
	existing.S3Key = update.S3Key
	existing.Content = update.Content
	existing.Type = update.Type
	existing.Language = update.Language
	existing.Platform = update.Platform
	existing.UpdateTime = time.Now()

	if err := s.repo.UpdateResource(existing); err != nil {
		return err
	}

	s.repo.RecalculatePatchAggregates(existing.GalgameID)
	return nil
}

func (s *PatchService) DeleteResource(resourceID, userID int) error {
	resource, err := s.repo.GetResourceByID(resourceID)
	if err != nil {
		return fmt.Errorf("resource not found")
	}
	if resource.UserID != userID {
		return fmt.Errorf("can only delete your own resources")
	}

	if err := s.repo.DeleteResource(resourceID); err != nil {
		return err
	}

	s.repo.UpdateCount(resource.GalgameID, "resource_count", -1)
	s.repo.RecalculatePatchAggregates(resource.GalgameID)
	s.repo.UpdateMoemoepoint(userID, -3)
	return nil
}

func (s *PatchService) ToggleResourceDisable(resourceID, userID int, isPrivileged bool) error {
	resource, err := s.repo.GetResourceByID(resourceID)
	if err != nil {
		return fmt.Errorf("resource not found")
	}
	if resource.UserID != userID && !isPrivileged {
		return fmt.Errorf("no permission to operate on this resource")
	}
	return s.repo.ToggleResourceStatus(resourceID)
}

func (s *PatchService) IncrementResourceDownload(resourceID int) error {
	resource, err := s.repo.GetResourceByID(resourceID)
	if err != nil {
		return fmt.Errorf("resource not found")
	}
	return s.repo.IncrementResourceDownload(resourceID, resource.GalgameID)
}

func (s *PatchService) ToggleResourceLike(resourceID, userID int) (bool, error) {
	resource, err := s.repo.GetResourceByID(resourceID)
	if err != nil {
		return false, fmt.Errorf("resource not found")
	}

	existing, err := s.repo.FindResourceLike(userID, resourceID)
	if err == nil {
		s.repo.DeleteResourceLike(existing.ID)
		s.db.Model(&model.PatchResource{}).Where("id = ?", resourceID).
			UpdateColumn("like_count", gorm.Expr("GREATEST(like_count - 1, 0)"))
		if resource.UserID != userID {
			s.repo.UpdateMoemoepoint(resource.UserID, -1)
		}
		return false, nil
	}

	rel := &model.UserPatchResourceLikeRelation{UserID: userID, ResourceID: resourceID}
	s.repo.CreateResourceLike(rel)
	s.db.Model(&model.PatchResource{}).Where("id = ?", resourceID).
		UpdateColumn("like_count", gorm.Expr("like_count + 1"))
	if resource.UserID != userID {
		s.repo.UpdateMoemoepoint(resource.UserID, 1)
	}
	return true, nil
}

// ===== Favorites =====

func (s *PatchService) ToggleFavorite(patchID, userID int) (bool, error) {
	patch, err := s.repo.GetPatchByID(patchID)
	if err != nil {
		return false, fmt.Errorf("patch not found")
	}

	existing, err := s.repo.FindFavorite(userID, patchID)
	if err == nil {
		s.repo.DeleteFavorite(existing.ID)
		s.repo.UpdateCount(patchID, "favorite_count", -1)
		if patch.UserID != userID {
			s.repo.UpdateMoemoepoint(patch.UserID, -1)
		}
		return false, nil
	}

	rel := &model.UserPatchFavoriteRelation{UserID: userID, GalgameID: patchID}
	s.repo.CreateFavorite(rel)
	s.repo.UpdateCount(patchID, "favorite_count", 1)
	if patch.UserID != userID {
		s.repo.UpdateMoemoepoint(patch.UserID, 1)
	}
	return true, nil
}

func (s *PatchService) IsFavorited(userID, patchID int) bool {
	_, err := s.repo.FindFavorite(userID, patchID)
	return err == nil
}

// ===== Contributors =====

// GetContributorIDs returns the user_ids of every contributor on a patch.
// Handler enriches them via OAuth /users/batch (pkg/userclient).
func (s *PatchService) GetContributorIDs(patchID int) ([]int, error) {
	return s.repo.GetContributorIDs(patchID)
}

// ===== Mention detection =====

// ExtractMentionUserIDs delegates to the markdown package so the regex used
// for notification routing matches exactly what the renderer treats as a
// mention link.
func (s *PatchService) ExtractMentionUserIDs(content string) []int {
	return markdown.ExtractMentionedUIDs(content)
}

// ===== Notifications (simplified) =====

func (s *PatchService) notifyFavoritedUsers(patchID, senderID int) {
	var userIDs []int
	s.db.Model(&model.UserPatchFavoriteRelation{}).
		Where("galgame_id = ? AND user_id != ?", patchID, senderID).
		Pluck("user_id", &userIDs)

	for _, uid := range userIDs {
		s.createDedupMessage(senderID, uid, "patchResourceCreate",
			"New resource added to patch",
			fmt.Sprintf("/patch/%d/resource", patchID))
	}
}

func (s *PatchService) createDedupMessage(senderID, recipientID int, msgType, content, link string) {
	var count int64
	s.db.Table("user_message").Where(
		"type = ? AND sender_id = ? AND recipient_id = ? AND link = ?",
		msgType, senderID, recipientID, link,
	).Count(&count)

	if count == 0 {
		s.db.Table("user_message").Create(map[string]any{
			"type":         msgType,
			"content":      content,
			"status":       0,
			"link":         link,
			"sender_id":    senderID,
			"recipient_id": recipientID,
			"created":      time.Now(),
			"updated":      time.Now(),
		})
	}
}

func (s *PatchService) CreateMentionMessages(senderID, patchID int, content string) {
	ids := s.ExtractMentionUserIDs(content)
	excerpt := content
	if len(excerpt) > 233 {
		excerpt = excerpt[:233]
	}
	for _, uid := range ids {
		if uid != senderID {
			s.createDedupMessage(senderID, uid, "mention", excerpt,
				fmt.Sprintf("/patch/%d", patchID))
		}
	}
}

func (s *PatchService) CreateCommentNotification(senderID int, comment *model.PatchComment) {
	if comment.ParentID != nil {
		parent, err := s.repo.GetCommentByID(*comment.ParentID)
		if err == nil && parent.UserID != senderID {
			s.createDedupMessage(senderID, parent.UserID, "comment",
				"Replied to your comment",
				fmt.Sprintf("/patch/%d", comment.GalgameID))
		}
	}
}

func (s *PatchService) CreateLikeCommentNotification(senderID int, comment *model.PatchComment) {
	if comment.UserID != senderID {
		s.createDedupMessage(senderID, comment.UserID, "likeComment",
			"Liked your comment",
			fmt.Sprintf("/patch/%d", comment.GalgameID))
	}
}

// ===== Admin Settings Check =====

func (s *PatchService) IsCommentVerifyEnabled() bool {
	val, err := s.rdb.Get(context.Background(), "admin:enable_comment_verify").Result()
	return err == nil && val == "true"
}
