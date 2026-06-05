package app

import (
	"log/slog"

	adminHandler "kun-galgame-patch-api/internal/admin/handler"
	adminRepo "kun-galgame-patch-api/internal/admin/repository"
	adminService "kun-galgame-patch-api/internal/admin/service"
	authHandler "kun-galgame-patch-api/internal/auth/handler"
	authRepo "kun-galgame-patch-api/internal/auth/repository"
	docHandler "kun-galgame-patch-api/internal/doc/handler"
	docRepository "kun-galgame-patch-api/internal/doc/repository"
	docService "kun-galgame-patch-api/internal/doc/service"
	authService "kun-galgame-patch-api/internal/auth/service"
	chatHandler "kun-galgame-patch-api/internal/chat/handler"
	chatRepo "kun-galgame-patch-api/internal/chat/repository"
	chatService "kun-galgame-patch-api/internal/chat/service"
	"kun-galgame-patch-api/internal/common"
	searchPkg "kun-galgame-patch-api/internal/common/search"
	uploadPkg "kun-galgame-patch-api/internal/common/upload"
	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/internal/infrastructure/cache"
	cronJobs "kun-galgame-patch-api/internal/infrastructure/cron"
	"kun-galgame-patch-api/internal/infrastructure/database"
	"kun-galgame-patch-api/internal/infrastructure/storage"
	messageHandler "kun-galgame-patch-api/internal/message/handler"
	messageRepo "kun-galgame-patch-api/internal/message/repository"
	messageService "kun-galgame-patch-api/internal/message/service"
	"kun-galgame-patch-api/internal/middleware"
	patchHandler "kun-galgame-patch-api/internal/patch/handler"
	patchRepo "kun-galgame-patch-api/internal/patch/repository"
	patchService "kun-galgame-patch-api/internal/patch/service"
	settingService "kun-galgame-patch-api/internal/setting/service"
	userHandler "kun-galgame-patch-api/internal/user/handler"
	userRepo "kun-galgame-patch-api/internal/user/repository"
	userService "kun-galgame-patch-api/internal/user/service"
	"kun-galgame-patch-api/pkg/config"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/imageclient"
	"kun-galgame-patch-api/pkg/moemoepoint"
	"kun-galgame-patch-api/pkg/response"
	"kun-galgame-patch-api/pkg/userclient"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type App struct {
	Fiber      *fiber.App
	DB         *gorm.DB
	RDB        *redis.Client
	S3         *storage.S3Client
	UserClient *userclient.Client
	Config     *config.Config

	// Handlers
	AuthHandler    *authHandler.AuthHandler
	PatchHandler   *patchHandler.PatchHandler
	UserHandler    *userHandler.UserHandler
	MessageHandler *messageHandler.MessageHandler
	AdminHandler   *adminHandler.AdminHandler
	CommonHandler  *common.CommonHandler
	UploadHandler  *uploadPkg.Handler
	ChatHandler    *chatHandler.ChatHandler
	SearchHandler  *searchPkg.Handler
	DocHandler     *docHandler.DocHandler

	// CronStop is called during graceful shutdown to stop the cron jobs.
	CronStop func()
}

func New(cfg *config.Config) *App {
	// Infrastructure
	db := database.NewPostgres(cfg.Database, cfg.Server.Mode)
	rdb := cache.NewRedis(cfg.Redis)
	s3 := storage.NewS3(cfg.S3)
	wiki := galgameClient.New(cfg.GalgameWiki.BaseURL)
	// Wiki's /galgame/messages/feed uses OAuth Client Basic Auth (same
	// client_id/secret as /users/batch). The wiki-sync cron is the sole
	// consumer; user-facing endpoints continue to use Bearer transparently.
	wiki.SetBasicAuth(cfg.OAuth.ClientID, cfg.OAuth.ClientSecret)

	// OAuth user-brief client (Phase 5-6 will inject this into renderers).
	usrCli := userclient.New(userclient.Config{
		BaseURL:      cfg.OAuth.ServerURL,
		ClientID:     cfg.OAuth.ClientID,
		ClientSecret: cfg.OAuth.ClientSecret,
	})

	// moemoepoint: OAuth is the unified source of truth. The Awarder routes every
	// balance change through OAuth's idempotent s2s endpoint and mirrors the
	// returned balance into the local user.moemoepoint read-cache. Same OAuth
	// base + Basic-Auth creds as the user-brief client.
	mpClient := moemoepoint.New(moemoepoint.Config{
		BaseURL:      cfg.OAuth.ServerURL,
		ClientID:     cfg.OAuth.ClientID,
		ClientSecret: cfg.OAuth.ClientSecret,
	})
	mpAwarder := moemoepoint.NewAwarder(mpClient, db)

	// Auth module
	authRepository := authRepo.New(db)
	authSvc := authService.New(authRepository, rdb, cfg.OAuth)
	authHdl := authHandler.New(authSvc, rdb, db, usrCli)

	// Site settings (source of truth for admin toggles; shared by patch + admin)
	settingSvc := settingService.New(db)

	// Patch module
	patchRepository := patchRepo.New(db)
	patchSvc := patchService.New(patchRepository, settingSvc, db, s3, wiki, usrCli, mpAwarder)
	patchHdl := patchHandler.New(patchSvc, wiki, usrCli)

	// User module
	userRepository := userRepo.New(db)
	userSvc := userService.New(userRepository, s3, usrCli, wiki, db, mpAwarder)
	userHdl := userHandler.New(userSvc, wiki, usrCli)

	// Message module
	messageRepository := messageRepo.New(db)
	messageSvc := messageService.New(messageRepository)
	messageHdl := messageHandler.New(messageSvc, usrCli)

	// Admin module
	adminRepository := adminRepo.New(db)
	adminSvc := adminService.New(adminRepository, rdb, settingSvc, s3)
	adminHdl := adminHandler.New(adminSvc, wiki, usrCli)

	// Common handler (direct DB access for simple aggregation endpoints)
	commonHdl := common.NewHandler(db, wiki, usrCli)

	// Upload module (D10: minio-go presigned URL direct upload).
	// rdb is passed so verifyAndFinalize can SETNX-dedupe Complete calls and
	// prevent double-charging daily_upload_size (MOYU-PR7 / M5).
	uploadSvc := uploadPkg.New(s3, db, rdb)
	// image_service client (W2 / PR3b). Defaults credentials to the project's
	// OAuth client when the dedicated KUN_IMAGE_OAUTH_* env vars are unset —
	// image_service reuses the OAuth oauth_client table as its "site" registry,
	// so the same credentials work end-to-end provided the admin flipped
	// image_enabled=true for this client.
	imgCfg := cfg.ImageService
	if imgCfg.ClientID == "" {
		imgCfg.ClientID = cfg.OAuth.ClientID
	}
	if imgCfg.ClientSecret == "" {
		imgCfg.ClientSecret = cfg.OAuth.ClientSecret
	}
	imgCli := imageclient.New(imageclient.Config{
		BaseURL:      imgCfg.BaseURL,
		CDNBase:      imgCfg.CDNBase,
		ClientID:     imgCfg.ClientID,
		ClientSecret: imgCfg.ClientSecret,
	})
	uploadHdl := uploadPkg.NewHandler(uploadSvc, imgCli)

	// Chat module (D9: REST only, no WebSocket)
	chatRepository := chatRepo.New(db)
	chatSvc := chatService.New(chatRepository)
	chatHdl := chatHandler.New(chatSvc, usrCli)

	// Search module (D11: delegate to Galgame Wiki Service)
	searchHdl := searchPkg.New(db, wiki)

	// Doc module (migration 016; unifies the former /about + /blog into one
	// "doc" feature): category-tree + slug docs with admin CRUD; banners and
	// inline images go through image_service (imgCli derives URLs from hashes).
	docRepo := docRepository.New(db)
	docSvc := docService.New(docRepo, imgCli, usrCli)
	docHdl := docHandler.New(docSvc)

	// Cookie mode: use Secure cookies in prod; must be off for HTTP in dev
	middleware.SecureCookies = cfg.Server.Mode == "prod"

	// Fiber app
	//
	// ReadBufferSize raised from the 4 KB default to 32 KB to survive the dev
	// environment's shared 127.0.0.1 cookie jar — browsers don't isolate cookies
	// by port, so OAuth (9277 + 9420) + moyu (5214 + 6969) all accumulate into
	// one jar and the combined Cookie header trips Fiber's request header limit
	// (logs: `Request Header Fields Too Large`). 32 KB is well under fasthttp's
	// hard limit and inert in prod where the services live on separate domains.
	app := fiber.New(fiber.Config{
		BodyLimit:      10 * 1024 * 1024, // 10MB
		ReadBufferSize: 32 * 1024,        // 32KB headers, see comment above
		ErrorHandler:   globalErrorHandler,
	})

	app.Use(recover.New())
	app.Use(middleware.CORS(cfg.CORS))

	// Start cron jobs (wiki-sync registered only when wiki client is available)
	cronStop := cronJobs.Start(db, s3, wiki, mpClient)

	slog.Info("Application initialized")

	return &App{
		Fiber:          app,
		DB:             db,
		RDB:            rdb,
		S3:             s3,
		UserClient:     usrCli,
		Config:         cfg,
		AuthHandler:    authHdl,
		PatchHandler:   patchHdl,
		UserHandler:    userHdl,
		MessageHandler: messageHdl,
		AdminHandler:   adminHdl,
		CommonHandler:  commonHdl,
		UploadHandler:  uploadHdl,
		ChatHandler:    chatHdl,
		SearchHandler:  searchHdl,
		DocHandler:     docHdl,
		CronStop:       cronStop,
	}
}

func globalErrorHandler(c *fiber.Ctx, err error) error {
	if appErr, ok := err.(*errors.AppError); ok {
		return response.Error(c, appErr)
	}

	slog.Error("Unhandled error", "error", err, "method", c.Method(), "path", c.Path())
	return response.Error(c, errors.ErrInternal(""))
}
