// cmd/image-refping runs the image_service ref-ping once and exits non-zero on
// any problem. It's the SAME logic the in-process daily cron runs (cron.Run-
// ReferencePing) — extracted as a standalone binary so it can be scheduled
// externally (crontab / systemd timer / k8s CronJob) where a non-zero exit is
// an alertable signal. An in-process robfig cron can only slog.Error; it can't
// fail "loudly" to a monitor. Running both is harmless (ping is idempotent);
// switching the daily refresh to this external runner is the eventual goal.
//
// Exits 1 when: image_service unconfigured, the DB scan fails, the ping HTTP
// call fails, OR image_service refreshed 0 of N referenced hashes (the silent-
// breakage signature — see cron.RunReferencePing).
//
//	go run ./cmd/image-refping
//	docker run --rm --network <net> --env-file <api.env> ghcr.io/kunmoe/moyu-tools image-refping
package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"kun-galgame-patch-api/internal/infrastructure/cron"
	"kun-galgame-patch-api/internal/infrastructure/database"
	"kun-galgame-patch-api/pkg/config"
	"kun-galgame-patch-api/pkg/imageclient"
	"kun-galgame-patch-api/pkg/logger"

	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	cfg := config.Load()
	logger.Init(cfg.Server.Mode)

	// Same credential-defaulting as the server (app.go).
	imgCfg := cfg.ImageService
	if imgCfg.ClientID == "" {
		imgCfg.ClientID = cfg.OAuth.ClientID
	}
	if imgCfg.ClientSecret == "" {
		imgCfg.ClientSecret = cfg.OAuth.ClientSecret
	}
	img := imageclient.New(imageclient.Config{
		BaseURL:      imgCfg.BaseURL,
		CDNBase:      imgCfg.CDNBase,
		ClientID:     imgCfg.ClientID,
		ClientSecret: imgCfg.ClientSecret,
	})
	if !img.Configured() {
		slog.Error("image_service 未配置 (KUN_IMAGE_SERVICE_BASE_URL / client_id / secret)")
		os.Exit(1)
	}

	db := database.NewPostgres(cfg.Database, cfg.Server.Mode)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	updated, notFound, err := cron.RunReferencePing(ctx, db, img)
	if err != nil {
		slog.Error("image ref-ping 失败", "error", err, "updated", updated, "not_found", notFound)
		os.Exit(1)
	}
	slog.Info("image ref-ping 完成", "updated", updated, "not_found", notFound)
}
