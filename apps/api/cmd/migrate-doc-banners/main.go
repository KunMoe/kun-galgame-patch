// cmd/migrate-doc-banners moves legacy static doc banners into image_service.
//
// Before migration 016 the /about (now doc) posts used static banner files
// (`/posts/<category>/<slug>/banner.avif`, served by the web container). The
// unified doc feature wants all banners in image_service. For each doc that
// still has a static `banner` and no `banner_image_hash`, this downloads the
// file from the public site and uploads it to image_service, then stores the
// returned hash. Idempotent — already-migrated docs (hash set) are skipped, so
// it is safe to re-run.
//
// Usage:
//
//	go run ./cmd/migrate-doc-banners                          # uses KUN_* env
//	go run ./cmd/migrate-doc-banners -dry-run                 # list only
//	go run ./cmd/migrate-doc-banners -banner-base=https://www.moyu.moe
//
// Containerized (must reach image_service on the dokploy network):
//
//	docker run --rm --network <net> -e KUN_DATABASE_URL=... \
//	  -e KUN_IMAGE_SERVICE_BASE_URL=http://image:9278 \
//	  -e KUN_IMAGE_CDN_BASE=... -e OAUTH_CLIENT_ID=... -e OAUTH_CLIENT_SECRET=... \
//	  ghcr.io/kunmoe/moyu-tools migrate-doc-banners
package main

import (
	"bytes"
	"context"
	"flag"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	docModel "kun-galgame-patch-api/internal/doc/model"
	"kun-galgame-patch-api/internal/infrastructure/database"
	"kun-galgame-patch-api/pkg/config"
	"kun-galgame-patch-api/pkg/imageclient"
	"kun-galgame-patch-api/pkg/logger"

	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	dryRun := flag.Bool("dry-run", false, "只列出待迁移项，不上传/写库")
	bannerBase := flag.String("banner-base", "https://www.moyu.moe", "静态封面所在站点 origin")
	preset := flag.String("preset", "topic", "image_service preset（moyu 已启用 topic）")
	flag.Parse()

	cfg := config.Load()
	logger.Init(cfg.Server.Mode)

	db := database.NewPostgres(cfg.Database, cfg.Server.Mode)

	// Same credential-defaulting as the server (app.go): fall back to the OAuth
	// client when the dedicated KUN_IMAGE_OAUTH_* vars are unset.
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

	var docs []docModel.Doc
	if err := db.Where("banner <> '' AND (banner_image_hash = '' OR banner_image_hash IS NULL)").
		Find(&docs).Error; err != nil {
		slog.Error("查询 doc 失败", "error", err)
		os.Exit(1)
	}
	slog.Info("待迁移封面", "count", len(docs))

	hc := &http.Client{Timeout: 30 * time.Second}
	ctx := context.Background()
	var ok int
	for _, d := range docs {
		url := d.Banner
		if strings.HasPrefix(url, "/") {
			url = strings.TrimRight(*bannerBase, "/") + url
		}
		if *dryRun {
			slog.Info("dry-run", "slug", d.Slug, "banner", url)
			continue
		}

		resp, err := hc.Get(url)
		if err != nil || resp.StatusCode != http.StatusOK {
			code := 0
			if resp != nil {
				code = resp.StatusCode
				resp.Body.Close()
			}
			slog.Warn("下载封面失败，跳过", "slug", d.Slug, "url", url, "status", code, "err", err)
			continue
		}
		data, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		fn := url[strings.LastIndex(url, "/")+1:]
		res, err := img.Upload(ctx, bytes.NewReader(data), fn, "", *preset)
		if err != nil {
			slog.Warn("上传 image_service 失败，跳过", "slug", d.Slug, "err", err)
			continue
		}
		if err := db.Model(&docModel.Doc{}).Where("id = ?", d.ID).
			Update("banner_image_hash", res.Hash).Error; err != nil {
			slog.Warn("写 banner_image_hash 失败", "slug", d.Slug, "err", err)
			continue
		}
		slog.Info("已迁移封面", "slug", d.Slug, "hash", res.Hash)
		ok++
	}
	slog.Info("封面迁移完成", "migrated", ok, "total", len(docs))
}
