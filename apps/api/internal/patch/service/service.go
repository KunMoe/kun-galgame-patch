package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"strconv"
	"strings"
	"time"

	authModel "kun-galgame-patch-api/internal/auth/model"
	"kun-galgame-patch-api/internal/favorite"
	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/internal/infrastructure/markdown"
	"kun-galgame-patch-api/internal/patch/model"
	"kun-galgame-patch-api/internal/patch/repository"
	settingService "kun-galgame-patch-api/internal/setting/service"
	"kun-galgame-patch-api/pkg/artifactclient"
	"kun-galgame-patch-api/pkg/catalogv2"
	"kun-galgame-patch-api/pkg/moemoepoint"
	"kun-galgame-patch-api/pkg/userclient"
	"kun-galgame-patch-api/pkg/utils"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrGalgameMissing = errors.New("galgame missing for vndb_id")

type AuditLogger interface {
	CreateLog(actorID int, action string, data any) error
}

type PatchService struct {
	repo    *repository.PatchRepository
	setting *settingService.Service
	db      *gorm.DB
	art     *artifactclient.Client
	galgame *galgameClient.Client
	users   *userclient.Client
	mp      *moemoepoint.Awarder
	audit   AuditLogger
}

func New(repo *repository.PatchRepository, setting *settingService.Service, db *gorm.DB, art *artifactclient.Client, galgame *galgameClient.Client, users *userclient.Client, mp *moemoepoint.Awarder, audit AuditLogger) *PatchService {
	return &PatchService{repo: repo, setting: setting, db: db, art: art, galgame: galgame, users: users, mp: mp, audit: audit}
}

func (s *PatchService) CreatePatchByGalgameID(ctx context.Context, userID, galgameID int) (int, error) {
	briefs, err := s.galgame.GalgameBatch(ctx, []int{galgameID}, "")
	if err != nil {
		return 0, fmt.Errorf("调用 galgame 校验失败: %w", err)
	}
	var brief *galgameClient.GalgameBrief
	for i := range briefs {
		if briefs[i].ID == galgameID {
			brief = &briefs[i]
			break
		}
	}
	if brief == nil {
		return 0, ErrGalgameMissing
	}
	vndb := brief.VndbID
	if vndb == "" {
		vndb = fmt.Sprintf("wiki-%d", galgameID)
	}
	return s.createPatchRow(ctx, userID, galgameID, vndb)
}

// resolveCatalogWork turns this site's vndb id into a catalog work. A
// `wiki-<n>` id is a catalog work id with a prefix — that is what it was
// minted from — and a `pending-<n>` placeholder has no work at all.
func (s *PatchService) resolveCatalogWork(ctx context.Context, vndbID string) *int64 {
	if rest, cut := strings.CutPrefix(vndbID, "wiki-"); cut {
		if n, err := strconv.ParseInt(rest, 10, 64); err == nil && n > 0 {
			return &n
		}
		return nil
	}
	if !strings.HasPrefix(vndbID, "v") {
		return nil
	}
	w, err := s.galgame.V2().WorkByRef(ctx, "vndb", vndbID, true)
	if err != nil || w == nil {
		return nil
	}
	n, ok := catalogv2.ParseID(w.ID)
	if !ok {
		return nil
	}
	return &n
}

func (s *PatchService) createPatchRow(ctx context.Context, userID, galgameID int, vndbID string) (int, error) {
	if existing, _ := s.repo.GetPatchDetail(galgameID); existing != nil && existing.ID != 0 {
		if existing.IsStub {
			return s.adoptStub(ctx, userID, galgameID)
		}
		return existing.ID, nil
	}

	var releaseDate *time.Time
	if env, gErr := s.galgame.GetGalgame(ctx, galgameID, ""); gErr == nil && env != nil && env.Galgame.ReleaseDate != nil {
		releaseDate = utils.ParseGalgameReleaseDate(*env.Galgame.ReleaseDate)
	}

	// Without this a game created after the folder cutover cannot be
	// favourited: a folder item names a catalog work and patch.id is a
	// different id space (migration 034). Resolution is best effort — a
	// `pending-<n>` placeholder has no work yet — and the backfill script
	// picks up whatever was missed.
	workID := s.resolveCatalogWork(ctx, vndbID)

	var patchID int
	txErr := s.db.Transaction(func(tx *gorm.DB) error {
		p := &model.Patch{
			ID:            galgameID,
			VndbID:        vndbID,
			UserID:        userID,
			ReleaseDate:   releaseDate,
			CatalogWorkID: workID,
		}
		if err := tx.Create(p).Error; err != nil {
			return fmt.Errorf("创建 patch 失败: %w", err)
		}
		patchID = p.ID

		if err := tx.Create(&model.UserPatchContributeRelation{
			UserID: userID, GalgameID: p.ID,
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
	go s.mp.Award(context.Background(), userID, 3, "content_approved",
		fmt.Sprintf("galgame:%d", patchID), fmt.Sprintf("moyu:patch_create:%d", patchID))
	return patchID, nil
}

func (s *PatchService) adoptStub(ctx context.Context, userID, galgameID int) (int, error) {
	var adopted bool
	txErr := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&model.Patch{}).
			Where("id = ? AND is_stub = ?", galgameID, true).
			Updates(map[string]any{"user_id": userID, "is_stub": false})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return nil
		}
		adopted = true

		rel := model.UserPatchContributeRelation{UserID: userID, GalgameID: galgameID}
		cr := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&rel)
		if cr.Error != nil {
			return cr.Error
		}
		if cr.RowsAffected > 0 {
			if err := tx.Model(&model.Patch{}).Where("id = ?", galgameID).
				UpdateColumn("contribute_count", gorm.Expr("contribute_count + 1")).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if txErr != nil {
		return 0, txErr
	}
	if adopted {
		go s.mp.Award(context.Background(), userID, 3, "content_approved",
			fmt.Sprintf("galgame:%d", galgameID), fmt.Sprintf("moyu:patch_create:%d", galgameID))
	}
	return galgameID, nil
}

func (s *PatchService) GetPatch(ctx context.Context, id int) (*model.Patch, error) {
	return s.repo.GetPatchDetail(id)
}

func (s *PatchService) GetPatchesByIDs(ids []int) ([]model.Patch, error) {
	return s.repo.GetPatchesByIDs(ids)
}

func (s *PatchService) GetPatchDetail(ctx context.Context, id int) (*model.Patch, error) {
	return s.repo.GetPatchDetail(id)
}

func (s *PatchService) ensureLocalPatch(ctx context.Context, id, actorID int) (*model.Patch, error) {
	patch, err := s.repo.GetPatchDetail(id)
	if err == nil {
		return patch, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	briefs, bErr := s.galgame.GalgameBatch(ctx, []int{id}, "")
	if bErr != nil {
		return nil, err
	}
	var brief *galgameClient.GalgameBrief
	for i := range briefs {
		if briefs[i].ID == id {
			brief = &briefs[i]
			break
		}
	}
	if brief == nil {
		return nil, err
	}

	vndb := brief.VndbID
	if vndb == "" {
		vndb = fmt.Sprintf("wiki-%d", id)
	}

	row := &model.Patch{ID: id, VndbID: vndb, UserID: actorID, IsStub: true}
	if row.UserID > 0 {
		if uErr := s.db.WithContext(ctx).
			Clauses(clause.OnConflict{DoNothing: true}).
			Create(&authModel.User{ID: row.UserID}).Error; uErr != nil {
			return nil, uErr
		}
	}
	if cErr := s.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(row).Error; cErr != nil {
		return nil, cErr
	}
	return s.repo.GetPatchDetail(id)
}

func (s *PatchService) RegisterClaimedGalgame(userID, galgameID int, vndbID string) (int, error) {
	if galgameID <= 0 {
		return 0, fmt.Errorf("invalid galgame id")
	}
	if existing, _ := s.repo.GetPatchByID(galgameID); existing != nil && existing.ID != 0 {
		s.TouchResourceUpdateTime(galgameID)
		return existing.ID, nil
	}

	var patchID int
	txErr := s.db.Transaction(func(tx *gorm.DB) error {
		p := &model.Patch{ID: galgameID, VndbID: vndbID, UserID: userID}
		if err := tx.Create(p).Error; err != nil {
			return fmt.Errorf("创建 patch 失败: %w", err)
		}
		patchID = p.ID

		if err := tx.Create(&model.UserPatchContributeRelation{
			UserID: userID, GalgameID: p.ID,
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
	go s.mp.Award(context.Background(), userID, 3, "content_approved",
		fmt.Sprintf("galgame:%d", patchID), fmt.Sprintf("moyu:claim:%d", patchID))
	return patchID, nil
}

func (s *PatchService) DB() *gorm.DB { return s.db }

func (s *PatchService) TouchResourceUpdateTime(gid int) {
	s.db.Model(&model.Patch{}).Where("id = ?", gid).
		Update("resource_update_time", time.Now())
}

func (s *PatchService) UpdatePatch(ctx context.Context, id, userID int, isPrivileged bool, vndbID string) error {
	existing, err := s.repo.GetPatchByID(id)
	if err != nil {
		return fmt.Errorf("patch not found")
	}
	if existing.UserID != userID && !isPrivileged {
		return fmt.Errorf("no permission to modify this patch")
	}

	exists, galgameID, err := s.galgame.CheckGalgameByVndbID(ctx, vndbID)
	if err != nil {
		return fmt.Errorf("调用 galgame 校验 vndb_id 失败: %w", err)
	}
	if !exists {
		return fmt.Errorf("Galgame 资料库中不存在 vndb_id=%s 的游戏", vndbID)
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

	uuids, uErr := s.repo.GetPatchLiveArtifactUUIDs(id)
	if uErr != nil {
		slog.Warn("DeletePatch: failed to enumerate artifact_uuids for cleanup", "patch_id", id, "error", uErr)
		uuids = nil
	}

	if err := s.repo.DeletePatch(id); err != nil {
		return err
	}

	if len(uuids) > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		s.SoftDeleteArtifacts(ctx, uuids)
	}
	return nil
}

func (s *PatchService) SoftDeleteArtifacts(ctx context.Context, uuids []string) {
	for _, uuid := range uuids {
		if uuid == "" {
			continue
		}
		if err := s.art.Delete(ctx, uuid); err != nil {
			slog.Warn("SoftDeleteArtifacts: 软删 artifact 失败", "artifact_uuid", uuid, "error", err)
		}
	}
}

func (s *PatchService) IncrementView(id int) error {
	return s.repo.IncrementView(id)
}

func (s *PatchService) GetRandomPatchID(ctx context.Context, contentLimit string, includeEmpty bool) (int, error) {
	if contentLimit == "" {
		return s.repo.GetRandomPatchID(includeEmpty)
	}
	const sampleSize = 60
	ids, err := s.repo.GetRandomPatchIDs(sampleSize, includeEmpty)
	if err != nil || len(ids) == 0 {
		return 0, err
	}
	briefs, bErr := s.galgame.GalgameBatch(ctx, ids, contentLimit)
	if bErr != nil {
		return 0, bErr
	}
	if len(briefs) == 0 {
		return 0, gorm.ErrRecordNotFound
	}
	return briefs[rand.Intn(len(briefs))].ID, nil
}

func (s *PatchService) GetComments(ctx context.Context, patchID, currentUID, page, limit int) ([]model.PatchComment, int64, error) {
	offset := (page - 1) * limit
	comments, total, err := s.repo.GetComments(patchID, offset, limit)
	if err != nil {
		return comments, total, err
	}
	s.enrichComments(ctx, comments, currentUID)
	return comments, total, nil
}

func (s *PatchService) GetResourceComments(ctx context.Context, resourceID, currentUID, page, limit int) ([]model.PatchComment, int64, error) {
	offset := (page - 1) * limit
	comments, total, err := s.repo.GetResourceComments(resourceID, offset, limit)
	if err != nil {
		return comments, total, err
	}
	s.enrichComments(ctx, comments, currentUID)
	return comments, total, nil
}

func (s *PatchService) CountResourceComments(resourceID int) (int64, error) {
	return s.repo.CountResourceComments(resourceID)
}

func (s *PatchService) GetResourcePatchID(resourceID int) (int, error) {
	return s.repo.GetResourcePatchID(resourceID)
}

func (s *PatchService) enrichComments(ctx context.Context, comments []model.PatchComment, currentUID int) {
	for i := range comments {
		comments[i].ContentHTML = markdown.MustRender(comments[i].Content)
		for j := range comments[i].Replies {
			comments[i].Replies[j].ContentHTML = markdown.MustRender(comments[i].Replies[j].Content)
		}
	}

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
		return
	}

	ids := make([]int, 0, len(comments))
	for i := range comments {
		ids = append(ids, comments[i].ID)
		for j := range comments[i].Replies {
			ids = append(ids, comments[i].Replies[j].ID)
		}
	}
	liked, err := s.repo.GetLikedCommentIDs(currentUID, ids)
	if err != nil {
		return
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
}

func briefToPatchUser(b *userclient.Brief) *model.PatchUser {
	if b == nil {
		return nil
	}
	return &model.PatchUser{ID: int(b.ID), Name: b.Name, Avatar: b.Avatar, AvatarImageHash: b.AvatarImageHash, Roles: b.Roles, SiteRoles: b.SiteRoles}
}

func (s *PatchService) CreateComment(patchID, userID int, content string, parentID *int) (*model.PatchComment, error) {
	if _, err := s.ensureLocalPatch(context.Background(), patchID, userID); err != nil {
		return nil, fmt.Errorf("patch not found")
	}
	if err := s.checkCommentParent(parentID, patchID, nil); err != nil {
		return nil, err
	}
	return s.createComment(patchID, nil, userID, content, parentID)
}

func (s *PatchService) CreateResourceComment(resourceID, userID int, content string, parentID *int) (*model.PatchComment, error) {
	resource, err := s.repo.GetResourceByID(resourceID)
	if err != nil {
		return nil, fmt.Errorf("resource not found")
	}
	if resource.Status == 2 {
		return nil, fmt.Errorf("resource not found")
	}
	if err := s.checkCommentParent(parentID, resource.GalgameID, &resourceID); err != nil {
		return nil, err
	}
	return s.createComment(resource.GalgameID, &resourceID, userID, content, parentID)
}

func (s *PatchService) checkCommentParent(parentID *int, galgameID int, resourceID *int) error {
	if parentID == nil {
		return nil
	}
	parent, err := s.repo.GetCommentByID(*parentID)
	if err != nil {
		return fmt.Errorf("parent comment not found")
	}
	if parent.ParentID != nil {
		return fmt.Errorf("cannot reply to a reply")
	}
	sameArea := parent.GalgameID == galgameID &&
		((parent.ResourceID == nil) == (resourceID == nil)) &&
		(resourceID == nil || *parent.ResourceID == *resourceID)
	if !sameArea {
		return fmt.Errorf("parent comment belongs to another discussion")
	}
	return nil
}

func (s *PatchService) createComment(patchID int, resourceID *int, userID int, content string, parentID *int) (*model.PatchComment, error) {
	pending := s.IsCommentVerifyEnabled()
	status := 0
	if pending {
		status = 1
	}
	comment := &model.PatchComment{
		GalgameID:  patchID,
		ResourceID: resourceID,
		UserID:     userID,
		Content:    content,
		ParentID:   parentID,
		Status:     status,
	}
	if err := s.repo.CreateComment(comment); err != nil {
		return nil, err
	}

	if !pending {
		s.applyCommentSideEffects(patchID, userID, comment.ID)
	}

	comment.ContentHTML = markdown.MustRender(comment.Content)

	return comment, nil
}

func (s *PatchService) applyCommentSideEffects(patchID, userID, commentID int) {
	s.repo.UpdateCount(patchID, "comment_count", 1)
	patch, _ := s.repo.GetPatchByID(patchID)
	if patch != nil && patch.UserID != userID {
		go s.mp.Award(context.Background(), patch.UserID, 1, "liked",
			fmt.Sprintf("comment:%d", commentID), fmt.Sprintf("moyu:comment:%d", commentID))
	}
	s.repo.EnsureContributor(userID, patchID)
}

func (s *PatchService) ApproveComment(commentID int) (*model.PatchComment, error) {
	comment, err := s.repo.GetCommentByID(commentID)
	if err != nil {
		return nil, fmt.Errorf("comment not found")
	}
	if comment.Status == 0 {
		comment.ContentHTML = markdown.MustRender(comment.Content)
		return comment, nil
	}
	if err := s.repo.UpdateCommentStatus(commentID, 0); err != nil {
		return nil, err
	}
	comment.Status = 0
	s.applyCommentSideEffects(comment.GalgameID, comment.UserID, comment.ID)
	comment.ContentHTML = markdown.MustRender(comment.Content)
	return comment, nil
}

func (s *PatchService) UpdateComment(commentID, userID int, content string) (*model.PatchComment, error) {
	comment, err := s.repo.GetCommentByID(commentID)
	if err != nil {
		return nil, fmt.Errorf("comment not found")
	}
	if comment.UserID != userID {
		return nil, fmt.Errorf("can only edit your own comments")
	}
	comment.Content = content
	comment.Edit = time.Now().Format(time.RFC3339)
	if err := s.repo.UpdateComment(comment); err != nil {
		return nil, err
	}
	comment.ContentHTML = markdown.MustRender(comment.Content)
	return comment, nil
}

func (s *PatchService) DeleteComment(commentID, userID int, isPrivileged bool, reason string) error {
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

	if comment.UserID != userID {
		content := "您发布的评论已被版主删除。"
		if reason != "" {
			content += "原因：" + reason
		} else {
			content += "如有疑问可联系管理员。"
		}
		area := fmt.Sprintf("/patch/%d/comment", comment.GalgameID)
		if comment.ResourceID != nil {
			area = fmt.Sprintf("/resource/%d", *comment.ResourceID)
		}
		if err := s.db.Table("user_message").Create(map[string]any{
			"type":         "system",
			"content":      content,
			"status":       0,
			"link":         area,
			"sender_id":    nil,
			"recipient_id": comment.UserID,
			"created":      time.Now(),
			"updated":      time.Now(),
		}).Error; err != nil {
			slog.Warn("DeleteComment: 写评论删除通知失败",
				"comment_id", commentID, "owner", comment.UserID, "error", err)
		}
		if s.audit != nil && userID != 0 {
			_ = s.audit.CreateLog(userID, "deleteComment", map[string]any{
				"comment_id": commentID,
				"owner_id":   comment.UserID,
				"galgame_id": comment.GalgameID,
				"reason":     reason,
			})
		}
	}
	return nil
}

func (s *PatchService) ToggleCommentLike(commentID, userID int) (bool, error) {
	comment, err := s.repo.GetCommentByID(commentID)
	if err != nil {
		return false, fmt.Errorf("comment not found")
	}

	existing, err := s.repo.FindCommentLike(userID, commentID)
	if err == nil {
		s.repo.DeleteCommentLike(existing.ID)
		s.db.Model(&model.PatchComment{}).Where("id = ?", commentID).
			UpdateColumn("like_count", gorm.Expr("GREATEST(like_count - 1, 0)"))
		if comment.UserID != userID {
			go s.mp.Award(context.Background(), comment.UserID, -1, "liked",
				fmt.Sprintf("comment:%d", commentID), fmt.Sprintf("moyu:comment_unlike:%d", existing.ID))
		}
		return false, nil
	}

	rel := &model.UserPatchCommentLikeRelation{UserID: userID, CommentID: commentID}
	s.repo.CreateCommentLike(rel)
	s.db.Model(&model.PatchComment{}).Where("id = ?", commentID).
		UpdateColumn("like_count", gorm.Expr("like_count + 1"))
	if comment.UserID != userID {
		go s.mp.Award(context.Background(), comment.UserID, 1, "liked",
			fmt.Sprintf("comment:%d", commentID), fmt.Sprintf("moyu:comment_like:%d", rel.ID))
		go s.CreateLikeCommentNotification(userID, comment)
	}
	return true, nil
}

func (s *PatchService) GetCommentMarkdown(commentID int) (string, error) {
	return s.repo.GetCommentMarkdown(commentID)
}

func (s *PatchService) GetCommentPatchID(commentID int) (int, error) {
	return s.repo.GetCommentPatchID(commentID)
}

func (s *PatchService) GetResources(ctx context.Context, patchID, currentUID int) ([]model.PatchResource, error) {
	resources, err := s.repo.GetResources(patchID)
	if err != nil {
		return resources, err
	}
	model.RenderResourceNotes(resources)
	attachUsersToResources(ctx, s.users, resources)
	s.markResourceLiked(currentUID, resources)
	s.markResourceFavorited(currentUID, resources)
	return resources, nil
}

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

func (s *PatchService) markResourceFavorited(currentUID int, rs []model.PatchResource) {
	if currentUID == 0 || len(rs) == 0 {
		return
	}
	ids := make([]int, 0, len(rs))
	for _, r := range rs {
		ids = append(ids, r.ID)
	}
	favorited, err := s.repo.GetFavoritedResourceIDs(currentUID, ids)
	if err != nil {
		return
	}
	favSet := make(map[int]bool, len(favorited))
	for _, id := range favorited {
		favSet[id] = true
	}
	for i := range rs {
		rs[i].IsFavorite = favSet[rs[i].ID]
	}
}

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

func (s *PatchService) CreateResource(ctx context.Context, resource *model.PatchResource, userID int) error {
	resource.UserID = userID

	if _, err := s.ensureLocalPatch(ctx, resource.GalgameID, userID); err != nil {
		return fmt.Errorf("patch not found")
	}

	if resource.Storage == "s3" {
		if resource.ArtifactUUID == "" {
			return fmt.Errorf("缺少上传文件标识")
		}
		resource.S3Key = ""
		resource.Content = ""
	} else {
		if strings.TrimSpace(resource.Content) == "" {
			return fmt.Errorf("请填写资源链接")
		}
	}

	if err := s.repo.CreateResource(resource); err != nil {
		msg := err.Error()
		if strings.Contains(msg, "idx_patch_resource_s3_key_unique") ||
			strings.Contains(msg, "idx_patch_resource_artifact_uuid_unique") ||
			strings.Contains(msg, "duplicate key value") {
			return fmt.Errorf("该上传已被其它资源占用，请重新上传一次")
		}
		return err
	}

	s.repo.UpdateCount(resource.GalgameID, "resource_count", 1)
	s.repo.RecalculatePatchAggregates(resource.GalgameID)
	if err := s.repo.MarkIndexed(resource.GalgameID); err != nil {
		slog.Warn("CreateResource: 标记 SEO 索引失败", "gid", resource.GalgameID, "error", err)
	}

	s.db.Model(&model.Patch{}).Where("id = ?", resource.GalgameID).
		Update("resource_update_time", time.Now())

	go s.mp.Award(context.Background(), userID, 3, "content_approved",
		fmt.Sprintf("resource:%d", resource.ID), fmt.Sprintf("moyu:resource_publish:%d", resource.ID))

	s.repo.EnsureContributor(userID, resource.GalgameID)

	go s.notifyFavoritedUsers(resource.GalgameID, userID)

	resource.NoteHTML = markdown.MustRender(resource.Note)

	if s.users != nil {
		one := []model.PatchResource{*resource}
		attachUsersToResources(ctx, s.users, one)
		resource.User = one[0].User
	}

	return nil
}

func (s *PatchService) UpdateResource(ctx context.Context, resourceID, userID int, update *model.PatchResource, reason string, actorRole int) (*model.PatchResource, error) {
	existing, err := s.repo.GetResourceByID(resourceID)
	if err != nil {
		return nil, fmt.Errorf("resource not found")
	}
	if existing.UserID != userID && actorRole < 2 {
		return nil, fmt.Errorf("can only edit your own resources")
	}

	if update.Storage == "s3" {
		switch {
		case update.ArtifactUUID != "":
			update.S3Key = ""
			update.Content = ""
		case update.S3Key != "":
			update.Content = update.S3Key
		default:
			return nil, fmt.Errorf("缺少上传文件标识")
		}
	} else {
		if strings.TrimSpace(update.Content) == "" {
			return nil, fmt.Errorf("请填写资源链接")
		}
	}

	fileChanged := update.Storage != existing.Storage ||
		update.S3Key != existing.S3Key ||
		update.Content != existing.Content ||
		update.ArtifactUUID != existing.ArtifactUUID

	var orphanArtifactUUID string
	if existing.ArtifactUUID != "" && update.ArtifactUUID != existing.ArtifactUUID {
		orphanArtifactUUID = existing.ArtifactUUID
	}

	galgameID := existing.GalgameID
	changes := diffResourceFields(existing, update)
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if fileChanged {
			hist := &model.PatchResourceFileHistory{
				ResourceID:      existing.ID,
				OldStorage:      existing.Storage,
				OldS3Key:        existing.S3Key,
				OldArtifactUUID: existing.ArtifactUUID,
				OldBlake3:       existing.Blake3,
				OldSize:         existing.Size,
				OldContent:      existing.Content,
				Reason:          reason,
				ActorID:         userID,
				ActorRole:       actorRole,
			}
			if err := tx.Create(hist).Error; err != nil {
				return fmt.Errorf("write file history: %w", err)
			}
		}

		if len(changes) > 0 {
			rev := &model.PatchResourceRevision{
				ResourceID: existing.ID,
				Action:     "updated",
				Changes:    changes,
				Reason:     reason,
				ActorID:    userID,
				ActorRole:  actorRole,
			}
			if err := tx.Create(rev).Error; err != nil {
				return fmt.Errorf("write resource revision: %w", err)
			}
		}

		existing.Storage = update.Storage
		existing.Name = update.Name
		existing.ModelName = update.ModelName
		existing.Size = update.Size
		existing.Code = update.Code
		existing.Password = update.Password
		existing.Note = update.Note
		existing.S3Key = update.S3Key
		existing.ArtifactUUID = update.ArtifactUUID
		existing.Content = update.Content
		existing.Type = update.Type
		existing.Language = update.Language
		existing.Platform = update.Platform
		existing.UpdateTime = time.Now()

		return tx.Save(existing).Error
	}); err != nil {
		return nil, err
	}

	if orphanArtifactUUID != "" {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.art.Delete(cleanupCtx, orphanArtifactUUID); err != nil {
			slog.Warn("UpdateResource: 软删旧 artifact 失败", "artifact_uuid", orphanArtifactUUID, "resource_id", resourceID, "error", err)
		}
	}

	s.repo.RecalculatePatchAggregates(galgameID)

	s.db.Model(&model.Patch{}).Where("id = ?", galgameID).
		Update("resource_update_time", time.Now())

	existing.NoteHTML = markdown.MustRender(existing.Note)
	if s.users != nil {
		one := []model.PatchResource{*existing}
		attachUsersToResources(ctx, s.users, one)
		existing.User = one[0].User
	}

	if fileChanged {
		s.notifyResourceFavoritedUsers(resourceID, userID)
	}
	return existing, nil
}

func (s *PatchService) DeleteResource(resourceID, userID int, isPrivileged bool, reason string) error {
	resource, err := s.repo.GetResourceByID(resourceID)
	if err != nil {
		return fmt.Errorf("resource not found")
	}
	if resource.UserID != userID && !isPrivileged {
		return fmt.Errorf("can only delete your own resources")
	}

	if err := s.repo.DeleteResource(resourceID); err != nil {
		return err
	}

	if resource.ArtifactUUID != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.art.Delete(ctx, resource.ArtifactUUID); err != nil {
			slog.Warn("DeleteResource: 软删 artifact 失败", "artifact_uuid", resource.ArtifactUUID, "resource_id", resourceID, "error", err)
		}
	}

	s.repo.UpdateCount(resource.GalgameID, "resource_count", -1)
	s.repo.RecalculatePatchAggregates(resource.GalgameID)
	go s.mp.Award(context.Background(), resource.UserID, -3, "content_removed",
		fmt.Sprintf("resource:%d", resource.ID), fmt.Sprintf("moyu:resource_delete:%d", resource.ID))

	if resource.UserID != userID {
		subject := "一个补丁资源"
		if resource.Name != "" {
			subject = fmt.Sprintf("补丁资源「%s」", resource.Name)
		}
		content := fmt.Sprintf("您发布的%s已被版主删除。", subject)
		if reason != "" {
			content += "原因：" + reason
		} else {
			content += "如有疑问可联系管理员。"
		}
		if err := s.db.Table("user_message").Create(map[string]any{
			"type":         "system",
			"content":      content,
			"status":       0,
			"link":         fmt.Sprintf("/patch/%d/resource", resource.GalgameID),
			"sender_id":    nil,
			"recipient_id": resource.UserID,
			"created":      time.Now(),
			"updated":      time.Now(),
		}).Error; err != nil {
			slog.Warn("DeleteResource: 写资源删除通知失败",
				"resource_id", resourceID, "owner", resource.UserID, "error", err)
		}

		if s.audit != nil && userID != 0 {
			_ = s.audit.CreateLog(userID, "deleteResource", map[string]any{
				"resource_id": resource.ID,
				"owner_id":    resource.UserID,
				"galgame_id":  resource.GalgameID,
				"name":        resource.Name,
				"reason":      reason,
			})
		}
	}
	return nil
}

func (s *PatchService) ToggleResourceDisable(resourceID, userID int, isPrivileged bool) (int, error) {
	resource, err := s.repo.GetResourceByID(resourceID)
	if err != nil {
		return 0, fmt.Errorf("resource not found")
	}
	if resource.UserID != userID && !isPrivileged {
		return 0, fmt.Errorf("no permission to operate on this resource")
	}
	if resource.Status == 2 {
		return 0, fmt.Errorf("resource is hidden by moderation and cannot be toggled")
	}
	if err := s.repo.ToggleResourceStatus(resourceID); err != nil {
		return 0, err
	}
	if resource.Status == 0 {
		return 1, nil
	}
	return 0, nil
}

func (s *PatchService) IncrementResourceDownload(resourceID int) error {
	resource, err := s.repo.GetResourceByID(resourceID)
	if err != nil {
		return fmt.Errorf("resource not found")
	}
	return s.repo.IncrementResourceDownload(resourceID, resource.GalgameID)
}

func (s *PatchService) GetResourceDownloadInfo(resourceID int) (*model.PatchResource, error) {
	r, err := s.repo.GetResourceByID(resourceID)
	if err != nil {
		return nil, fmt.Errorf("resource not found")
	}
	return r, nil
}

func (s *PatchService) ResolveDownloadURL(ctx context.Context, r *model.PatchResource) error {
	if r == nil || r.ArtifactUUID == "" {
		return nil
	}
	dl, err := s.art.Download(ctx, r.ArtifactUUID)
	if err != nil {
		return fmt.Errorf("获取下载地址失败: %w", err)
	}
	r.DownloadURL = dl.Url
	return nil
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
			go s.mp.Award(context.Background(), resource.UserID, -1, "liked",
				fmt.Sprintf("resource:%d", resourceID), fmt.Sprintf("moyu:resource_unlike:%d", existing.ID))
		}
		return false, nil
	}

	rel := &model.UserPatchResourceLikeRelation{UserID: userID, ResourceID: resourceID}
	s.repo.CreateResourceLike(rel)
	s.db.Model(&model.PatchResource{}).Where("id = ?", resourceID).
		UpdateColumn("like_count", gorm.Expr("like_count + 1"))
	if resource.UserID != userID {
		go s.mp.Award(context.Background(), resource.UserID, 1, "liked",
			fmt.Sprintf("resource:%d", resourceID), fmt.Sprintf("moyu:resource_like:%d", rel.ID))
		go s.notifyContentInteraction(userID, resource.UserID, resource.GalgameID,
			"likeResource", fmt.Sprintf("/resource/%d", resourceID))
	}
	return true, nil
}

func (s *PatchService) ToggleResourceFavorite(resourceID, userID int) (bool, error) {
	resource, err := s.repo.GetResourceByID(resourceID)
	if err != nil {
		return false, fmt.Errorf("resource not found")
	}
	existing, err := s.repo.FindResourceFavorite(userID, resourceID)
	if err == nil {
		if delErr := s.repo.DeleteResourceFavorite(existing.ID); delErr != nil {
			return false, delErr
		}
		return false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return false, err
	}
	if err := s.repo.CreateResourceFavorite(&model.UserPatchResourceFavoriteRelation{UserID: userID, ResourceID: resourceID}); err != nil {
		return false, err
	}
	if resource.UserID != userID {
		go s.notifyContentInteraction(userID, resource.UserID, resource.GalgameID,
			"favoriteResource", fmt.Sprintf("/resource/%d", resourceID))
	}
	return true, nil
}

func (s *PatchService) IsResourceFavorited(userID, resourceID int) bool {
	_, err := s.repo.FindResourceFavorite(userID, resourceID)
	return err == nil
}

// ToggleFavorite and IsFavorited used to read and write
// user_patch_favorite_relation. Favourites are catalog folder memberships
// since the cutover; see service/folders.go. The table is frozen as rollback
// material and read by nothing.

func (s *PatchService) GetContributorIDs(patchID int) ([]int, error) {
	return s.repo.GetContributorIDs(patchID)
}

func (s *PatchService) ExtractMentionUserIDs(content string) []int {
	return markdown.ExtractMentionedUserIDs(content)
}

// The audience is the catalog's, because the folders have been the only record
// of who favourited a game since the 2026-09-07 cutover. Two things about that
// answer cannot be seen from this file.
//
// It needs folder_holders:read on the application key. The scope is granted by
// an operator, never self-service, so a deployment whose key has not been
// granted it gets 403 here and nowhere else.
//
// And it names central sign-in uids, which are not this site's users: the
// folders are shared with the forum, and 5308 of the 11005 accounts holding one
// have no row in "user" here. user_message.recipient_id is a foreign key and
// createDedupMessage drops Create's error, so an unfiltered fan-out would
// insert nothing for those and say nothing about it.
func (s *PatchService) notifyFavoritedUsers(patchID, senderID int) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	byPatch, err := utils.WorkIDsByPatchIDs(s.db, []int{patchID})
	if err != nil || byPatch[patchID] == 0 {
		return
	}
	holders, err := favorite.Holders(ctx, s.galgame, byPatch[patchID])
	if err != nil {
		slog.Warn("notifyFavoritedUsers: 读取 catalog 收藏者失败", "patch_id", patchID, "error", err)
		return
	}
	if len(holders) == 0 {
		return
	}

	var userIDs []int
	if err := s.db.Table("user").Where("id IN ? AND id != ?", holders, senderID).
		Pluck("id", &userIDs).Error; err != nil {
		slog.Warn("notifyFavoritedUsers: 收藏者与本站用户求交失败", "patch_id", patchID, "error", err)
		return
	}

	for _, userID := range userIDs {
		s.createDedupMessage(senderID, userID, "patchResourceCreate",
			"您收藏的游戏发布了新补丁资源",
			fmt.Sprintf("/patch/%d/resource", patchID), true)
	}
}

func (s *PatchService) notifyResourceFavoritedUsers(resourceID, senderID int) {
	var userIDs []int
	s.db.Model(&model.UserPatchResourceFavoriteRelation{}).
		Where("resource_id = ? AND user_id != ?", resourceID, senderID).
		Pluck("user_id", &userIDs)

	for _, userID := range userIDs {
		s.createDedupMessage(senderID, userID, "patchResourceUpdate",
			"您收藏的补丁资源有更新",
			fmt.Sprintf("/resource/%d", resourceID), true)
	}
}

func (s *PatchService) createDedupMessage(senderID, recipientID int, msgType, content, link string, redeliverAfterRead bool) {
	q := s.db.Table("user_message").Where(
		"type = ? AND sender_id = ? AND recipient_id = ? AND link = ?",
		msgType, senderID, recipientID, link,
	)
	if redeliverAfterRead {
		q = q.Where("status = ?", 0)
	}
	var count int64
	q.Count(&count)

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

func commentAnchorLink(c *model.PatchComment) string {
	if c.ResourceID != nil {
		return fmt.Sprintf("/resource/%d#comment-%d", *c.ResourceID, c.ID)
	}
	return fmt.Sprintf("/patch/%d/comment#comment-%d", c.GalgameID, c.ID)
}

func (s *PatchService) CreateMentionMessages(senderID int, comment *model.PatchComment, content string) {
	ids := s.ExtractMentionUserIDs(content)
	excerpt := content
	if len(excerpt) > 233 {
		excerpt = excerpt[:233]
	}
	link := commentAnchorLink(comment)
	for _, userID := range ids {
		if userID != senderID {
			s.createDedupMessage(senderID, userID, "mention", excerpt, link, false)
		}
	}
}

func (s *PatchService) CreateCommentNotification(senderID int, comment *model.PatchComment) {
	if comment.ParentID != nil {
		parent, err := s.repo.GetCommentByID(*comment.ParentID)
		if err == nil && parent.UserID != senderID {
			s.createDedupMessage(senderID, parent.UserID, "comment",
				"回复了您的评论", commentAnchorLink(comment), false)
		}
	}
}

type LocateCommentResult struct {
	Page       int  `json:"page"`
	RootID     int  `json:"root_id"`
	IsReply    bool `json:"is_reply"`
	GalgameID  int  `json:"galgame_id"`
	ResourceID *int `json:"resource_id,omitempty"`
}

func (s *PatchService) LocateComment(commentID, limit int) (*LocateCommentResult, error) {
	if limit <= 0 || limit > 30 {
		limit = 30
	}
	c, err := s.repo.GetCommentByID(commentID)
	if err != nil {
		return nil, fmt.Errorf("comment not found")
	}
	root := c
	isReply := false
	if c.ParentID != nil {
		isReply = true
		root, err = s.repo.GetCommentByID(*c.ParentID)
		if err != nil {
			return nil, fmt.Errorf("comment not found")
		}
	}
	if root.Status != 0 || root.ParentID != nil {
		return nil, fmt.Errorf("comment not locatable")
	}
	before, err := s.repo.CountRootCommentsBefore(root)
	if err != nil {
		return nil, err
	}
	return &LocateCommentResult{
		Page:       int(before)/limit + 1,
		RootID:     root.ID,
		IsReply:    isReply,
		GalgameID:  root.GalgameID,
		ResourceID: root.ResourceID,
	}, nil
}

func (s *PatchService) CreateLikeCommentNotification(senderID int, comment *model.PatchComment) {
	if comment.UserID != senderID {
		s.createDedupMessage(senderID, comment.UserID, "likeComment",
			"赞了您的评论", commentAnchorLink(comment), false)
	}
}

func galgameDisplayName(b *galgameClient.GalgameBrief) string {
	for _, n := range []string{b.NameZhCn, b.NameJaJp, b.NameEnUs, b.NameZhTw} {
		if n != "" {
			return n
		}
	}
	return b.VndbID
}

func (s *PatchService) resolveGalgameName(patchID int) string {
	briefs, err := s.galgame.GalgameBatch(context.Background(), []int{patchID}, "")
	if err != nil {
		return ""
	}
	for i := range briefs {
		if briefs[i].ID == patchID {
			return galgameDisplayName(&briefs[i])
		}
	}
	return ""
}

func (s *PatchService) notifyContentInteraction(actorID, ownerID, patchID int, msgType, link string) {
	if ownerID == 0 || ownerID == actorID {
		return
	}
	name := s.resolveGalgameName(patchID)
	var content string
	switch msgType {
	case "likeResource":
		content = "点赞了您发布的补丁资源"
		if name != "" {
			content = fmt.Sprintf("点赞了您在 %s 下发布的补丁资源", name)
		}
	case "favoriteResource":
		content = "收藏了您发布的补丁资源"
		if name != "" {
			content = fmt.Sprintf("收藏了您在 %s 下发布的补丁资源", name)
		}
	case "favorite":
		content = "收藏了您发布的补丁"
		if name != "" {
			content = name
		}
	default:
		return
	}
	s.createDedupMessage(actorID, ownerID, msgType, content, link, false)
}

func (s *PatchService) IsCommentVerifyEnabled() bool {
	return s.setting.GetBool(settingService.KeyCommentVerify)
}

func (s *PatchService) IsCreatorOnlyEnabled() bool {
	return s.setting.GetBool(settingService.KeyCreatorOnly)
}

func diffResourceFields(before, after *model.PatchResource) model.ResourceChangeList {
	var ch model.ResourceChangeList
	addStr := func(field, label, b, a string) {
		if b != a {
			ch = append(ch, model.ResourceFieldChange{Field: field, Label: label, Before: b, After: a})
		}
	}
	addArr := func(field, label string, b, a model.JSONArray) {
		bs, as := strings.Join(b, "、"), strings.Join(a, "、")
		if bs != as {
			ch = append(ch, model.ResourceFieldChange{Field: field, Label: label, Before: bs, After: as})
		}
	}
	addStr("name", "资源名称", before.Name, after.Name)
	addStr("size", "文件大小", before.Size, after.Size)
	addStr("model_name", "AI 模型", before.ModelName, after.ModelName)
	addStr("storage", "存储方式", before.Storage, after.Storage)
	addStr("note", "备注", before.Note, after.Note)
	addArr("language", "语言", before.Language, after.Language)
	addArr("platform", "平台", before.Platform, after.Platform)
	addArr("type", "类型", before.Type, after.Type)
	if before.Code != after.Code ||
		before.Password != after.Password ||
		before.Content != after.Content ||
		before.S3Key != after.S3Key {
		ch = append(ch, model.ResourceFieldChange{
			Field:  "download",
			Label:  "下载文件 / 链接 / 提取码 / 密码",
			Before: "",
			After:  "已更新",
		})
	}
	return ch
}

func (s *PatchService) GetResourceRevisions(resourceID, page, limit int) ([]model.PatchResourceRevision, int64, error) {
	return s.repo.GetResourceRevisions(resourceID, (page-1)*limit, limit)
}
