package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/internal/infrastructure/markdown"
	"kun-galgame-patch-api/internal/infrastructure/storage"
	"kun-galgame-patch-api/internal/patch/model"
	"kun-galgame-patch-api/internal/patch/repository"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type PatchService struct {
	repo *repository.PatchRepository
	rdb  *redis.Client
	db   *gorm.DB
	s3   *storage.S3Client
	wiki *galgameClient.Client
}

func New(repo *repository.PatchRepository, rdb *redis.Client, db *gorm.DB, s3 *storage.S3Client, wiki *galgameClient.Client) *PatchService {
	return &PatchService{repo: repo, rdb: rdb, db: db, s3: s3, wiki: wiki}
}

// ===== Patch =====

// CreatePatch handles POST /api/patch (D12, 2026-04-21).
//
// The client only provides vndb_id; the server:
//  1. Calls Wiki /galgame/check?vndb_id=... to verify existence and fetch galgame_id
//  2. Checks whether a patch with the same vndb_id already exists locally
//  3. In one transaction: create the patch row, award +3 moemoepoint to the user, register contributor
//
// No banner upload (banner is fetched directly from Wiki).
func (s *PatchService) CreatePatch(ctx context.Context, uid int, vndbID string) (int, error) {
	// 1. Check with Wiki: must exist, and get galgame_id
	exists, galgameID, err := s.wiki.CheckGalgameByVndbID(ctx, vndbID)
	if err != nil {
		return 0, fmt.Errorf("调用 Wiki 校验 vndb_id 失败: %w", err)
	}
	if !exists {
		return 0, fmt.Errorf("Galgame Wiki 中不存在 vndb_id=%s 的游戏，请先在 Wiki 创建", vndbID)
	}

	// 2. Local dedup
	if existing, _ := s.repo.FindPatchByVndbID(vndbID); existing != nil && existing.ID != 0 {
		return 0, fmt.Errorf("该 VNDB ID 已经存在对应的补丁")
	}

	// 3. Transaction
	var patchID int
	txErr := s.db.Transaction(func(tx *gorm.DB) error {
		p := &model.Patch{
			VndbID:    vndbID,
			GalgameID: galgameID,
			UserID:    uid,
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
			UserID: uid, PatchID: p.ID,
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
	p, err := s.repo.GetPatchByID(id)
	if err != nil {
		return nil, err
	}
	s.selfHealGalgameID(p)
	return p, nil
}

func (s *PatchService) GetPatchDetail(id int) (*model.Patch, error) {
	p, err := s.repo.GetPatchDetail(id)
	if err != nil {
		return nil, err
	}
	s.selfHealGalgameID(p)
	return p, nil
}

// selfHealGalgameID looks up the Wiki by vndb_id when the local row is missing
// galgame_id. Hit means the patch is not really an orphan, just a stale row
// from migration / pre-Wiki publish; we backfill galgame_id in place so the
// next read is fast and the enricher can find the game.
//
// Failures (network blip, Wiki really doesn't have this vndb_id) fall through
// silently — the existing "missing name" rendering still works as the
// last-resort behavior for genuine orphans.
func (s *PatchService) selfHealGalgameID(p *model.Patch) {
	if p == nil || p.GalgameID > 0 || p.VndbID == "" {
		return
	}
	// Skip placeholder vndb_ids ("pending-N") created when a creator left it blank.
	if strings.HasPrefix(p.VndbID, "pending-") {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	exists, gid, err := s.wiki.CheckGalgameByVndbID(ctx, p.VndbID)
	if err != nil || !exists || gid <= 0 {
		return
	}

	if err := s.db.Model(&model.Patch{}).Where("id = ?", p.ID).
		UpdateColumn("galgame_id", gid).Error; err != nil {
		// Log but do not fail the read — backfill is best-effort.
		return
	}
	p.GalgameID = gid
}

// UpdatePatch: after D12, only "rebind vndb_id" is allowed (a rare case, e.g. a mislinked entry).
// Re-validates via Wiki and refreshes galgame_id.
func (s *PatchService) UpdatePatch(ctx context.Context, id, userID, userRole int, vndbID string) error {
	existing, err := s.repo.GetPatchByID(id)
	if err != nil {
		return fmt.Errorf("patch not found")
	}
	if existing.UserID != userID && userRole < 3 {
		return fmt.Errorf("no permission to modify this patch")
	}

	exists, galgameID, err := s.wiki.CheckGalgameByVndbID(ctx, vndbID)
	if err != nil {
		return fmt.Errorf("调用 Wiki 校验 vndb_id 失败: %w", err)
	}
	if !exists {
		return fmt.Errorf("Galgame Wiki 中不存在 vndb_id=%s 的游戏", vndbID)
	}

	return s.db.Model(&model.Patch{}).Where("id = ?", id).Updates(map[string]any{
		"vndb_id":    vndbID,
		"galgame_id": galgameID,
	}).Error
}

func (s *PatchService) DeletePatch(id, userID, userRole int) error {
	patch, err := s.repo.GetPatchByID(id)
	if err != nil {
		return fmt.Errorf("patch not found")
	}
	if patch.UserID != userID && userRole < 4 {
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
// renders content_html, and marks is_liked for the given currentUID
// (0 = anonymous, no like marks applied).
func (s *PatchService) GetComments(patchID, currentUID, page, limit int) ([]model.PatchComment, int64, error) {
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

	if currentUID == 0 || len(comments) == 0 {
		return comments, total, nil
	}

	// Collect all comment IDs (top-level + replies) in one pass.
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

func (s *PatchService) CreateComment(patchID, userID int, content string, parentID *int) (*model.PatchComment, error) {
	comment := &model.PatchComment{
		PatchID:  patchID,
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

func (s *PatchService) DeleteComment(commentID, userID, userRole int) error {
	comment, err := s.repo.GetCommentByID(commentID)
	if err != nil {
		return fmt.Errorf("comment not found")
	}
	if comment.UserID != userID && userRole < 3 {
		return fmt.Errorf("no permission to delete this comment")
	}

	count, _ := s.repo.CountCommentAndReplies(commentID)
	if err := s.repo.DeleteComment(commentID); err != nil {
		return err
	}
	s.repo.UpdateCount(comment.PatchID, "comment_count", -int(count))
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

func (s *PatchService) GetResources(patchID int) ([]model.PatchResource, error) {
	resources, err := s.repo.GetResources(patchID)
	if err == nil {
		model.RenderResourceNotes(resources)
	}
	return resources, err
}

func (s *PatchService) CreateResource(resource *model.PatchResource, userID int) error {
	resource.UserID = userID

	if err := s.repo.CreateResource(resource); err != nil {
		return err
	}

	// Update aggregates
	s.repo.UpdateCount(resource.PatchID, "resource_count", 1)
	s.repo.RecalculatePatchAggregates(resource.PatchID)

	// Update resource_update_time
	s.db.Model(&model.Patch{}).Where("id = ?", resource.PatchID).
		Update("resource_update_time", time.Now())

	// Moemoepoint +3
	s.repo.UpdateMoemoepoint(userID, 3)

	// Ensure contributor
	s.repo.EnsureContributor(userID, resource.PatchID)

	// Notify favorited users
	s.notifyFavoritedUsers(resource.PatchID, userID)

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

	s.repo.RecalculatePatchAggregates(existing.PatchID)
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

	s.repo.UpdateCount(resource.PatchID, "resource_count", -1)
	s.repo.RecalculatePatchAggregates(resource.PatchID)
	s.repo.UpdateMoemoepoint(userID, -3)
	return nil
}

func (s *PatchService) ToggleResourceDisable(resourceID, userID, userRole int) error {
	resource, err := s.repo.GetResourceByID(resourceID)
	if err != nil {
		return fmt.Errorf("resource not found")
	}
	if resource.UserID != userID && userRole < 3 {
		return fmt.Errorf("no permission to operate on this resource")
	}
	return s.repo.ToggleResourceStatus(resourceID)
}

func (s *PatchService) IncrementResourceDownload(resourceID int) error {
	resource, err := s.repo.GetResourceByID(resourceID)
	if err != nil {
		return fmt.Errorf("resource not found")
	}
	return s.repo.IncrementResourceDownload(resourceID, resource.PatchID)
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

	rel := &model.UserPatchFavoriteRelation{UserID: userID, PatchID: patchID}
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

func (s *PatchService) GetContributors(patchID int) ([]model.PatchUser, error) {
	return s.repo.GetContributors(patchID)
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
		Where("patch_id = ? AND user_id != ?", patchID, senderID).
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
				fmt.Sprintf("/patch/%d", comment.PatchID))
		}
	}
}

func (s *PatchService) CreateLikeCommentNotification(senderID int, comment *model.PatchComment) {
	if comment.UserID != senderID {
		s.createDedupMessage(senderID, comment.UserID, "likeComment",
			"Liked your comment",
			fmt.Sprintf("/patch/%d", comment.PatchID))
	}
}

// ===== Admin Settings Check =====

func (s *PatchService) IsCommentVerifyEnabled() bool {
	val, err := s.rdb.Get(context.Background(), "admin:enable_comment_verify").Result()
	return err == nil && val == "true"
}

func (s *PatchService) IsCreatorOnlyEnabled() bool {
	val, err := s.rdb.Get(context.Background(), "admin:enable_only_creator_create").Result()
	return err == nil && val == "true"
}
