// Package cron centralizes all cron jobs.
package cron

import (
	"context"
	"log/slog"
	"time"

	"kun-galgame-patch-api/internal/constants"
	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/internal/infrastructure/storage"
	"kun-galgame-patch-api/pkg/imageclient"
	"kun-galgame-patch-api/pkg/moemoepoint"

	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

// Start starts all cron jobs and returns a stop function for graceful shutdown.
//
// Job list:
//  1. Daily 00:00: reset daily_image_count / daily_check_in / daily_upload_size on the user table
//  2. Every 6 hours: clean up S3 multipart uploads still unfinished after 24h (D10 plan B)
//  3. Every 10 minutes: pull Wiki message feed, apply approved/declined/banned/unbanned events
//     (idempotent via wiki_message_processed; awards +3 moemoepoint on approved)
//  4. Daily 04:00: ref-ping image_service for every hash moyu still references
//     (doc banners + content /image/<hash> tokens) so its GC doesn't reclaim them
func Start(db *gorm.DB, s3 *storage.S3Client, wiki *galgameClient.Client, mp *moemoepoint.Client, img *imageclient.Client) func() {
	// Pin the schedule to Asia/Shanghai so the daily 00:00 reset fires at the
	// intended civil midnight regardless of host TZ (audit F085). The check-in
	// idempotency key's date (user/service) is pinned to the same zone so the
	// "day" boundary agrees on both sides. Fall back to host-local if tzdata
	// is unavailable.
	loc, locErr := time.LoadLocation("Asia/Shanghai")
	if locErr != nil || loc == nil {
		loc = time.Local
	}
	c := cron.New(cron.WithLocation(loc))

	// ── Daily 00:00: reset quota fields ───────────────
	if _, err := c.AddFunc("0 0 * * *", func() {
		result := db.Table("user").Where(
			"daily_image_count <> 0 OR daily_check_in <> 0 OR daily_upload_size <> 0",
		).Updates(map[string]any{
			"daily_image_count": 0,
			"daily_check_in":    0,
			"daily_upload_size": 0,
		})
		if result.Error != nil {
			slog.Error("每日重置失败", "error", result.Error)
			return
		}
		slog.Info("每日重置完成", "affected", result.RowsAffected)
	}); err != nil {
		slog.Error("注册每日重置任务失败", "error", err)
	}

	// ── Every 6 hours: clean up unfinished S3 multipart uploads ──
	if _, err := c.AddFunc("0 */6 * * *", func() {
		cleanupAbortedMultiparts(s3)
	}); err != nil {
		slog.Error("注册 multipart 清理任务失败", "error", err)
	}

	// ── Every 10 minutes: sync Wiki message feed ─────────
	// Only registered when wiki is configured; tests / cmd helpers that build
	// the app without a wiki client (e.g. cmd/remap-patch-ids) won't run this.
	if wiki != nil {
		if _, err := c.AddFunc(wikiSyncSchedule, func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			applied, cursor, err := RunWikiMessageSync(ctx, db, wiki, mp)
			if err != nil {
				slog.Error("Wiki 消息同步失败", "error", err, "applied", applied, "cursor", cursor)
				return
			}
			if applied > 0 {
				slog.Info("Wiki 消息同步完成", "applied", applied, "cursor", cursor)
			}
		}); err != nil {
			slog.Error("注册 Wiki 消息同步任务失败", "error", err)
		}
	}

	// ── Daily 04:00: ref-ping image_service ──────────────
	// Refreshes last_referenced_at for every hash moyu still references so the
	// image_service GC (cold-storage TTL ~60d) doesn't reclaim them. Only when
	// image_service is configured (skipped in dev with no client).
	if img != nil && img.Configured() {
		if _, err := c.AddFunc("0 4 * * *", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			updated, notFound, err := RunReferencePing(ctx, db, img)
			if err != nil {
				slog.Error("image ref-ping 失败", "error", err)
				return
			}
			slog.Info("image ref-ping 完成", "updated", updated, "not_found", notFound)
		}); err != nil {
			slog.Error("注册 image ref-ping 任务失败", "error", err)
		}
	}

	c.Start()
	slog.Info("定时任务已启动")

	return func() {
		ctx := c.Stop()
		<-ctx.Done()
		slog.Info("定时任务已停止")
	}
}

// cleanupAbortedMultiparts scans all multipart uploads in the bucket and aborts any that have been pending for more than 24h.
func cleanupAbortedMultiparts(s3 *storage.S3Client) {
	if s3 == nil || !s3.Ready() {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	uploads, err := s3.ListIncompleteUploads(ctx, "")
	if err != nil {
		slog.Error("列出未完成 multipart 失败", "error", err)
		return
	}

	cutoff := time.Now().Add(-constants.MultipartUploadOrphanTTL)
	aborted := 0
	for _, u := range uploads {
		if !u.Initiated.Before(cutoff) {
			continue
		}
		if err := s3.RemoveIncompleteUpload(ctx, u.Key); err != nil {
			slog.Warn("abort multipart 失败", "key", u.Key, "error", err)
			continue
		}
		aborted++
	}
	if aborted > 0 {
		slog.Info("清理孤儿 multipart 完成", "aborted", aborted, "scanned", len(uploads))
	}
}
