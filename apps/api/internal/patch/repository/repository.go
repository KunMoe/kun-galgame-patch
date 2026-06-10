package repository

import (
	"fmt"
	"time"

	"kun-galgame-patch-api/internal/patch/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PatchRepository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *PatchRepository {
	return &PatchRepository{db: db}
}

// ===== Patch CRUD =====

func (r *PatchRepository) CreatePatch(patch *model.Patch) error {
	return r.db.Create(patch).Error
}

func (r *PatchRepository) GetPatchByID(id int) (*model.Patch, error) {
	var patch model.Patch
	err := r.db.First(&patch, id).Error
	return &patch, err
}

// GetPatchesByIDs fetches multiple patch rows in one query, preserving the
// caller-supplied id order so the enricher's downstream join keeps galgame
// ordering intact. Empty id slice returns nil without hitting the DB.
//
// Used by WikiTaxonomyDetailProxy to attach moyu-side counts/dates to Wiki's
// tag/official galgame listings — Wiki only knows metadata (name / banner /
// content_limit), the per-patch stats live here.
func (r *PatchRepository) GetPatchesByIDs(ids []int) ([]model.Patch, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var rows []model.Patch
	if err := r.db.Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, err
	}
	// Reorder to match `ids` so the response preserves Wiki's intended order
	// (Wiki may sort by relevance / popularity / chronology — we don't want
	// to scramble that during enrichment).
	byID := make(map[int]model.Patch, len(rows))
	for _, p := range rows {
		byID[p.ID] = p
	}
	ordered := make([]model.Patch, 0, len(rows))
	for _, id := range ids {
		if p, ok := byID[id]; ok {
			ordered = append(ordered, p)
		}
	}
	return ordered, nil
}

func (r *PatchRepository) GetPatchDetail(id int) (*model.Patch, error) {
	var patch model.Patch
	err := r.db.First(&patch, id).Error
	return &patch, err
}

func (r *PatchRepository) UpdatePatch(patch *model.Patch) error {
	return r.db.Save(patch).Error
}

func (r *PatchRepository) DeletePatch(id int) error {
	// user_message has NO FK to patch / patch_resource, so the DB CASCADE that
	// wipes the owned rows leaves notification rows dangling — their links
	// (/patch/:id/resource and each resource's /resource/:rid) would then 404.
	// Delete them in the SAME tx, BEFORE the patch (and its CASCADE'd resources)
	// go away, so the resource ids are still resolvable. (See migration 019 for
	// the one-time cleanup of pre-existing dangling rows.)
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(
			`DELETE FROM user_message
			 WHERE link = ?
			    OR link IN (SELECT '/resource/' || id FROM patch_resource WHERE galgame_id = ?)`,
			fmt.Sprintf("/patch/%d/resource", id), id,
		).Error; err != nil {
			return err
		}
		return tx.Delete(&model.Patch{}, id).Error
	})
}

// GetPatchResourceS3Keys returns every non-empty s3_key on patch_resource
// rows owned by the patch. Used by DeletePatch right before the row goes
// away — we need the keys in hand BEFORE the FK CASCADE wipes the rows,
// because once the rows are gone we have no way to enumerate which B2
// objects to clean up. storage='s3' guard skips user-link rows.
func (r *PatchRepository) GetPatchResourceS3Keys(patchID int) ([]string, error) {
	var keys []string
	err := r.db.Model(&model.PatchResource{}).
		Where("galgame_id = ? AND storage = ? AND s3_key <> ''", patchID, "s3").
		Pluck("s3_key", &keys).Error
	return keys, err
}

// GetPatchResourceFileHistoryS3Keys returns every non-empty old_s3_key on
// patch_resource_file_history rows belonging to the patch's resources.
// UpdateResource appends one history row per file substitution preserving
// the *previous* s3_key; without snapshotting these before DeletePatch the
// FK CASCADE wipes both patch_resource AND history rows, stranding the B2
// objects pointed to by history.old_s3_key.
//
// Disjoint from GetPatchResourceS3Keys: history rows only record keys that
// have *already been replaced* by a newer s3_key, so live patch_resource
// rows and history rows never reference the same object.
func (r *PatchRepository) GetPatchResourceFileHistoryS3Keys(patchID int) ([]string, error) {
	var keys []string
	err := r.db.Table("patch_resource_file_history AS h").
		Joins("JOIN patch_resource AS r ON r.id = h.resource_id").
		Where("r.galgame_id = ? AND h.old_storage = ? AND h.old_s3_key <> ''", patchID, "s3").
		Pluck("h.old_s3_key", &keys).Error
	return keys, err
}

// GetResourceFileHistoryS3Keys returns the history's old_s3_keys for a single
// resource. Used by DeleteResource for the same reason as the patch-scoped
// helper above.
func (r *PatchRepository) GetResourceFileHistoryS3Keys(resourceID int) ([]string, error) {
	var keys []string
	err := r.db.Model(&model.PatchResourceFileHistory{}).
		Where("resource_id = ? AND old_storage = ? AND old_s3_key <> ''", resourceID, "s3").
		Pluck("old_s3_key", &keys).Error
	return keys, err
}

func (r *PatchRepository) FindPatchByVndbID(vndbID string) (*model.Patch, error) {
	var patch model.Patch
	err := r.db.Where("vndb_id = ?", vndbID).First(&patch).Error
	return &patch, err
}

func (r *PatchRepository) IncrementView(id int) error {
	return r.db.Model(&model.Patch{}).Where("id = ?", id).
		UpdateColumn("view", gorm.Expr("view + 1")).Error
}

// includeEmpty=false hides games with no patch resources (the "显示无补丁资源的
// 游戏" toggle, default off) so "随机游戏" never lands on a patch-less game.
func (r *PatchRepository) GetRandomPatchID(includeEmpty bool) (int, error) {
	var id int
	q := r.db.Model(&model.Patch{}).Select("id")
	if !includeEmpty {
		q = q.Where("resource_count > 0")
	}
	err := q.Order("RANDOM()").Limit(1).Scan(&id).Error
	return id, err
}

// GetRandomPatchIDs returns up to n random patch ids. Used by the random-patch
// endpoint so the service layer can ask wiki to filter the candidate set by
// content_limit before picking one — a single RANDOM() pick has no way to
// "retry" if it lands on a NSFW row under a sfw caller. includeEmpty=false also
// drops patch-less games up front (see GetRandomPatchID).
func (r *PatchRepository) GetRandomPatchIDs(n int, includeEmpty bool) ([]int, error) {
	if n <= 0 {
		return nil, nil
	}
	var ids []int
	q := r.db.Model(&model.Patch{}).Select("id")
	if !includeEmpty {
		q = q.Where("resource_count > 0")
	}
	err := q.Order("RANDOM()").Limit(n).Scan(&ids).Error
	return ids, err
}

// NOTE: ReplaceAliases is deprecated per D12 (2026-04-21). Aliases are owned by Wiki /galgame/:gid/aliases.

// ===== Comments =====

func (r *PatchRepository) GetComments(patchID, offset, limit int) ([]model.PatchComment, int64, error) {
	var comments []model.PatchComment
	var total int64

	// Independent statements for Count vs Find — see gorm v2 reuse footgun
	// in message/repository.go GetMessages.
	// status = 0 → only APPROVED comments are public; pending (status=1) ones
	// stay hidden until an admin approves (comment-verify). Applied to both the
	// top-level query and the Replies preload so a pending reply is hidden too.
	base := r.db.Model(&model.PatchComment{}).
		Where("galgame_id = ? AND parent_id IS NULL AND status = 0", patchID)
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := base.Session(&gorm.Session{}).Order("created DESC, id DESC").Offset(offset).Limit(limit).
		Preload("Replies", func(db *gorm.DB) *gorm.DB {
			return db.Where("status = 0").Order("created ASC, id ASC")
		}).
		Find(&comments).Error

	return comments, total, err
}

func (r *PatchRepository) CreateComment(comment *model.PatchComment) error {
	return r.db.Create(comment).Error
}

// UpdateCommentStatus sets a comment's moderation status (0=approved,
// 1=pending). Used by PatchService.ApproveComment.
func (r *PatchRepository) UpdateCommentStatus(commentID, status int) error {
	return r.db.Model(&model.PatchComment{}).Where("id = ?", commentID).
		Update("status", status).Error
}

func (r *PatchRepository) GetCommentByID(id int) (*model.PatchComment, error) {
	var comment model.PatchComment
	err := r.db.First(&comment, id).Error
	return &comment, err
}

// CountRootCommentsBefore returns how many APPROVED root comments of the patch
// sort BEFORE the given root under the list order (created DESC, id DESC).
// (created, id) > (created, id) row-comparison reproduces "earlier in a DESC
// sort". Used by LocateComment to compute which page a comment lands on.
func (r *PatchRepository) CountRootCommentsBefore(galgameID int, created time.Time, id int) (int64, error) {
	var n int64
	err := r.db.Model(&model.PatchComment{}).
		Where("galgame_id = ? AND parent_id IS NULL AND status = 0", galgameID).
		Where("(created, id) > (?, ?)", created, id).
		Count(&n).Error
	return n, err
}

func (r *PatchRepository) UpdateComment(comment *model.PatchComment) error {
	return r.db.Save(comment).Error
}

func (r *PatchRepository) DeleteComment(id int) error {
	return r.db.Delete(&model.PatchComment{}, id).Error
}

// CountCommentAndReplies counts a comment + its direct replies, restricted to
// APPROVED rows (status = 0). DeleteComment uses this to decrement
// patch.comment_count, and only approved comments were ever added to that
// count (pending ones are deferred to approval) — so a pending comment being
// deleted must not subtract from it.
func (r *PatchRepository) CountCommentAndReplies(commentID int) (int64, error) {
	var count int64
	r.db.Model(&model.PatchComment{}).
		Where("(id = ? OR parent_id = ?) AND status = 0", commentID, commentID).
		Count(&count)
	return count, nil
}

func (r *PatchRepository) GetCommentMarkdown(commentID int) (string, error) {
	var content string
	err := r.db.Model(&model.PatchComment{}).Where("id = ?", commentID).Pluck("content", &content).Error
	return content, err
}

// GetCommentPatchID returns the comment's owning patch.id — used by handlers
// that need to NSFW-gate the comment's content (markdown view, etc.). Returns
// 0 + ErrRecordNotFound when the comment doesn't exist.
func (r *PatchRepository) GetCommentPatchID(commentID int) (int, error) {
	var patchID int
	err := r.db.Model(&model.PatchComment{}).Where("id = ?", commentID).Pluck("galgame_id", &patchID).Error
	if err != nil {
		return 0, err
	}
	if patchID == 0 {
		return 0, gorm.ErrRecordNotFound
	}
	return patchID, nil
}

// ===== Comment Likes =====

func (r *PatchRepository) FindCommentLike(userID, commentID int) (*model.UserPatchCommentLikeRelation, error) {
	var rel model.UserPatchCommentLikeRelation
	err := r.db.Where("user_id = ? AND comment_id = ?", userID, commentID).First(&rel).Error
	return &rel, err
}

func (r *PatchRepository) CreateCommentLike(rel *model.UserPatchCommentLikeRelation) error {
	return r.db.Create(rel).Error
}

func (r *PatchRepository) DeleteCommentLike(id int) error {
	return r.db.Delete(&model.UserPatchCommentLikeRelation{}, id).Error
}

// ===== Resources =====

func (r *PatchRepository) GetResources(patchID int) ([]model.PatchResource, error) {
	var resources []model.PatchResource
	err := r.db.Where("galgame_id = ?", patchID).
		Order("created DESC, id DESC").
		Find(&resources).Error
	return resources, err
}

func (r *PatchRepository) CreateResource(resource *model.PatchResource) error {
	return r.db.Create(resource).Error
}

func (r *PatchRepository) GetResourceByID(id int) (*model.PatchResource, error) {
	var resource model.PatchResource
	err := r.db.First(&resource, id).Error
	return &resource, err
}

// NOTE: the former PatchRepository.UpdateResource(resource) helper was
// removed (MOYU-PR5 / M3). All resource updates now go through
// PatchService.UpdateResource which wraps the same Save inside a transaction
// that ALSO writes patch_resource_file_history on file-substantive change.
// Calling tx.Save directly via the repo would silently bypass the audit
// trail — keep the entry point at the service layer.

func (r *PatchRepository) DeleteResource(id int) error {
	// Drop any notification linking to this resource's detail page in the same
	// tx — user_message has no FK to cascade, so a deleted resource would
	// otherwise leave a dangling /resource/:id link. (See migration 019.)
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(
			"DELETE FROM user_message WHERE link = ?",
			fmt.Sprintf("/resource/%d", id),
		).Error; err != nil {
			return err
		}
		return tx.Delete(&model.PatchResource{}, id).Error
	})
}

func (r *PatchRepository) IncrementResourceDownload(resourceID, patchID int) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.PatchResource{}).Where("id = ?", resourceID).
			UpdateColumn("download", gorm.Expr("download + 1")).Error; err != nil {
			return err
		}
		return tx.Model(&model.Patch{}).Where("id = ?", patchID).
			UpdateColumn("download", gorm.Expr("download + 1")).Error
	})
}

func (r *PatchRepository) ToggleResourceStatus(resourceID int) error {
	return r.db.Model(&model.PatchResource{}).Where("id = ?", resourceID).
		UpdateColumn("status", gorm.Expr("CASE WHEN status = 0 THEN 1 ELSE 0 END")).Error
}

// ===== Resource Likes =====

func (r *PatchRepository) FindResourceLike(userID, resourceID int) (*model.UserPatchResourceLikeRelation, error) {
	var rel model.UserPatchResourceLikeRelation
	err := r.db.Where("user_id = ? AND resource_id = ?", userID, resourceID).First(&rel).Error
	return &rel, err
}

func (r *PatchRepository) CreateResourceLike(rel *model.UserPatchResourceLikeRelation) error {
	return r.db.Create(rel).Error
}

func (r *PatchRepository) DeleteResourceLike(id int) error {
	return r.db.Delete(&model.UserPatchResourceLikeRelation{}, id).Error
}

// ===== Resource Favorites (per-resource subscription) =====

func (r *PatchRepository) FindResourceFavorite(userID, resourceID int) (*model.UserPatchResourceFavoriteRelation, error) {
	var rel model.UserPatchResourceFavoriteRelation
	err := r.db.Where("user_id = ? AND resource_id = ?", userID, resourceID).First(&rel).Error
	return &rel, err
}

func (r *PatchRepository) CreateResourceFavorite(rel *model.UserPatchResourceFavoriteRelation) error {
	return r.db.Create(rel).Error
}

func (r *PatchRepository) DeleteResourceFavorite(id int) error {
	return r.db.Delete(&model.UserPatchResourceFavoriteRelation{}, id).Error
}

// ===== Favorites =====

func (r *PatchRepository) FindFavorite(userID, patchID int) (*model.UserPatchFavoriteRelation, error) {
	var rel model.UserPatchFavoriteRelation
	err := r.db.Where("user_id = ? AND galgame_id = ?", userID, patchID).First(&rel).Error
	return &rel, err
}

func (r *PatchRepository) CreateFavorite(rel *model.UserPatchFavoriteRelation) error {
	return r.db.Create(rel).Error
}

func (r *PatchRepository) DeleteFavorite(id int) error {
	return r.db.Delete(&model.UserPatchFavoriteRelation{}, id).Error
}

// ===== Contributors =====

// GetContributorIDs returns the user_ids of every contributor on a patch.
// The handler layer batches these via OAuth /users/batch (pkg/userclient)
// to assemble the wire shape; we no longer SELECT name/avatar from the local
// user table because those columns are owned by OAuth (migration 005).
func (r *PatchRepository) GetContributorIDs(patchID int) ([]int, error) {
	var ids []int
	err := r.db.Table("user_patch_contribute_relation").
		Where("galgame_id = ?", patchID).
		Pluck("user_id", &ids).Error
	return ids, err
}

func (r *PatchRepository) EnsureContributor(userID, patchID int) error {
	rel := model.UserPatchContributeRelation{UserID: userID, GalgameID: patchID}
	result := r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&rel)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		return r.db.Model(&model.Patch{}).Where("id = ?", patchID).
			UpdateColumn("contribute_count", gorm.Expr("contribute_count + 1")).Error
	}
	return nil
}

// ===== Aggregate Updates =====

func (r *PatchRepository) RecalculatePatchAggregates(patchID int) error {
	var resources []model.PatchResource
	r.db.Where("galgame_id = ?", patchID).Select("type", "language", "platform").Find(&resources)

	typeSet := make(map[string]bool)
	langSet := make(map[string]bool)
	platSet := make(map[string]bool)
	for _, res := range resources {
		for _, t := range res.Type {
			typeSet[t] = true
		}
		for _, l := range res.Language {
			langSet[l] = true
		}
		for _, p := range res.Platform {
			platSet[p] = true
		}
	}

	return r.db.Model(&model.Patch{}).Where("id = ?", patchID).Updates(map[string]any{
		"type":     model.JSONArray(setToSlice(typeSet)),
		"language": model.JSONArray(setToSlice(langSet)),
		"platform": model.JSONArray(setToSlice(platSet)),
	}).Error
}

func (r *PatchRepository) UpdateCount(patchID int, field string, delta int) error {
	expr := fmt.Sprintf("%s + %d", field, delta)
	if delta < 0 {
		expr = fmt.Sprintf("GREATEST(%s + %d, 0)", field, delta)
	}
	return r.db.Model(&model.Patch{}).Where("id = ?", patchID).
		UpdateColumn(field, gorm.Expr(expr)).Error
}

// moemoepoint is no longer mutated locally — OAuth is the unified source of
// truth (see pkg/moemoepoint). The local user.moemoepoint column is a read-cache
// updated from each OAuth adjust response by moemoepoint.Awarder.

// ===== Liked resource IDs for a user =====

func (r *PatchRepository) GetLikedResourceIDs(userID int, resourceIDs []int) ([]int, error) {
	var ids []int
	err := r.db.Model(&model.UserPatchResourceLikeRelation{}).
		Where("user_id = ? AND resource_id IN ?", userID, resourceIDs).
		Pluck("resource_id", &ids).Error
	return ids, err
}

// GetFavoritedResourceIDs returns the subset of resourceIDs the user has
// subscribed to (收藏资源) — mirrors GetLikedResourceIDs, used to stamp
// is_favorite on the resource list.
func (r *PatchRepository) GetFavoritedResourceIDs(userID int, resourceIDs []int) ([]int, error) {
	var ids []int
	err := r.db.Model(&model.UserPatchResourceFavoriteRelation{}).
		Where("user_id = ? AND resource_id IN ?", userID, resourceIDs).
		Pluck("resource_id", &ids).Error
	return ids, err
}

func (r *PatchRepository) GetLikedCommentIDs(userID int, commentIDs []int) ([]int, error) {
	var ids []int
	err := r.db.Model(&model.UserPatchCommentLikeRelation{}).
		Where("user_id = ? AND comment_id IN ?", userID, commentIDs).
		Pluck("comment_id", &ids).Error
	return ids, err
}

func setToSlice(s map[string]bool) []string {
	result := make([]string, 0, len(s))
	for k := range s {
		result = append(result, k)
	}
	return result
}

// GetResourceFileHistory returns the file-replacement audit rows for one
// resource, newest first. Public surface (the privacy-safe projection that
// drops old_s3_key / old_content happens in the service). Mirrors
// AdminRepository.GetResourceFileHistory so the public history endpoint does
// not reach across modules into the admin repo.
func (r *PatchRepository) GetResourceFileHistory(resourceID, offset, limit int) ([]model.PatchResourceFileHistory, int64, error) {
	var rows []model.PatchResourceFileHistory
	var total int64
	base := r.db.Model(&model.PatchResourceFileHistory{}).Where("resource_id = ?", resourceID)
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := base.Session(&gorm.Session{}).
		Order("created_at DESC, id DESC").
		Offset(offset).Limit(limit).
		Find(&rows).Error
	return rows, total, err
}

// GetResourceRevisions returns the per-field edit-diff history for one resource,
// newest first. Public (stored Changes are secret-free). Paginated.
func (r *PatchRepository) GetResourceRevisions(resourceID, offset, limit int) ([]model.PatchResourceRevision, int64, error) {
	var rows []model.PatchResourceRevision
	var total int64
	base := r.db.Model(&model.PatchResourceRevision{}).Where("resource_id = ?", resourceID)
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := base.Session(&gorm.Session{}).
		Order("created_at DESC, id DESC").
		Offset(offset).Limit(limit).
		Find(&rows).Error
	return rows, total, err
}
