package app

import (
	"log/slog"

	aboutHandler "kun-galgame-patch-api/internal/about/handler"
	aboutService "kun-galgame-patch-api/internal/about/service"
	adminHandler "kun-galgame-patch-api/internal/admin/handler"
	adminRepo "kun-galgame-patch-api/internal/admin/repository"
	adminService "kun-galgame-patch-api/internal/admin/service"
	authHandler "kun-galgame-patch-api/internal/auth/handler"
	authRepo "kun-galgame-patch-api/internal/auth/repository"
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
	"kun-galgame-patch-api/internal/infrastructure/mail"
	"kun-galgame-patch-api/internal/infrastructure/storage"
	messageHandler "kun-galgame-patch-api/internal/message/handler"
	messageRepo "kun-galgame-patch-api/internal/message/repository"
	messageService "kun-galgame-patch-api/internal/message/service"
	"kun-galgame-patch-api/internal/middleware"
	patchHandler "kun-galgame-patch-api/internal/patch/handler"
	patchRepo "kun-galgame-patch-api/internal/patch/repository"
	patchService "kun-galgame-patch-api/internal/patch/service"
	userHandler "kun-galgame-patch-api/internal/user/handler"
	userRepo "kun-galgame-patch-api/internal/user/repository"
	userService "kun-galgame-patch-api/internal/user/service"
	"kun-galgame-patch-api/pkg/config"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type App struct {
	Fiber  *fiber.App
	DB     *gorm.DB
	RDB    *redis.Client
	S3     *storage.S3Client
	Mailer *mail.Mailer
	Config *config.Config

	// Handlers
	AuthHandler     *authHandler.AuthHandler
	PatchHandler    *patchHandler.PatchHandler
	UserHandler     *userHandler.UserHandler
	MessageHandler  *messageHandler.MessageHandler
	AdminHandler    *adminHandler.AdminHandler
	CommonHandler   *common.CommonHandler
	UploadHandler   *uploadPkg.Handler
	ChatHandler     *chatHandler.ChatHandler
	SearchHandler   *searchPkg.Handler
	AboutHandler    *aboutHandler.AboutHandler

	// CronStop is called during graceful shutdown to stop the cron jobs.
	CronStop func()
}

func New(cfg *config.Config) *App {
	// Infrastructure
	db := database.NewPostgres(cfg.Database, cfg.Server.Mode)
	rdb := cache.NewRedis(cfg.Redis)
	s3 := storage.NewS3(cfg.S3)
	mailer := mail.NewMailer(cfg.Mail)
	wiki := galgameClient.New(cfg.GalgameWiki.BaseURL)

	// Auth module
	authRepository := authRepo.New(db)
	authSvc := authService.New(authRepository, rdb, mailer, cfg.OAuth)
	authHdl := authHandler.New(authSvc, rdb, db)

	// Patch module
	patchRepository := patchRepo.New(db)
	patchSvc := patchService.New(patchRepository, rdb, db, s3, wiki)
	patchHdl := patchHandler.New(patchSvc, wiki)

	// User module
	userRepository := userRepo.New(db)
	userSvc := userService.New(userRepository, authSvc, s3)
	userHdl := userHandler.New(userSvc, wiki)

	// Message module
	messageRepository := messageRepo.New(db)
	messageSvc := messageService.New(messageRepository)
	messageHdl := messageHandler.New(messageSvc)

	// Admin module
	adminRepository := adminRepo.New(db)
	adminSvc := adminService.New(adminRepository, rdb)
	adminHdl := adminHandler.New(adminSvc, wiki)

	// Common handler (direct DB access for simple aggregation endpoints)
	commonHdl := common.NewHandler(db, wiki)

	// Upload module (D10: minio-go presigned URL direct upload)
	uploadSvc := uploadPkg.New(s3, db)
	uploadHdl := uploadPkg.NewHandler(uploadSvc)

	// Chat module (D9: REST only, no WebSocket)
	chatRepository := chatRepo.New(db)
	chatSvc := chatService.New(chatRepository)
	chatHdl := chatHandler.New(chatSvc)

	// Search module (D11: delegate to Galgame Wiki Service)
	searchHdl := searchPkg.New(db, wiki)

	// About module (static .mdx posts under cfg.About.PostsDir)
	aboutSvc := aboutService.New(cfg.About.PostsDir)
	aboutHdl := aboutHandler.New(aboutSvc)

	// Cookie mode: use Secure cookies in prod; must be off for HTTP in dev
	middleware.SecureCookies = cfg.Server.Mode == "prod"

	// Fiber app
	app := fiber.New(fiber.Config{
		BodyLimit:    10 * 1024 * 1024, // 10MB
		ErrorHandler: globalErrorHandler,
	})

	app.Use(recover.New())
	app.Use(middleware.CORS(cfg.CORS))

	// Start cron jobs
	cronStop := cronJobs.Start(db, s3)

	slog.Info("Application initialized")

	return &App{
		Fiber:           app,
		DB:              db,
		RDB:             rdb,
		S3:              s3,
		Mailer:          mailer,
		Config:          cfg,
		AuthHandler:     authHdl,
		PatchHandler:    patchHdl,
		UserHandler:     userHdl,
		MessageHandler:  messageHdl,
		AdminHandler:    adminHdl,
		CommonHandler:   commonHdl,
		UploadHandler:   uploadHdl,
		ChatHandler:     chatHdl,
		SearchHandler:   searchHdl,
		AboutHandler:    aboutHdl,
		CronStop:        cronStop,
	}
}

func globalErrorHandler(c *fiber.Ctx, err error) error {
	if appErr, ok := err.(*errors.AppError); ok {
		return response.Error(c, appErr)
	}

	slog.Error("Unhandled error", "error", err, "method", c.Method(), "path", c.Path())
	return response.Error(c, errors.ErrInternal(""))
}
