package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	commentRepo "kun-galgame-patch-api/internal/comment/repository"
	commentService "kun-galgame-patch-api/internal/comment/service"
	communityAnchor "kun-galgame-patch-api/internal/community/anchor"
	"kun-galgame-patch-api/internal/infrastructure/cron"
	"kun-galgame-patch-api/internal/infrastructure/database"
	"kun-galgame-patch-api/pkg/communityclient"
	"kun-galgame-patch-api/pkg/config"
	"kun-galgame-patch-api/pkg/imageclient"
	"kun-galgame-patch-api/pkg/logger"

	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	cfg := config.Load()
	logger.Init(cfg.Server.Mode)

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

	// Comment bodies are upstream now, so a run without the community client
	// would ping only what is left in SQL and quietly stop referencing every
	// image ever posted in a comment.
	communityCli := communityclient.New(communityclient.Config{
		BaseURL:      cfg.Community.BaseURL,
		ClientID:     cfg.Community.ClientID,
		ClientSecret: cfg.Community.ClientSecret,
	})
	if !communityCli.Configured() {
		slog.Error("community 未配置 (KUN_COMMUNITY_API_BASE / client_id / secret); 评论里的图片不会被 ref-ping")
		os.Exit(1)
	}
	commentRepository := commentRepo.New(db)
	commentSvc := commentService.New(communityCli, commentRepository,
		communityAnchor.New(nil, commentRepository), nil, nil, db, nil, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	updated, notFound, err := cron.RunReferencePing(ctx, db, img, commentSvc)
	if err != nil {
		slog.Error("image ref-ping 失败", "error", err, "updated", updated, "not_found", notFound)
		os.Exit(1)
	}
	slog.Info("image ref-ping 完成", "updated", updated, "not_found", notFound)
}
