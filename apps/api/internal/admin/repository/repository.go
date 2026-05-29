package repository

import (
	"encoding/json"
	stderrors "errors"
	"time"

	adminModel "kun-galgame-patch-api/internal/admin/model"
	authModel "kun-galgame-patch-api/internal/auth/model"
	patchModel "kun-galgame-patch-api/internal/patch/model"
	userModel "kun-galgame-patch-api/internal/user/model"

	"gorm.io/gorm"
)

// ErrUserOwnsPatches is returned by PurgeUser when the target still owns ≥1
// patch and purgeOwnedPatches is false: DELETE FROM "user" would violate the
// patch.user_id ON DELETE RESTRICT FK. The service maps this to a 400 telling
// the admin to enable the force-delete option.
var ErrUserOwnsPatches = stderrors.New("user still owns patches")

type AdminRepository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *AdminRepository {
	return &AdminRepository{db: db}
}

// ===== Comments =====

func (r *AdminRepository) GetComments(search string, offset, limit int) ([]patchModel.PatchComment, int64, error) {
	var comments []patchModel.PatchComment
	var total int64

	// Independent statements for Count vs Find — see gorm v2 reuse footgun
	// documented in message/repository.go GetMessages.
	base := r.db.Model(&patchModel.PatchComment{})
	if search != "" {
		base = base.Where("content ILIKE ?", "%"+search+"%")
	}
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := base.Session(&gorm.Session{}).Order("created DESC, id DESC").Offset(offset).Limit(limit).
		Find(&comments).Error
	return comments, total, err
}

func (r *AdminRepository) UpdateComment(commentID int, content string) error {
	return r.db.Model(&patchModel.PatchComment{}).Where("id = ?", commentID).
		Update("content", content).Error
}

func (r *AdminRepository) DeleteComment(commentID int) error {
	return r.db.Delete(&patchModel.PatchComment{}, commentID).Error
}

// ===== Resources =====

func (r *AdminRepository) GetResources(search string, offset, limit int) ([]patchModel.PatchResource, int64, error) {
	var resources []patchModel.PatchResource
	var total int64

	base := r.db.Model(&patchModel.PatchResource{})
	if search != "" {
		base = base.Where("name ILIKE ? OR content ILIKE ?", "%"+search+"%", "%"+search+"%")
	}
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := base.Session(&gorm.Session{}).Order("created DESC, id DESC").Offset(offset).Limit(limit).
		Find(&resources).Error
	return resources, total, err
}

func (r *AdminRepository) UpdateResource(resourceID int, note string) error {
	return r.db.Model(&patchModel.PatchResource{}).Where("id = ?", resourceID).
		Update("note", note).Error
}

func (r *AdminRepository) DeleteResource(resourceID int) error {
	return r.db.Delete(&patchModel.PatchResource{}, resourceID).Error
}

// GetResourceByID exposes the resource row so the service layer can read
// Storage / S3Key before DELETE (admin needs them to clean up the S3 object,
// see AdminService.DeleteResource). Mirrors PatchRepository.GetResourceByID
// rather than calling across modules so admin keeps its own narrow surface.
func (r *AdminRepository) GetResourceByID(resourceID int) (*patchModel.PatchResource, error) {
	var resource patchModel.PatchResource
	err := r.db.First(&resource, resourceID).Error
	return &resource, err
}

// GetResourceFileHistoryS3Keys mirrors PatchRepository's same-named helper
// (kept here so admin doesn't reach across modules into the patch repo).
// Needed for the same reason: patch_resource_file_history is CASCADE'd when
// the resource is deleted, so the old_s3_key must be snapshotted first or
// the corresponding B2 objects strand.
func (r *AdminRepository) GetResourceFileHistoryS3Keys(resourceID int) ([]string, error) {
	var keys []string
	err := r.db.Model(&patchModel.PatchResourceFileHistory{}).
		Where("resource_id = ? AND old_storage = ? AND old_s3_key <> ''", resourceID, "s3").
		Pluck("old_s3_key", &keys).Error
	return keys, err
}

// User management & creator-application repo methods are gone with the
// migration: identity is owned by OAuth, and the creator role was retired.

// ===== Stats =====

func (r *AdminRepository) GetStats(since time.Time) (newUser, newActive, newGalgame, newResource, newComment int64) {
	r.db.Model(&authModel.User{}).Where("created >= ?", since).Count(&newUser)
	r.db.Model(&authModel.User{}).Where("last_login_time >= ?", since.Format(time.RFC3339)).Count(&newActive)
	r.db.Model(&patchModel.Patch{}).Where("created >= ?", since).Count(&newGalgame)
	r.db.Model(&patchModel.PatchResource{}).Where("created >= ?", since).Count(&newResource)
	r.db.Model(&patchModel.PatchComment{}).Where("created >= ?", since).Count(&newComment)
	return
}

func (r *AdminRepository) GetStatsSum() (userCount, galgameCount, resourceCount, commentCount int64) {
	r.db.Model(&authModel.User{}).Count(&userCount)
	r.db.Model(&patchModel.Patch{}).Count(&galgameCount)
	r.db.Model(&patchModel.PatchResource{}).Count(&resourceCount)
	r.db.Model(&patchModel.PatchComment{}).Count(&commentCount)
	return
}

// ===== Resource file history (MOYU-PR5 / M3) =====

// GetResourceFileHistory returns the audit trail for one resource, newest
// first. Page-based; default limit comes from the caller. Admin-only — exposed
// at GET /api/v1/admin/resource/:id/history.
func (r *AdminRepository) GetResourceFileHistory(
	resourceID, offset, limit int,
) ([]patchModel.PatchResourceFileHistory, int64, error) {
	var rows []patchModel.PatchResourceFileHistory
	var total int64

	base := r.db.Model(&patchModel.PatchResourceFileHistory{}).
		Where("resource_id = ?", resourceID)
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := base.Session(&gorm.Session{}).
		Order("created_at DESC, id DESC").
		Offset(offset).Limit(limit).
		Find(&rows).Error
	return rows, total, err
}

// ===== Admin Logs =====

func (r *AdminRepository) GetLogs(offset, limit int) ([]adminModel.AdminLog, int64, error) {
	var logs []adminModel.AdminLog
	var total int64

	base := r.db.Model(&adminModel.AdminLog{})
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := base.Session(&gorm.Session{}).Order("created DESC, id DESC").Offset(offset).Limit(limit).
		Find(&logs).Error
	return logs, total, err
}

func (r *AdminRepository) CreateLog(adminUID int, logType string, data any) error {
	content, _ := json.Marshal(data)
	log := &adminModel.AdminLog{
		Type:    logType,
		Content: string(content),
		UserID:  adminUID,
	}
	return r.db.Create(log).Error
}

// ===== All Patches (admin browse) =====

// GetAllPatches lists every patch with pagination, optionally filtering by
// substring of vndb_id (game names are owned by Wiki and cannot be searched
// locally; the admin frontend pairs this listing with the patch_id-based
// patch detail link to navigate further).
func (r *AdminRepository) GetAllPatches(search string, offset, limit int) ([]patchModel.Patch, int64, error) {
	var patches []patchModel.Patch
	var total int64

	base := r.db.Model(&patchModel.Patch{})
	if search != "" {
		base = base.Where("vndb_id ILIKE ?", "%"+search+"%")
	}
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := base.Session(&gorm.Session{}).Order("created DESC, id DESC").Offset(offset).Limit(limit).
		Find(&patches).Error
	return patches, total, err
}

// ===== Orphan Patches (D12 cleanup) =====

// orphanCond is the SQL predicate for an "orphan" patch — one whose vndb_id is
// not a well-formed VNDB visual-novel id (`vN`), so it can have no matching
// Wiki galgame (Wiki is keyed by vndb_id). Captures both `pending-N`
// placeholders and malformed ids (e.g. VNDB *release* ids `rN`, stray slashes).
//
// D13 NOTE: the old per-row `galgame_id` sentinel column (== 0 meant "no Wiki
// match") was DROPPED when patch.id became the Wiki galgame id. Orphan-ness is
// no longer materialized in a column, so we derive it from vndb_id shape. The
// "well-formed vndb_id that Wiki nonetheless lacks" case can't be detected
// locally post-D13 (it would need a Wiki scan of all 6k+ patches per request)
// and is intentionally out of scope here.
const orphanCond = "vndb_id !~ '^v[0-9]+$'"

// GetOrphanPatches returns a paginated list of orphan patches (see orphanCond),
// ordered by resource count descending so admins can prioritize "important"
// orphans that already have resources.
func (r *AdminRepository) GetOrphanPatches(offset, limit int) ([]patchModel.Patch, int64, error) {
	var patches []patchModel.Patch
	var total int64
	base := r.db.Model(&patchModel.Patch{}).Where(orphanCond)
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := base.Session(&gorm.Session{}).
		Order("resource_count DESC, comment_count DESC, favorite_count DESC, id ASC").
		Offset(offset).Limit(limit).
		Find(&patches).Error
	return patches, total, err
}

// CountOrphanPatches splits the orphan total into the two locally-knowable
// categories: pending placeholders (`pending-N`) vs. otherwise-malformed
// vndb_ids (not `vN`, not `pending-`).
func (r *AdminRepository) CountOrphanPatches() (pendingCount, badVndbCount int64, err error) {
	if err := r.db.Model(&patchModel.Patch{}).
		Where("vndb_id LIKE 'pending-%'").
		Count(&pendingCount).Error; err != nil {
		return 0, 0, err
	}
	err = r.db.Model(&patchModel.Patch{}).
		Where(orphanCond+" AND vndb_id NOT LIKE 'pending-%'").
		Count(&badVndbCount).Error
	return
}

// ===== User purge (anti-spam) =====

// PurgePreviewCounts is the raw count breakdown for a user purge dry-run. The
// service maps it to dto.UserPurgePreview (and derives CanDeleteUserRow).
type PurgePreviewCounts struct {
	UserExists          bool
	Comments            int64
	Resources           int64
	ResourceS3Files     int64 // own resources: live s3 objects + replaced-file history
	CommentLikes        int64
	ResourceLikes       int64
	Favorites           int64
	Contributes         int64
	Following           int64
	Followers           int64
	ChatMemberships     int64
	ChatMessages        int64
	PrivateMessages     int64
	OwnedPatches        int64
	OwnedPatchResources int64
	OwnedPatchComments  int64
	OwnedPatchS3Files   int64
	// MiscTraces: rows in tables that store a user id WITHOUT a FK to "user"
	// (so the user-row CASCADE can't reach them) — wiki_message_read_state +
	// patch_resource_file_history authored by the user. Purged explicitly.
	MiscTraces int64
}

// ownedPatchIDsSubquery is the `id IN (patches owned by U)` building block,
// reused across the owned-patch collateral counts and the S3-key collection.
func (r *AdminRepository) ownedPatchIDsSubquery(userID int) *gorm.DB {
	return r.db.Model(&patchModel.Patch{}).Select("id").Where("user_id = ?", userID)
}

// PurgePreview returns the count breakdown without mutating anything.
func (r *AdminRepository) PurgePreview(userID int, includeOwnedPatches bool) (*PurgePreviewCounts, error) {
	var c PurgePreviewCounts

	var userCount int64
	if err := r.db.Model(&authModel.User{}).Where("id = ?", userID).Count(&userCount).Error; err != nil {
		return nil, err
	}
	c.UserExists = userCount > 0

	// count runs a scoped COUNT and stores into dst; first error short-circuits.
	var firstErr error
	count := func(dst *int64, q *gorm.DB) {
		if firstErr != nil {
			return
		}
		if err := q.Count(dst).Error; err != nil {
			firstErr = err
		}
	}

	count(&c.Comments, r.db.Model(&patchModel.PatchComment{}).Where("user_id = ?", userID))
	count(&c.Resources, r.db.Model(&patchModel.PatchResource{}).Where("user_id = ?", userID))
	count(&c.CommentLikes, r.db.Model(&patchModel.UserPatchCommentLikeRelation{}).Where("user_id = ?", userID))
	count(&c.ResourceLikes, r.db.Model(&patchModel.UserPatchResourceLikeRelation{}).Where("user_id = ?", userID))
	count(&c.Favorites, r.db.Model(&patchModel.UserPatchFavoriteRelation{}).Where("user_id = ?", userID))
	count(&c.Contributes, r.db.Model(&patchModel.UserPatchContributeRelation{}).Where("user_id = ?", userID))
	count(&c.Following, r.db.Model(&userModel.UserFollowRelation{}).Where("follower_id = ?", userID))
	count(&c.Followers, r.db.Model(&userModel.UserFollowRelation{}).Where("following_id = ?", userID))
	count(&c.ChatMemberships, r.db.Table("chat_member").Where("user_id = ?", userID))
	count(&c.ChatMessages, r.db.Table("chat_message").Where("sender_id = ?", userID))
	count(&c.PrivateMessages, r.db.Table("user_message").Where("sender_id = ? OR recipient_id = ?", userID, userID))
	count(&c.OwnedPatches, r.db.Model(&patchModel.Patch{}).Where("user_id = ?", userID))

	// FK-less per-user tables (the user-row CASCADE can't reach these).
	var readStates, fileHistory int64
	count(&readStates, r.db.Table("wiki_message_read_state").Where("user_id = ?", userID))
	count(&fileHistory, r.db.Table("patch_resource_file_history").Where("actor_id = ?", userID))
	c.MiscTraces = readStates + fileHistory

	// own-resource S3 objects = live (storage='s3') + replaced-file history.
	var ownLiveS3, ownHistS3 int64
	count(&ownLiveS3, r.db.Model(&patchModel.PatchResource{}).Where("user_id = ? AND storage = ? AND s3_key <> ''", userID, "s3"))
	count(&ownHistS3, r.db.Table("patch_resource_file_history AS h").
		Joins("JOIN patch_resource AS r ON r.id = h.resource_id").
		Where("r.user_id = ? AND h.old_storage = ? AND h.old_s3_key <> ''", userID, "s3"))
	c.ResourceS3Files = ownLiveS3 + ownHistS3

	if includeOwnedPatches {
		count(&c.OwnedPatchResources, r.db.Model(&patchModel.PatchResource{}).Where("galgame_id IN (?)", r.ownedPatchIDsSubquery(userID)))
		count(&c.OwnedPatchComments, r.db.Model(&patchModel.PatchComment{}).Where("galgame_id IN (?)", r.ownedPatchIDsSubquery(userID)))
		var opLiveS3, opHistS3 int64
		count(&opLiveS3, r.db.Model(&patchModel.PatchResource{}).
			Where("galgame_id IN (?) AND storage = ? AND s3_key <> ''", r.ownedPatchIDsSubquery(userID), "s3"))
		count(&opHistS3, r.db.Table("patch_resource_file_history AS h").
			Joins("JOIN patch_resource AS r ON r.id = h.resource_id").
			Where("r.galgame_id IN (?) AND h.old_storage = ? AND h.old_s3_key <> ''", r.ownedPatchIDsSubquery(userID), "s3"))
		c.OwnedPatchS3Files = opLiveS3 + opHistS3
	}

	if firstErr != nil {
		return nil, firstErr
	}
	return &c, nil
}

// CollectUserS3Keys returns the deduped set of moyu-bucket S3 object keys that
// the purge must delete (PG CASCADE removes the rows but never the objects):
// the user's own resources' live s3_key + their replaced-file history, plus —
// when force-deleting owned patches — every resource key (any author) under
// those patches + their history. Mirrors PatchRepository's S3-key helpers.
func (r *AdminRepository) CollectUserS3Keys(userID int, includeOwnedPatches bool) ([]string, error) {
	seen := make(map[string]struct{})
	add := func(keys []string) {
		for _, k := range keys {
			if k != "" {
				seen[k] = struct{}{}
			}
		}
	}

	pluck := func(q *gorm.DB, col string) ([]string, error) {
		var keys []string
		err := q.Pluck(col, &keys).Error
		return keys, err
	}

	// own resources — live
	if keys, err := pluck(r.db.Model(&patchModel.PatchResource{}).
		Where("user_id = ? AND storage = ? AND s3_key <> ''", userID, "s3"), "s3_key"); err != nil {
		return nil, err
	} else {
		add(keys)
	}
	// own resources — replaced-file history
	if keys, err := pluck(r.db.Table("patch_resource_file_history AS h").
		Joins("JOIN patch_resource AS r ON r.id = h.resource_id").
		Where("r.user_id = ? AND h.old_storage = ? AND h.old_s3_key <> ''", userID, "s3"), "h.old_s3_key"); err != nil {
		return nil, err
	} else {
		add(keys)
	}

	// files this user replaced as actor (patch_resource_file_history.actor_id
	// has no FK to "user"): covers their own resources AND — if they were ever
	// privileged — replacements on OTHER users' resources whose history rows
	// won't CASCADE with the user. Dedup-safe with the blocks above.
	if keys, err := pluck(r.db.Table("patch_resource_file_history").
		Where("actor_id = ? AND old_storage = ? AND old_s3_key <> ''", userID, "s3"), "old_s3_key"); err != nil {
		return nil, err
	} else {
		add(keys)
	}

	if includeOwnedPatches {
		// every resource under owned patches (any author) — live
		if keys, err := pluck(r.db.Model(&patchModel.PatchResource{}).
			Where("galgame_id IN (?) AND storage = ? AND s3_key <> ''", r.ownedPatchIDsSubquery(userID), "s3"), "s3_key"); err != nil {
			return nil, err
		} else {
			add(keys)
		}
		// ...and their replaced-file history
		if keys, err := pluck(r.db.Table("patch_resource_file_history AS h").
			Joins("JOIN patch_resource AS r ON r.id = h.resource_id").
			Where("r.galgame_id IN (?) AND h.old_storage = ? AND h.old_s3_key <> ''", r.ownedPatchIDsSubquery(userID), "s3"), "h.old_s3_key"); err != nil {
			return nil, err
		} else {
			add(keys)
		}
	}

	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	return out, nil
}

// PurgeUser wipes every moyu-side trace of a user in one transaction, then the
// service does best-effort S3 cleanup with the keys from CollectUserS3Keys.
//
// CASCADE from the user row removes all user_id=U rows automatically; this
// method only does what CASCADE can't: clear the two RESTRICT FKs first
// (follows + optionally owned patches) and recompute the denormalized counters
// left dangling on SURVIVING content. Affected-id sets are snapshotted before
// the deletes so the post-cascade recompute targets the right rows.
func (r *AdminRepository) PurgeUser(userID int, purgeOwnedPatches bool) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var ownedPatches int64
		if err := tx.Model(&patchModel.Patch{}).Where("user_id = ?", userID).Count(&ownedPatches).Error; err != nil {
			return err
		}
		if ownedPatches > 0 && !purgeOwnedPatches {
			return ErrUserOwnsPatches
		}

		// Snapshot affected ids for the counter recompute BEFORE deleting.
		distinctInts := func(table, col, where string, args ...any) ([]int, error) {
			var ids []int
			err := tx.Table(table).Where(where, args...).Distinct().Pluck(col, &ids).Error
			return ids, err
		}
		pc, err := distinctInts("patch_comment", "galgame_id", "user_id = ?", userID)
		if err != nil {
			return err
		}
		pr, err := distinctInts("patch_resource", "galgame_id", "user_id = ?", userID)
		if err != nil {
			return err
		}
		pf, err := distinctInts("user_patch_favorite_relation", "galgame_id", "user_id = ?", userID)
		if err != nil {
			return err
		}
		pco, err := distinctInts("user_patch_contribute_relation", "galgame_id", "user_id = ?", userID)
		if err != nil {
			return err
		}
		affectedPatchIDs := unionInts(pc, pr, pf, pco)

		likedCommentIDs, err := distinctInts("user_patch_comment_like_relation", "comment_id", "user_id = ?", userID)
		if err != nil {
			return err
		}
		likedResourceIDs, err := distinctInts("user_patch_resource_like_relation", "resource_id", "user_id = ?", userID)
		if err != nil {
			return err
		}
		followingPeers, err := distinctInts("user_follow_relation", "following_id", "follower_id = ?", userID)
		if err != nil {
			return err
		}
		followerPeers, err := distinctInts("user_follow_relation", "follower_id", "following_id = ?", userID)
		if err != nil {
			return err
		}
		peers := unionInts(followingPeers, followerPeers)

		// 1. Delete follow rows (clears the being-followed RESTRICT on following_id).
		if err := tx.Where("follower_id = ? OR following_id = ?", userID, userID).
			Delete(&userModel.UserFollowRelation{}).Error; err != nil {
			return err
		}

		// 1b. FK-less per-user tables — the user-row CASCADE below can't reach
		//     these, so clear them explicitly or they'd dangle:
		//       - wiki_message_read_state: per-user read marker (PK user_id)
		//       - patch_resource_file_history.actor_id: file-replacement audit
		//         authored by U (own-resource rows also CASCADE via resource_id;
		//         actor rows on OTHER users' resources would otherwise survive).
		if err := tx.Exec(`DELETE FROM wiki_message_read_state WHERE user_id = ?`, userID).Error; err != nil {
			return err
		}
		if err := tx.Where("actor_id = ?", userID).
			Delete(&patchModel.PatchResourceFileHistory{}).Error; err != nil {
			return err
		}

		// 2. Force-delete owned patches if requested (CASCADEs their resources,
		//    comments, links, fav/contribute relations; clears patch RESTRICT).
		if purgeOwnedPatches && ownedPatches > 0 {
			if err := tx.Where("user_id = ?", userID).Delete(&patchModel.Patch{}).Error; err != nil {
				return err
			}
		}

		// 3. Delete the user row → CASCADE removes every remaining user_id=U row
		//    (comments, resources, chat, messages, likes, favorites, contributes,
		//    follower-side follows, admin_log).
		if err := tx.Delete(&authModel.User{}, userID).Error; err != nil {
			return err
		}

		// 4. Recompute denormalized counters on survivors. Force-deleted patches
		//    won't match the IN list, so they're skipped harmlessly.
		if len(affectedPatchIDs) > 0 {
			if err := tx.Exec(`UPDATE patch SET
				comment_count    = (SELECT COUNT(*) FROM patch_comment WHERE patch_comment.galgame_id = patch.id),
				resource_count   = (SELECT COUNT(*) FROM patch_resource WHERE patch_resource.galgame_id = patch.id),
				favorite_count   = (SELECT COUNT(*) FROM user_patch_favorite_relation WHERE user_patch_favorite_relation.galgame_id = patch.id),
				contribute_count = (SELECT COUNT(*) FROM user_patch_contribute_relation WHERE user_patch_contribute_relation.galgame_id = patch.id)
				WHERE id IN ?`, affectedPatchIDs).Error; err != nil {
				return err
			}
		}
		if len(likedCommentIDs) > 0 {
			if err := tx.Exec(`UPDATE patch_comment SET like_count =
				(SELECT COUNT(*) FROM user_patch_comment_like_relation WHERE user_patch_comment_like_relation.comment_id = patch_comment.id)
				WHERE id IN ?`, likedCommentIDs).Error; err != nil {
				return err
			}
		}
		if len(likedResourceIDs) > 0 {
			if err := tx.Exec(`UPDATE patch_resource SET like_count =
				(SELECT COUNT(*) FROM user_patch_resource_like_relation WHERE user_patch_resource_like_relation.resource_id = patch_resource.id)
				WHERE id IN ?`, likedResourceIDs).Error; err != nil {
				return err
			}
		}
		if len(peers) > 0 {
			if err := tx.Exec(`UPDATE "user" SET
				follower_count  = (SELECT COUNT(*) FROM user_follow_relation WHERE user_follow_relation.following_id = "user".id),
				following_count = (SELECT COUNT(*) FROM user_follow_relation WHERE user_follow_relation.follower_id = "user".id)
				WHERE id IN ?`, peers).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// unionInts merges several int slices into one deduped slice (order unimportant
// — used only for `WHERE id IN (...)` recompute targets).
func unionInts(slices ...[]int) []int {
	seen := make(map[int]struct{})
	for _, s := range slices {
		for _, v := range s {
			seen[v] = struct{}{}
		}
	}
	out := make([]int, 0, len(seen))
	for v := range seen {
		out = append(out, v)
	}
	return out
}
