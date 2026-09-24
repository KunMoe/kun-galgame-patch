package cron

import (
	"context"
	"log/slog"
	"time"

	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/pkg/catalogv2"
	"kun-galgame-patch-api/pkg/imageclient"

	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

type NotificationSync interface {
	Configured() bool
	Sync(ctx context.Context) (int, error)
}

func Start(
	db *gorm.DB,
	galgame *galgameClient.Client,
	img *imageclient.Client,
	comments CommentImages,
	notifications NotificationSync,
) func() {
	loc, locErr := time.LoadLocation("Asia/Shanghai")
	if locErr != nil || loc == nil {
		loc = time.Local
	}
	c := cron.New(cron.WithLocation(loc))

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

	var catalog *catalogv2.Client
	if galgame != nil {
		catalog = galgame.V2()
	}
	if catalog != nil && catalog.Configured() {
		if _, err := c.AddFunc(claimSyncSchedule, func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			applied, cursor, err := RunClaimEventSync(ctx, db, catalog, galgame)
			if err != nil {
				slog.Error("catalog claim 同步失败", "error", err, "applied", applied, "cursor", cursor)
				return
			}
			if applied > 0 {
				slog.Info("catalog claim 同步完成", "applied", applied, "cursor", cursor)
			}
		}); err != nil {
			slog.Error("注册 catalog claim 同步任务失败", "error", err)
		}

		if _, err := c.AddFunc(mirrorSyncSchedule, func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()
			updated, caughtUp, err := RunCatalogMirrorSync(ctx, db, catalog, galgame)
			if err != nil {
				slog.Error("catalog 展示轴同步失败", "error", err, "updated", updated)
				return
			}
			if updated > 0 || !caughtUp {
				slog.Info("catalog 展示轴同步完成", "updated", updated, "caught_up", caughtUp)
			}
		}); err != nil {
			slog.Error("注册 catalog 展示轴同步任务失败", "error", err)
		}

		if _, err := c.AddFunc(mergeSyncSchedule, func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()
			report, caughtUp, err := RunCatalogMergeSync(ctx, db, catalog, true)
			if err != nil {
				slog.Error("catalog 合并同步失败", "error", err, "applied", report.Applied())
				return
			}
			if report.Applied() > 0 || !caughtUp {
				slog.Info("catalog 合并同步完成",
					"folded", report.Folded, "renumbered", report.Renumbered,
					"ledger", report.Ledger, "caught_up", caughtUp)
			}
		}); err != nil {
			slog.Error("注册 catalog 合并同步任务失败", "error", err)
		}
	}

	if img != nil && img.Configured() {
		if _, err := c.AddFunc("0 4 * * *", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			updated, notFound, err := RunReferencePing(ctx, db, img, comments)
			if err != nil {
				slog.Error("image ref-ping 失败", "error", err)
				return
			}
			slog.Info("image ref-ping 完成", "updated", updated, "not_found", notFound)
		}); err != nil {
			slog.Error("注册 image ref-ping 任务失败", "error", err)
		}
	}

	if notifications != nil && notifications.Configured() {
		job := cron.NewChain(cron.SkipIfStillRunning(cron.DiscardLogger)).Then(cron.FuncJob(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			n, err := notifications.Sync(ctx)
			if err != nil {
				slog.Error("社区通知同步失败", "error", err)
				return
			}
			if n > 0 {
				slog.Info("社区通知同步完成", "applied", n)
			}
		}))
		if _, err := c.AddJob("@every 20s", job); err != nil {
			slog.Error("注册社区通知同步任务失败", "error", err)
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
