package service

import (
	"context"
	"fmt"
	"time"

	"kun-galgame-patch-api/internal/admin/dto"
	"kun-galgame-patch-api/internal/admin/repository"
	"kun-galgame-patch-api/internal/infrastructure/markdown"
	patchModel "kun-galgame-patch-api/internal/patch/model"

	"github.com/redis/go-redis/v9"
)

type AdminService struct {
	repo *repository.AdminRepository
	rdb  *redis.Client
}

func New(repo *repository.AdminRepository, rdb *redis.Client) *AdminService {
	return &AdminService{repo: repo, rdb: rdb}
}

// ===== Comments =====

func (s *AdminService) GetComments(search string, page, limit int) ([]patchModel.PatchComment, int64, error) {
	comments, total, err := s.repo.GetComments(search, (page-1)*limit, limit)
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
	if err := s.repo.DeleteResource(resourceID); err != nil {
		return err
	}
	s.repo.CreateLog(adminUID, "deleteResource", map[string]any{"resource_id": resourceID})
	return nil
}

// ===== Users =====

func (s *AdminService) GetUsers(search string, page, limit int) ([]repository.AdminUserView, int64, error) {
	return s.repo.GetUsers(search, (page-1)*limit, limit)
}

func (s *AdminService) UpdateUser(uid int, fields map[string]any, adminUID, adminRole int) error {
	if role, ok := fields["role"]; ok {
		if r, ok := role.(int); ok && r >= 3 && adminRole < 4 {
			return fmt.Errorf("only super admin can set admin roles")
		}
	}
	if err := s.repo.UpdateUser(uid, fields); err != nil {
		return err
	}
	s.repo.CreateLog(adminUID, "updateUser", map[string]any{"uid": uid, "fields": fields})
	return nil
}

func (s *AdminService) DeleteUser(uid, adminUID int) error {
	user, err := s.repo.GetUserByID(uid)
	if err != nil {
		return fmt.Errorf("user not found")
	}

	// Cache ban info in Redis
	ctx := context.Background()
	if user.Email != "" {
		s.rdb.Set(ctx, "ban:email:"+user.Email, 1, 0)
	}
	if user.IP != "" {
		s.rdb.Set(ctx, "ban:ip:"+user.IP, 1, 0)
	}

	if err := s.repo.DeleteUser(uid); err != nil {
		return err
	}
	s.repo.CreateLog(adminUID, "deleteUser", map[string]any{"uid": uid, "email": user.Email})
	return nil
}

// ===== All Patches =====

func (s *AdminService) GetAllPatches(search string, page, limit int) ([]patchModel.Patch, int64, error) {
	return s.repo.GetAllPatches(search, (page-1)*limit, limit)
}

// ===== Creator Applications =====

func (s *AdminService) GetCreatorApplications(page, limit int) ([]repository.CreatorApplicationItem, int64, error) {
	return s.repo.GetCreatorApplications((page-1)*limit, limit)
}

func (s *AdminService) ApproveCreator(messageID, uid, adminUID int) error {
	if err := s.repo.ApproveCreator(messageID, uid); err != nil {
		return err
	}
	s.repo.CreateLog(adminUID, "approveCreator", map[string]any{"message_id": messageID, "uid": uid})
	return nil
}

func (s *AdminService) DeclineCreator(messageID, adminUID int, reason string) error {
	if err := s.repo.DeclineCreator(messageID); err != nil {
		return err
	}
	s.repo.CreateLog(adminUID, "declineCreator", map[string]any{"message_id": messageID, "reason": reason})
	return nil
}

// ===== Settings =====

func (s *AdminService) GetSetting(key string) bool {
	val, err := s.rdb.Get(context.Background(), key).Result()
	return err == nil && val == "true"
}

func (s *AdminService) SetSetting(key string, enabled bool) {
	val := "false"
	if enabled {
		val = "true"
	}
	s.rdb.Set(context.Background(), key, val, 0)
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

func (s *AdminService) GetLogs(page, limit int) (any, int64, error) {
	return s.repo.GetLogs((page-1)*limit, limit)
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
