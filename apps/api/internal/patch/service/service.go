package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"strings"
	"time"

	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/internal/infrastructure/markdown"
	"kun-galgame-patch-api/internal/infrastructure/storage"
	"kun-galgame-patch-api/internal/patch/dto"
	"kun-galgame-patch-api/internal/patch/model"
	"kun-galgame-patch-api/internal/patch/repository"
	settingService "kun-galgame-patch-api/internal/setting/service"
	"kun-galgame-patch-api/pkg/moemoepoint"
	"kun-galgame-patch-api/pkg/userclient"
	"kun-galgame-patch-api/pkg/utils"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrWikiGalgameMissing is returned by CreatePatch when the supplied
// vndb_id has no corresponding row on the Galgame Wiki yet. The handler
// translates this into the typed AppError so the frontend can pick it up
// via code = 44001 and render a "前往 Wiki 创建" CTA.
var ErrWikiGalgameMissing = errors.New("wiki galgame missing for vndb_id")

type PatchService struct {
	repo    *repository.PatchRepository
	setting *settingService.Service
	db      *gorm.DB
	s3      *storage.S3Client
	wiki    *galgameClient.Client
	users   *userclient.Client
	mp      *moemoepoint.Awarder
}

func New(repo *repository.PatchRepository, setting *settingService.Service, db *gorm.DB, s3 *storage.S3Client, wiki *galgameClient.Client, users *userclient.Client, mp *moemoepoint.Awarder) *PatchService {
	return &PatchService{repo: repo, setting: setting, db: db, s3: s3, wiki: wiki, users: users, mp: mp}
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
func (s *PatchService) CreatePatch(ctx context.Context, userID int, vndbID string) (int, error) {
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

	// Mirror Wiki's release_date locally so /api/galgame can sort/filter by
	// 发售日期 (the local patch table is what that endpoint paginates over;
	// see migration 010 + docs/galgame_wiki/00-handbook §17). Best-effort —
	// a wiki blip just leaves release_date NULL; the one-time backfill cmd
	// or a later re-sync fills it in. Never blocks patch creation.
	//
	// MUST use GetGalgame (/galgame/:gid), NOT GalgameBatch — the lightweight
	// batch endpoint does NOT include release_date (it returns only id / name
	// / banner / content_limit / status / user_id / resource_update_time /
	// original_language / age_limit). The single-detail endpoint does.
	var releaseDate *time.Time
	if env, gErr := s.wiki.GetGalgame(ctx, galgameID, ""); gErr == nil && env != nil && env.Galgame.ReleaseDate != nil {
		releaseDate = utils.ParseWikiReleaseDate(*env.Galgame.ReleaseDate)
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
			ID:          galgameID,
			VndbID:      vndbID,
			UserID:      userID,
			ReleaseDate: releaseDate,
		}
		if err := tx.Create(p).Error; err != nil {
			return fmt.Errorf("创建 patch 失败: %w", err)
		}
		patchID = p.ID

		// Register contributor
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
	// +3 reward for creating a galgame entry — awarded AFTER commit via OAuth
	// (s2s, out of the txn). content_approved; ref/key by galgame id (= patch id).
	go s.mp.Award(context.Background(), userID, 3, "content_approved",
		fmt.Sprintf("galgame:%d", patchID), fmt.Sprintf("moyu:patch_create:%d", patchID))
	return patchID, nil
}

// GetPatch returns the header row, lazily materializing it the same way as
// GetPatchDetail (the patch layout page hits /patch/:id in parallel with
// /patch/:id/detail; both must trigger materialization or the header 404s
// and the page shows "not found" even though detail would have created it).
func (s *PatchService) GetPatch(ctx context.Context, id int) (*model.Patch, error) {
	return s.ensureLocalPatch(ctx, id)
}

// GetPatchesByIDs returns existing patches for the given ids in the caller-
// supplied order — no lazy materialization. Used by handlers that enrich a
// list of Wiki galgame ids with moyu-side stats; ids that have no local row
// are simply absent from the result so the caller can degrade to a Wiki-
// only card (banner + name + content_limit, zero stats) for those entries.
func (s *PatchService) GetPatchesByIDs(ids []int) ([]model.Patch, error) {
	return s.repo.GetPatchesByIDs(ids)
}

// GetPatchDetail returns the local patch row, lazily materializing it the
// first time a moyu user opens a galgame that is globally published on Wiki
// (e.g. created/claimed on kungal) but has no moyu row yet.
//
// Design: docs/galgame_wiki/00-handbook-for-downstream.md §7.1.4a — the
// consumer INSERTs its local stats row when a user selects an already
// published galgame. moyu's `patch` table is that stats row (D13:
// patch.id == galgame_id).
//
// Behaviour decisions (confirmed):
//   - Visibility/existence check uses anonymous Wiki /galgame/batch, which
//     only returns status=0 rows — so a hit means "publicly published";
//     a miss means "doesn't exist / banned / someone's private draft" and
//     we keep the genuine 404.
//   - The row is a PURE STATS row: no +3 moemoepoint, no contributor, no
//     contribute_count bump (the publish/claim reward was already granted on
//     the originating side; moyu must not double-count).
//   - patch.user_id = the Wiki galgame's creator/claimer (brief.UserID); ids
//     are OAuth-aligned globally so it resolves on moyu too.
func (s *PatchService) GetPatchDetail(ctx context.Context, id int) (*model.Patch, error) {
	return s.ensureLocalPatch(ctx, id)
}

// ensureLocalPatch is the shared "read, else lazily materialize" logic used
// by both the header (/patch/:id) and detail (/patch/:id/detail) endpoints.
func (s *PatchService) ensureLocalPatch(ctx context.Context, id int) (*model.Patch, error) {
	patch, err := s.repo.GetPatchDetail(id)
	if err == nil {
		return patch, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	// No local row. Ask Wiki (anonymously → status=0 only) whether this is a
	// publicly published galgame and grab its vndb_id + creator.
	//
	// content_limit="" — no NSFW filter at this layer. The detail-handler
	// caller (GetPatch / GetPatchDetail) decides whether to surface this
	// patch through *its* sfw default; lazy-creating the local row even for
	// NSFW games is fine since the listing endpoints filter them out anyway.
	briefs, bErr := s.wiki.GalgameBatch(ctx, []int{id}, "")
	if bErr != nil {
		return nil, err // surface as not-found; Wiki transient failure
	}
	var brief *galgameClient.GalgameBrief
	for i := range briefs {
		if briefs[i].ID == id {
			brief = &briefs[i]
			break
		}
	}
	if brief == nil {
		return nil, err // not publicly visible → real 404
	}

	vndb := brief.VndbID
	if vndb == "" {
		// vndb_id is uniqueIndex/NOT NULL; original works have none. id is
		// already unique, so a deterministic placeholder keeps the index sane.
		vndb = fmt.Sprintf("wiki-%d", id)
	}

	// Pure stats row. ON CONFLICT DO NOTHING makes concurrent first-opens
	// idempotent; we always re-read the canonical row afterwards.
	row := &model.Patch{ID: id, VndbID: vndb, UserID: brief.UserID}
	// A lazily-materialized row (galgame merely opened on moyu, no resources yet)
	// must NOT jump to the top of the "最近更新" sort. Inherit the galgame's real
	// resource_update_time from Wiki instead of letting autoCreateTime stamp it
	// to now. Bad/empty value → leave zero → autoCreateTime (safe fallback).
	if t, pErr := time.Parse(time.RFC3339, brief.ResourceUpdateTime); pErr == nil {
		row.ResourceUpdateTime = t
	}
	if cErr := s.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(row).Error; cErr != nil {
		return nil, cErr
	}
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
func (s *PatchService) RegisterClaimedGalgame(userID, galgameID int, vndbID string) (int, error) {
	if galgameID <= 0 {
		return 0, fmt.Errorf("invalid galgame id")
	}
	if existing, _ := s.repo.GetPatchByID(galgameID); existing != nil && existing.ID != 0 {
		// Already materialized (e.g. the galgame was browsed before claiming) —
		// no reward, but claiming is still a content action → refresh
		// resource_update_time so it surfaces in 最近更新. (A brand-new patch
		// below gets now() from autoCreateTime, so both claim paths bump.)
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
	// Claim reward +3 — awarded once per claim (the early-return above guards
	// re-claims) AFTER commit via OAuth. Key by galgame id so it's replay-safe.
	go s.mp.Award(context.Background(), userID, 3, "content_approved",
		fmt.Sprintf("galgame:%d", patchID), fmt.Sprintf("moyu:claim:%d", patchID))
	return patchID, nil
}

// DB exposes the underlying *gorm.DB so a few thin "no-business-logic" handler
// endpoints (the wiki messages read-state shims) can do single-table reads /
// upserts without round-tripping through a dedicated repo + service layer.
// Anything with real business logic should still live in a service method.
func (s *PatchService) DB() *gorm.DB { return s.db }

// TouchResourceUpdateTime bumps a galgame's patch.resource_update_time to now —
// the moyu "最近更新" sort key. Called after a successful Wiki galgame-info edit
// so editing metadata also surfaces the galgame (publish/claim already stamp it
// on their own paths). No-op if the galgame has no local patch row yet — it'll
// get a correct time when the row is first materialized.
func (s *PatchService) TouchResourceUpdateTime(gid int) {
	s.db.Model(&model.Patch{}).Where("id = ?", gid).
		Update("resource_update_time", time.Now())
}

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

	// Snapshot the patch's S3 keys BEFORE the row goes away. The DB-level
	// FK CASCADE wipes all child patch_resource and history rows when we
	// DELETE the patch — but PostgreSQL only deletes rows, it doesn't know
	// about the B2 objects those s3_key columns point to. Without this step
	// the bucket accumulates unreferenced files indefinitely.
	//
	// Two disjoint sources to drain:
	//   1. patch_resource.s3_key                       — live objects
	//   2. patch_resource_file_history.old_s3_key      — superseded objects
	//      from prior UpdateResource file substitutions (also CASCADE'd via
	//      patch_resource → history)
	//
	// Log+continue on enumeration error rather than abort: a stale enum
	// shouldn't block deleting the patch, and a periodic offline scrub can
	// always sweep stragglers later. Read failure here is exceedingly rare
	// (each call is a SELECT on a single index).
	liveKeys, kErr := s.repo.GetPatchResourceS3Keys(id)
	if kErr != nil {
		slog.Warn("DeletePatch: failed to enumerate live s3_keys for cleanup", "patch_id", id, "error", kErr)
		liveKeys = nil
	}
	historyKeys, hErr := s.repo.GetPatchResourceFileHistoryS3Keys(id)
	if hErr != nil {
		slog.Warn("DeletePatch: failed to enumerate history old_s3_keys for cleanup", "patch_id", id, "error", hErr)
		historyKeys = nil
	}

	if err := s.repo.DeletePatch(id); err != nil {
		return err
	}

	// Best-effort B2 cleanup AFTER the DB delete. Same philosophy as
	// DeleteResource: row already gone (no rollback path), S3 failures
	// only WARN. The two key sets are disjoint by construction (history
	// only records keys that have already been replaced by a newer one),
	// so a plain loop suffices — no dedup needed.
	allKeys := append(liveKeys, historyKeys...)
	if s.s3.Ready() && len(allKeys) > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		for _, key := range allKeys {
			if err := s.s3.DeleteObject(ctx, key); err != nil {
				slog.Warn("DeletePatch: 删除 S3 对象失败", "s3_key", key, "patch_id", id, "error", err)
			}
		}
	}
	return nil
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

// GetRandomPatchID returns a random patch id, optionally constrained by the
// caller's content_limit. The single-row RANDOM() path can land on a NSFW
// patch, which is fine for cl == "" but a SEO leak the moment the random
// landing page renders. With a non-empty cl we sample a batch of candidates,
// ask wiki to filter them, and pick from the survivors. Returns gorm's
// ErrRecordNotFound (mapped to ErrInternal by the handler) when no candidate
// passes the filter — extremely rare in practice (would need the entire
// 60-row random sample to be NSFW).
func (s *PatchService) GetRandomPatchID(ctx context.Context, contentLimit string) (int, error) {
	if contentLimit == "" {
		return s.repo.GetRandomPatchID()
	}
	const sampleSize = 60
	ids, err := s.repo.GetRandomPatchIDs(sampleSize)
	if err != nil || len(ids) == 0 {
		return 0, err
	}
	briefs, bErr := s.wiki.GalgameBatch(ctx, ids, contentLimit)
	if bErr != nil {
		// Fail closed so we don't ship a NSFW landing page on wiki blip.
		return 0, bErr
	}
	if len(briefs) == 0 {
		return 0, gorm.ErrRecordNotFound
	}
	// Wiki returns matching briefs but in arbitrary order; pick a uniform
	// random element of the filtered set. Don't reuse ids' order — that
	// would bias toward the original RANDOM() pick when only one survives.
	return briefs[rand.Intn(len(briefs))].ID, nil
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
	// When the admin "评论需要审核" toggle is on, the comment is created in the
	// pending state (status=1), hidden from public reads until approved. All
	// the visible-comment side effects (comment_count++, owner moemoepoint,
	// contributor) are DEFERRED to ApproveComment so pending / rejected
	// comments never inflate counts or farm points.
	pending := s.IsCommentVerifyEnabled()
	status := 0
	if pending {
		status = 1
	}
	comment := &model.PatchComment{
		GalgameID: patchID,
		UserID:    userID,
		Content:   content,
		ParentID:  parentID,
		Status:    status,
	}
	if err := s.repo.CreateComment(comment); err != nil {
		return nil, err
	}

	if !pending {
		s.applyCommentSideEffects(patchID, userID, comment.ID)
	}

	// Pre-render content_html so the immediate POST response can be appended
	// directly into the comment list on the frontend without a second fetch
	// (only done for approved comments — pending ones aren't shown).
	comment.ContentHTML = markdown.MustRender(comment.Content)

	return comment, nil
}

// applyCommentSideEffects runs the once-per-visible-comment bookkeeping:
// bump the patch's comment_count, award the owner +1 moemoepoint (unless they
// authored it), and record the commenter as a contributor. Shared by
// CreateComment (verify off) and ApproveComment (verify on, deferred).
func (s *PatchService) applyCommentSideEffects(patchID, userID, commentID int) {
	s.repo.UpdateCount(patchID, "comment_count", 1)
	patch, _ := s.repo.GetPatchByID(patchID)
	if patch != nil && patch.UserID != userID {
		go s.mp.Award(context.Background(), patch.UserID, 1, "liked",
			fmt.Sprintf("comment:%d", commentID), fmt.Sprintf("moyu:comment:%d", commentID))
	}
	s.repo.EnsureContributor(userID, patchID)
}

// ApproveComment flips a pending comment to approved and applies the deferred
// visible-comment side effects. Idempotent: approving an already-approved
// comment is a no-op. Returns the comment with content_html rendered.
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
	// Render content_html so the frontend can apply the edit optimistically,
	// mirroring CreateComment (which also returns the rendered comment).
	comment.ContentHTML = markdown.MustRender(comment.Content)
	return comment, nil
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
		// Unlike — reverse the like with the same ref; per-relation-instance key.
		s.repo.DeleteCommentLike(existing.ID)
		s.db.Model(&model.PatchComment{}).Where("id = ?", commentID).
			UpdateColumn("like_count", gorm.Expr("GREATEST(like_count - 1, 0)"))
		if comment.UserID != userID {
			go s.mp.Award(context.Background(), comment.UserID, -1, "liked",
				fmt.Sprintf("comment:%d", commentID), fmt.Sprintf("moyu:comment_unlike:%d", existing.ID))
		}
		return false, nil
	}

	// Like
	rel := &model.UserPatchCommentLikeRelation{UserID: userID, CommentID: commentID}
	s.repo.CreateCommentLike(rel)
	s.db.Model(&model.PatchComment{}).Where("id = ?", commentID).
		UpdateColumn("like_count", gorm.Expr("like_count + 1"))
	if comment.UserID != userID {
		go s.mp.Award(context.Background(), comment.UserID, 1, "liked",
			fmt.Sprintf("comment:%d", commentID), fmt.Sprintf("moyu:comment_like:%d", rel.ID))
		// Notify the comment owner. The helper existed but was never wired
		// into this path (audit F070); createDedupMessage dedups so a
		// like/unlike/like cycle won't spam the owner.
		go s.CreateLikeCommentNotification(userID, comment)
	}
	return true, nil
}

func (s *PatchService) GetCommentMarkdown(commentID int) (string, error) {
	return s.repo.GetCommentMarkdown(commentID)
}

// GetCommentPatchID returns the comment's owning patch.id so the handler can
// NSFW-gate it before serving the markdown body.
func (s *PatchService) GetCommentPatchID(commentID int) (int, error) {
	return s.repo.GetCommentPatchID(commentID)
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

func (s *PatchService) CreateResource(ctx context.Context, resource *model.PatchResource, userID int) error {
	resource.UserID = userID

	// MOYU-PR7 / M5 — upload-handle integrity.
	//
	// The D10 upload flow has no upload_session table; the client just hands
	// the server an s3_key string and we trust it. Without these two checks
	// a malicious / buggy client could submit:
	//   (a) an s3_key pointing OUTSIDE the patch upload area (e.g. paths the
	//       upload service would never have minted, possibly disclosing
	//       other tenants' objects via signed-URL probing), or
	//   (b) an s3_key already attached to another patch_resource (single-use
	//       violation — same upload claimed by N rows).
	//
	// Migration 008 adds a partial UNIQUE INDEX on (s3_key) WHERE storage='s3'
	// AND s3_key<>'' so (b) is also enforced at the DB layer (caller sees a
	// "duplicate key" error if two CreateResource races to the same key).
	if resource.Storage == "s3" {
		prefix := fmt.Sprintf("patch/%d/", resource.GalgameID)
		if !strings.HasPrefix(resource.S3Key, prefix) {
			return fmt.Errorf("s3_key 不在该 galgame 的上传路径下（应以 %q 开头）", prefix)
		}
		// For s3 storage the canonical Content payload is just the s3_key — the
		// public download URL is rebuilt at GET /resource/:id/link time via
		// S3Client.PublicURL so a CDN/domain switch needs no DB backfill.
		// The frontend can leave Content empty (DTO has no `required` for it
		// any more) and we fill it here.
		resource.Content = resource.S3Key
	} else {
		// "user" mode: the frontend supplied the user's own download link(s).
		// Require at least one — DTO-level validation is intentionally relaxed
		// (no min=1) since it would also reject the s3 branch above.
		if strings.TrimSpace(resource.Content) == "" {
			return fmt.Errorf("请填写资源链接")
		}
	}

	if err := s.repo.CreateResource(resource); err != nil {
		// Surface duplicate-key errors as a clear user-facing message rather
		// than a raw Postgres unique-violation; the partial unique index on
		// (s3_key) enforces single-use of an upload.
		msg := err.Error()
		if strings.Contains(msg, "idx_patch_resource_s3_key_unique") ||
			strings.Contains(msg, "duplicate key value") {
			return fmt.Errorf("该上传已被其它资源占用，请重新上传一次")
		}
		return err
	}

	// Update aggregates
	s.repo.UpdateCount(resource.GalgameID, "resource_count", 1)
	s.repo.RecalculatePatchAggregates(resource.GalgameID)

	// Update resource_update_time
	s.db.Model(&model.Patch{}).Where("id = ?", resource.GalgameID).
		Update("resource_update_time", time.Now())

	// Moemoepoint +3 for publishing a resource (unified via OAuth).
	go s.mp.Award(context.Background(), userID, 3, "content_approved",
		fmt.Sprintf("resource:%d", resource.ID), fmt.Sprintf("moyu:resource_publish:%d", resource.ID))

	// Ensure contributor
	s.repo.EnsureContributor(userID, resource.GalgameID)

	// Notify favorited users
	s.notifyFavoritedUsers(resource.GalgameID, userID)

	// Pre-render note_html for the immediate POST response.
	resource.NoteHTML = markdown.MustRender(resource.Note)

	// Attach publisher brief so the response shape matches GetResources (which
	// renders r.user.name on the resource card). Without this the frontend's
	// optimistic prepend onto the list would render undefined → "Cannot read
	// properties of undefined (reading 'name')" NPE. Reuses the same OAuth
	// /users/batch path the list endpoint uses so failures degrade identically.
	if s.users != nil {
		one := []model.PatchResource{*resource}
		attachUsersToResources(ctx, s.users, one)
		resource.User = one[0].User
	}

	return nil
}

// UpdateResource mutates a resource in place. When the FILE fields (Storage /
// S3Key / Content) differ from the current row, an append-only history row is
// written first inside the same transaction (MOYU-PR5 / M3) so the previous
// file pointer + blake3 + size + reason + actor are recoverable for support
// triage. Pure metadata edits (name / note / type / ...) skip history.
//
// reason is the operator's optional explanation (DTO PatchResourceUpdateRequest
// .Reason, max 500 chars). actorRole is the role-snapshot integer (3=admin,
// 2=moderator, 1=user, 0=unknown) so the audit row records the privilege at
// time of edit.
func (s *PatchService) UpdateResource(ctx context.Context, resourceID, userID int, update *model.PatchResource, reason string, actorRole int) (*model.PatchResource, error) {
	existing, err := s.repo.GetResourceByID(resourceID)
	if err != nil {
		return nil, fmt.Errorf("resource not found")
	}
	// Moderators / admins bypass the owner check so they can edit any
	// resource from the public resource page (option B per the spec —
	// "admin can edit in-place on the front-end"). actorRole is already
	// resolved by the handler from the OAuth roles claim (3=admin / 2=mod
	// / 1=user / 0=unknown). The bypass also flows into the file-history
	// row's ActorRole field so audit shows it was a mod/admin edit.
	if existing.UserID != userID && actorRole < 2 {
		return nil, fmt.Errorf("can only edit your own resources")
	}

	// Mirror CreateResource's storage-aware Content normalization so updates
	// follow the same invariant: storage="s3" → Content = S3Key (download URL
	// is derived at fetch time); storage="user" → Content is the user's link.
	if update.Storage == "s3" {
		// Prefix check only runs when the s3_key actually changes (i.e. the
		// user replaced the file). Two reasons:
		//
		//   (1) Cross-row anti-tamper is already enforced by the UserID
		//       == caller check above (or moderator bypass) and the partial
		//       UNIQUE INDEX `idx_patch_resource_s3_key_unique` (MOYU-PR7 /
		//       M5) — a user can't redirect their row to someone else's
		//       upload because they don't own the target row, and they
		//       can't claim a key already attached to another row.
		//
		//   (2) Legacy data has s3_keys shaped as "patch/{legacy_patch_id}/..."
		//       where legacy_patch_id != current galgame_id (because
		//       remap-patch-ids rewrote galgame_id but did not touch the
		//       string-immutable s3_key). Pure-metadata edits on those rows
		//       must not be rejected — the s3_key isn't changing, there's
		//       nothing to validate.
		//
		// When the user DOES replace the file, the new s3_key comes from
		// buildPatchResourceKey(galgame_id, filename) on the upload path,
		// which always produces "patch/{current_galgame_id}/...", so the
		// strict prefix check still passes — and catches forged keys.
		if update.S3Key != existing.S3Key {
			prefix := fmt.Sprintf("patch/%d/", existing.GalgameID)
			if !strings.HasPrefix(update.S3Key, prefix) {
				return nil, fmt.Errorf("s3_key 不在该 galgame 的上传路径下（应以 %q 开头）", prefix)
			}
		}
		update.Content = update.S3Key
	} else {
		if strings.TrimSpace(update.Content) == "" {
			return nil, fmt.Errorf("请填写资源链接")
		}
	}

	// File-substantive change detection. We compare update vs existing on
	// the three fields that together identify "the file/link" — storage
	// class, the S3 object key, and the external link content. Anything
	// else (note, code, password, size string, type/lang/platform jsonb) is
	// metadata-only and doesn't merit a history row.
	fileChanged := update.Storage != existing.Storage ||
		update.S3Key != existing.S3Key ||
		update.Content != existing.Content

	// Snapshot the s3_key we'll need to delete AFTER the transaction commits.
	// Taken pre-transaction because the txn body overwrites `existing` in
	// place. Only set when the old row had an S3 object whose key is NOT
	// reused by the new state — covers both "replaced file (s3→s3 new key)"
	// and "switched away from s3 (s3→user)".
	var orphanS3Key string
	if existing.Storage == "s3" && existing.S3Key != "" {
		stillSameObject := update.Storage == "s3" && update.S3Key == existing.S3Key
		if !stillSameObject {
			orphanS3Key = existing.S3Key
		}
	}

	galgameID := existing.GalgameID
	// Per-field edit diff (public-safe) — computed from existing(before) vs
	// update(after) BEFORE the txn overwrites `existing` below. Stored as a
	// PatchResourceRevision so the resource page can show 改动前 → 改动后 for
	// language / platform / type / note / name / size / file. Empty = no-op save.
	changes := diffResourceFields(existing, update)
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if fileChanged {
			hist := &model.PatchResourceFileHistory{
				ResourceID: existing.ID,
				OldStorage: existing.Storage,
				OldS3Key:   existing.S3Key,
				OldBlake3:  existing.Blake3,
				OldSize:    existing.Size,
				OldContent: existing.Content,
				Reason:     reason,
				ActorID:    userID,
				ActorRole:  actorRole,
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
		existing.Content = update.Content
		existing.Type = update.Type
		existing.Language = update.Language
		existing.Platform = update.Platform
		existing.UpdateTime = time.Now()

		return tx.Save(existing).Error
	}); err != nil {
		return nil, err
	}

	// Best-effort old-object cleanup, AFTER the txn so we don't hold a DB
	// connection during an external IO call. Failure only warn — the row
	// already points at the new file (no user-facing impact), and the
	// patch_resource_file_history.old_s3_key audit trail still records the
	// old key so support can recover the object out-of-band if needed.
	// Mirrors DeleteResource's S3 cleanup so update + delete paths agree.
	if orphanS3Key != "" && s.s3.Ready() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.s3.DeleteObject(cleanupCtx, orphanS3Key); err != nil {
			slog.Warn("UpdateResource: 删除旧 S3 对象失败", "s3_key", orphanS3Key, "resource_id", resourceID, "error", err)
		}
	}

	// Aggregate refresh outside the transaction — it's a derived counter
	// touching the parent patch row; doesn't need to share the txn.
	s.repo.RecalculatePatchAggregates(galgameID)

	// Editing a resource is a content update → bump resource_update_time so the
	// galgame rises in the "最近更新" sort (mirrors CreateResource).
	s.db.Model(&model.Patch{}).Where("id = ?", galgameID).
		Update("resource_update_time", time.Now())

	// Re-render note_html + attach publisher brief so the response shape
	// matches GetResources / CreateResource. Without this the frontend's
	// optimistic list-row replacement would keep showing the OLD note_html
	// (form only sends raw markdown `note`, not rendered HTML) → "note 改了
	// 但简介不更新" bug. Same OAuth /users/batch hop the list endpoint uses.
	existing.NoteHTML = markdown.MustRender(existing.Note)
	if s.users != nil {
		one := []model.PatchResource{*existing}
		attachUsersToResources(ctx, s.users, one)
		existing.User = one[0].User
	}
	return existing, nil
}

func (s *PatchService) DeleteResource(resourceID, userID int, isPrivileged bool) error {
	resource, err := s.repo.GetResourceByID(resourceID)
	if err != nil {
		return fmt.Errorf("resource not found")
	}
	// Same option-B bypass as UpdateResource: moderators / admins can delete
	// any resource from the front-end public page without round-tripping
	// through /admin/resource/:id. The admin route still exists for audit /
	// management; both code paths converge on best-effort S3 cleanup below.
	if resource.UserID != userID && !isPrivileged {
		return fmt.Errorf("can only delete your own resources")
	}

	// Snapshot history old_s3_keys BEFORE DELETE — patch_resource_file_history
	// CASCADE's away with the resource row, taking its old_s3_key references
	// with it. See DeletePatch's drain pattern for the rationale.
	historyKeys, hErr := s.repo.GetResourceFileHistoryS3Keys(resourceID)
	if hErr != nil {
		slog.Warn("DeleteResource: failed to enumerate history old_s3_keys for cleanup", "resource_id", resourceID, "error", hErr)
		historyKeys = nil
	}

	if err := s.repo.DeleteResource(resourceID); err != nil {
		return err
	}

	// Best-effort S3 cleanup. We deliberately do NOT fail the whole op if this
	// errors — the DB row is already gone, the user-facing operation is "done",
	// and a left-over object is recoverable later (manual sweep / a future
	// orphan-scrub cron — cron currently only catches in-flight multiparts via
	// cleanupAbortedMultiparts, not completed-but-unreferenced small uploads).
	//
	// Drain both the current s3_key and the history's old_s3_keys (the two
	// sets are disjoint by construction — history rows record only keys
	// previously replaced by a newer one).
	if s.s3.Ready() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if resource.Storage == "s3" && resource.S3Key != "" {
			if err := s.s3.DeleteObject(ctx, resource.S3Key); err != nil {
				slog.Warn("DeleteResource: 删除 S3 对象失败", "s3_key", resource.S3Key, "resource_id", resourceID, "error", err)
			}
		}
		for _, key := range historyKeys {
			if err := s.s3.DeleteObject(ctx, key); err != nil {
				slog.Warn("DeleteResource: 删除 S3 历史对象失败", "s3_key", key, "resource_id", resourceID, "error", err)
			}
		}
	}

	s.repo.UpdateCount(resource.GalgameID, "resource_count", -1)
	s.repo.RecalculatePatchAggregates(resource.GalgameID)
	// Moemoepoint always decremented from the resource OWNER, not the caller.
	// When a mod/admin deletes someone else's resource the owner still loses
	// the +3 they earned at upload time. Same ref as the publish award so OAuth
	// can reconcile (content_removed reverses content_approved).
	go s.mp.Award(context.Background(), resource.UserID, -3, "content_removed",
		fmt.Sprintf("resource:%d", resource.ID), fmt.Sprintf("moyu:resource_delete:%d", resource.ID))
	return nil
}

// ToggleResourceDisable flips a resource between enabled (status 0, downloadable)
// and disabled (status 1, download link blocked — used to pull a virus-infected
// resource without deleting it). Permitted for the resource owner or a
// privileged user (moderator/admin). Returns the resulting status.
func (s *PatchService) ToggleResourceDisable(resourceID, userID int, isPrivileged bool) (int, error) {
	resource, err := s.repo.GetResourceByID(resourceID)
	if err != nil {
		return 0, fmt.Errorf("resource not found")
	}
	if resource.UserID != userID && !isPrivileged {
		return 0, fmt.Errorf("no permission to operate on this resource")
	}
	if err := s.repo.ToggleResourceStatus(resourceID); err != nil {
		return 0, err
	}
	// Repo flips 0↔1 atomically (SQL CASE); the new value is the inverse of the
	// one we just read.
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

// GetResourceDownloadInfo backs the lightweight GET /patch/resource/:id/link.
// The /resource/:id detail endpoint additionally Wiki-enriches the owning
// patch and fetches 5 recommendations, which is wasteful when the caller
// only wants the download links. This returns the bare resource row.
func (s *PatchService) GetResourceDownloadInfo(resourceID int) (*model.PatchResource, error) {
	r, err := s.repo.GetResourceByID(resourceID)
	if err != nil {
		return nil, fmt.Errorf("resource not found")
	}
	// Content is returned as stored: storage="s3" → the bare s3_key (object
	// path, CreateResource invariant), storage="user" → the raw link list. The
	// public download URL is assembled on the frontend (resolveDownloadLinks
	// prepends domain.storage for s3 — same pattern as resolveAvatarUrl with
	// domain.imageBed), so swapping the download CDN/domain is a single
	// frontend-config change with no DB backfill.
	return r, nil
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
			go s.mp.Award(context.Background(), patch.UserID, -1, "liked",
				fmt.Sprintf("galgame:%d", patchID), fmt.Sprintf("moyu:unfavorite:%d", existing.ID))
		}
		return false, nil
	}

	rel := &model.UserPatchFavoriteRelation{UserID: userID, GalgameID: patchID}
	s.repo.CreateFavorite(rel)
	s.repo.UpdateCount(patchID, "favorite_count", 1)
	if patch.UserID != userID {
		go s.mp.Award(context.Background(), patch.UserID, 1, "liked",
			fmt.Sprintf("galgame:%d", patchID), fmt.Sprintf("moyu:favorite:%d", rel.ID))
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
	return markdown.ExtractMentionedUserIDs(content)
}

// ===== Notifications (simplified) =====

func (s *PatchService) notifyFavoritedUsers(patchID, senderID int) {
	var userIDs []int
	s.db.Model(&model.UserPatchFavoriteRelation{}).
		Where("galgame_id = ? AND user_id != ?", patchID, senderID).
		Pluck("user_id", &userIDs)

	for _, userID := range userIDs {
		s.createDedupMessage(senderID, userID, "patchResourceCreate",
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
	for _, userID := range ids {
		if userID != senderID {
			s.createDedupMessage(senderID, userID, "mention", excerpt,
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
//
// Source of truth is the site_setting table via settingService (durable +
// audited), read directly — see internal/setting.

func (s *PatchService) IsCommentVerifyEnabled() bool {
	return s.setting.GetBool(settingService.KeyCommentVerify)
}

// IsCreatorOnlyEnabled reports the admin "仅创作者(role>2)可发布 Galgame" toggle.
// When on, the publish handlers reject non-moderator/admin users.
func (s *PatchService) IsCreatorOnlyEnabled() bool {
	return s.setting.GetBool(settingService.KeyCreatorOnly)
}

// GetResourceFileHistory returns the privacy-safe, paginated file-replacement
// audit for one resource. Public (any visitor, incl. anonymous): deliberately
// omits old_s3_key (internal storage key) and old_content (the old download
// links) — those stay behind the rate-limited /link endpoint. Callers see only
// when / who-role / why / old size + hash.
func (s *PatchService) GetResourceFileHistory(resourceID, page, limit int) ([]dto.PublicResourceFileHistoryItem, int64, error) {
	rows, total, err := s.repo.GetResourceFileHistory(resourceID, (page-1)*limit, limit)
	if err != nil {
		return nil, 0, err
	}
	items := make([]dto.PublicResourceFileHistoryItem, 0, len(rows))
	for _, h := range rows {
		items = append(items, dto.PublicResourceFileHistoryItem{
			ID:         h.ID,
			OldStorage: h.OldStorage,
			OldBlake3:  h.OldBlake3,
			OldSize:    h.OldSize,
			Reason:     h.Reason,
			ActorRole:  h.ActorRole,
			CreatedAt:  h.CreatedAt,
		})
	}
	return items, total, nil
}

// diffResourceFields computes the public-safe per-field diff between the
// pre-edit (before) and post-edit (after) resource. Secrets (download link /
// s3 key / extract code / unzip password) are never emitted as raw values —
// only a single "已更新" marker. Used by UpdateResource to record a revision.
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
	// blake3 故意不在此 diff:它由文件自动计算、UpdateResource 不写入它(编辑表单
	// 也不回传),直接比较会让每次元数据编辑都误报 "hash → (空)"。文件替换通过
	// size/storage 变化 + 下面的「已更新」标记体现,blake3 本身不参与字段 diff。
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

// GetResourceRevisions returns the paginated per-field edit history for one
// resource (public; Changes are secret-free).
func (s *PatchService) GetResourceRevisions(resourceID, page, limit int) ([]model.PatchResourceRevision, int64, error) {
	return s.repo.GetResourceRevisions(resourceID, (page-1)*limit, limit)
}
