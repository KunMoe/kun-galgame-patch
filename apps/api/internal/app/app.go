package app

import (
	"context"
	"log/slog"
	"time"

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
	uploadPkg "kun-galgame-patch-api/internal/common/upload"
	docHandler "kun-galgame-patch-api/internal/doc/handler"
	docRepository "kun-galgame-patch-api/internal/doc/repository"
	docService "kun-galgame-patch-api/internal/doc/service"
	"kun-galgame-patch-api/internal/face"
	faceHandler "kun-galgame-patch-api/internal/face/handler"
	faceRepo "kun-galgame-patch-api/internal/face/repository"
	faceService "kun-galgame-patch-api/internal/face/service"
	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/internal/infrastructure/cache"
	cronJobs "kun-galgame-patch-api/internal/infrastructure/cron"
	"kun-galgame-patch-api/internal/infrastructure/database"
	"kun-galgame-patch-api/internal/infrastructure/markdown"
	"kun-galgame-patch-api/internal/infrastructure/storelink"
	messageHandler "kun-galgame-patch-api/internal/message/handler"
	messageRepo "kun-galgame-patch-api/internal/message/repository"
	messageService "kun-galgame-patch-api/internal/message/service"
	"kun-galgame-patch-api/internal/middleware"
	patchHandler "kun-galgame-patch-api/internal/patch/handler"
	patchRepo "kun-galgame-patch-api/internal/patch/repository"
	patchService "kun-galgame-patch-api/internal/patch/service"
	settingService "kun-galgame-patch-api/internal/setting/service"
	"kun-galgame-patch-api/internal/trust/enforce"
	trustHandler "kun-galgame-patch-api/internal/trust/handler"
	trustService "kun-galgame-patch-api/internal/trust/service"
	userHandler "kun-galgame-patch-api/internal/user/handler"
	userRepo "kun-galgame-patch-api/internal/user/repository"
	userService "kun-galgame-patch-api/internal/user/service"
	"kun-galgame-patch-api/pkg/artifactclient"
	"kun-galgame-patch-api/pkg/config"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/imageclient"
	"kun-galgame-patch-api/pkg/moemoepoint"
	"kun-galgame-patch-api/pkg/problem"
	"kun-galgame-patch-api/pkg/response"
	"kun-galgame-patch-api/pkg/storeclient"
	"kun-galgame-patch-api/pkg/trustclient"
	"kun-galgame-patch-api/pkg/userclient"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type App struct {
	Fiber      *fiber.App
	DB         *gorm.DB
	RDB        *redis.Client
	UserClient *userclient.Client
	Config     *config.Config

	AuthHandler    *authHandler.AuthHandler
	PatchHandler   *patchHandler.PatchHandler
	UserHandler    *userHandler.UserHandler
	MessageHandler *messageHandler.MessageHandler
	AdminHandler   *adminHandler.AdminHandler
	CommonHandler  *common.CommonHandler
	FaceHandler    *faceHandler.Handler
	UploadHandler  *uploadPkg.Handler
	ChatHandler    *chatHandler.ChatHandler
	DocHandler     *docHandler.DocHandler
	TrustHandler   *trustHandler.TrustHandler

	CronStop func()
}

func validateConfig(cfg *config.Config) {
	if cfg.NextMoeAPI.BaseURL != "" && cfg.NextMoeAPI.APIKey == "" {
		panic("KUN_NEXTMOE_API_BASE is set but KUN_NEXTMOE_API_KEY is empty: " +
			"the galgame read face requires an internal-tier API key " +
			"(the legacy /api rollback valve was retired in open-API phase 2 wave 05)")
	}
}

func New(cfg *config.Config) *App {
	validateConfig(cfg)

	db := database.NewPostgres(cfg.Database, cfg.Server.Mode)
	rdb := cache.NewRedis(cfg.Redis)
	galgame := galgameClient.NewWithKey(cfg.NextMoeAPI.BaseURL, cfg.NextMoeAPI.APIKey).WithRedis(rdb)

	usrCli := userclient.New(userclient.Config{
		BaseURL:      cfg.OAuth.ServerURL,
		ClientID:     cfg.OAuth.ClientID,
		ClientSecret: cfg.OAuth.ClientSecret,
	})

	mpClient := moemoepoint.New(moemoepoint.Config{
		BaseURL:      cfg.OAuth.ServerURL,
		ClientID:     cfg.OAuth.ClientID,
		ClientSecret: cfg.OAuth.ClientSecret,
	})
	mpAwarder := moemoepoint.NewAwarder(mpClient, db)

	authRepository := authRepo.New(db)
	authSvc := authService.New(authRepository, rdb, cfg.OAuth)
	authHdl := authHandler.New(authSvc, rdb, db, usrCli)

	settingSvc := settingService.New(db)

	adminRepository := adminRepo.New(db)

	artCfg := cfg.Artifact
	if artCfg.ClientID == "" {
		artCfg.ClientID = cfg.OAuth.ClientID
	}
	if artCfg.ClientSecret == "" {
		artCfg.ClientSecret = cfg.OAuth.ClientSecret
	}
	artCli := artifactclient.New(artifactclient.Config{
		BaseURL:      artCfg.BaseURL,
		ClientID:     artCfg.ClientID,
		ClientSecret: artCfg.ClientSecret,
	})

	if galgame.V2().Configured() {
		slog.Info("catalog v2 client configured", "base_url", cfg.NextMoeAPI.BaseURL)
	} else {
		slog.Warn("catalog v2 client NOT configured; the claim lifecycle and its cron will not run — set KUN_NEXTMOE_API_BASE + KUN_NEXTMOE_API_KEY")
	}

	var storeCli *storeclient.Client
	if cfg.Dlsite.StoreConfigured() {
		storeCli = storeclient.New(storeclient.Config{
			BaseURL: cfg.Dlsite.StoreAPIBase,
			APIKey:  cfg.Dlsite.StoreAPIKey,
		})
	}
	storeLinks := storelink.New(storelink.Options{
		DB:           db,
		Client:       storeCli,
		LinkTemplate: cfg.Dlsite.LinkTemplate,
		StaticCoupon: cfg.Dlsite.CouponURL,
	})
	if cfg.Dlsite.Configured() || storeLinks.Configured() {
		slog.Info("dlsite 正版购买入口已启用",
			"short_links", storeLinks.Configured(),
			"template_fallback", cfg.Dlsite.Configured(),
			"static_coupon", cfg.Dlsite.CouponURL != "")
	} else {
		slog.Info("dlsite 正版购买入口未启用 (KUN_DLSITE_LINK_TEMPLATE 与 KUN_STORE_API_KEY 均为空); 游戏页不渲染购买按钮")
	}
	storeStop := storeLinks.Start()

	patchRepository := patchRepo.New(db)
	patchSvc := patchService.New(patchRepository, settingSvc, db, artCli, galgame, usrCli, mpAwarder, adminRepository)
	patchHdl := patchHandler.New(patchSvc, galgame, usrCli, storeLinks)

	userRepository := userRepo.New(db)
	userSvc := userService.New(userRepository, usrCli, galgame, db, mpAwarder)
	userHdl := userHandler.New(userSvc, galgame, usrCli)

	messageRepository := messageRepo.New(db)
	messageSvc := messageService.New(messageRepository)
	messageHdl := messageHandler.New(messageSvc, usrCli, galgame)

	adminSvc := adminService.New(adminRepository, rdb, settingSvc, patchSvc, galgame)
	adminHdl := adminHandler.New(adminSvc, galgame, usrCli)

	trustCli := trustclient.New(trustclient.Config{
		BaseURL:      cfg.Trust.BaseURL,
		ClientID:     cfg.OAuth.ClientID,
		ClientSecret: cfg.OAuth.ClientSecret,
	})
	if trustCli.Configured() {
		slog.Info("trust service client configured", "base_url", cfg.Trust.BaseURL)
	} else {
		slog.Warn("trust service client NOT configured; reporting returns 未启用 — set KUN_TRUST_BASE_URL + OAuth creds")
	}
	trustRegistry := enforce.Registry{
		"patch_comment": {
			Hide: func(_ context.Context, id int) error {
				return patchRepository.UpdateCommentStatus(id, 1)
			},
			Remove: func(_ context.Context, id int) error {
				return patchSvc.DeleteComment(id, 0, true, "内容违规（审核处置）")
			},
			AuthorID: func(_ context.Context, id int) (int, error) {
				cmt, err := patchRepository.GetCommentByID(id)
				if err != nil {
					return 0, nil
				}
				return cmt.UserID, nil
			},
		},
		"patch_resource": {
			Hide: func(_ context.Context, id int) error {
				return patchRepository.SetResourceStatus(id, 2)
			},
			Remove: func(_ context.Context, id int) error {
				return patchSvc.DeleteResource(id, 0, true, "内容违规（审核处置）")
			},
			Restore: func(_ context.Context, id int) error {
				return patchRepository.RestoreResourceFromModHide(id)
			},
			AuthorID: func(_ context.Context, id int) (int, error) {
				res, err := patchRepository.GetResourceByID(id)
				if err != nil {
					return 0, nil
				}
				return res.UserID, nil
			},
		},
	}
	trustEnforce := enforce.NewService(db, trustRegistry, nil)
	trustHdl := trustHandler.NewTrustHandler(
		trustService.NewTrustService(trustCli, cfg.Trust.Site),
		trustEnforce,
		cfg.Trust.CallbackSecret,
	)

	uploadSvc := uploadPkg.New(artCli, db, rdb)
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

	commonHdl := common.NewHandler(db, galgame, usrCli, artCli, imgCli)
	faceHdl := faceHandler.New(faceService.New(faceRepo.New(db), usrCli, imgCli, cfg.Site.BaseURL))
	uploadHdl := uploadPkg.NewHandler(uploadSvc, imgCli)

	markdown.SetContentImageResolver(imgCli.MainURL)

	contentImageMeta := imgCli.NewMetaResolver(3 * time.Second)
	markdown.SetContentImageMetaResolver(func(hashes []string) map[string]markdown.ImageMeta {
		got := contentImageMeta.Resolve(hashes)
		out := make(map[string]markdown.ImageMeta, len(got))
		for h, m := range got {
			out[h] = markdown.ImageMeta{Width: m.Width, Height: m.Height, Thumbhash: m.Thumbhash}
		}
		return out
	})

	chatRepository := chatRepo.New(db)
	chatSvc := chatService.New(chatRepository)
	chatHdl := chatHandler.New(chatSvc, usrCli)

	docRepo := docRepository.New(db)
	docSvc := docService.New(docRepo, imgCli, usrCli)
	docHdl := docHandler.New(docSvc)

	middleware.SecureCookies = cfg.Server.Mode == "prod"

	app := fiber.New(fiber.Config{
		BodyLimit:      10 * 1024 * 1024,
		ReadBufferSize: 32 * 1024,
		ErrorHandler:   globalErrorHandler,
	})

	app.Use(recover.New())
	app.Use(middleware.CORS(cfg.CORS))

	cronStop := cronJobs.Start(db, galgame, mpClient, imgCli)
	stopBackground := func() {
		cronStop()
		storeStop()
	}

	slog.Info("Application initialized")

	return &App{
		Fiber:          app,
		DB:             db,
		RDB:            rdb,
		UserClient:     usrCli,
		Config:         cfg,
		AuthHandler:    authHdl,
		PatchHandler:   patchHdl,
		UserHandler:    userHdl,
		MessageHandler: messageHdl,
		AdminHandler:   adminHdl,
		CommonHandler:  commonHdl,
		FaceHandler:    faceHdl,
		UploadHandler:  uploadHdl,
		ChatHandler:    chatHdl,
		DocHandler:     docHdl,
		TrustHandler:   trustHdl,
		CronStop:       stopBackground,
	}
}

// globalErrorHandler speaks whichever error language the path belongs to.
// Without the face branch an unrouted /v2/moyu path would answer in the site's
// house envelope -- exactly the second error dialect the face exists to avoid.
func globalErrorHandler(c fiber.Ctx, err error) error {
	if prob, ok := err.(*problem.Problem); ok {
		return problem.Write(c, prob)
	}
	if appErr, ok := err.(*errors.AppError); ok {
		if face.IsPath(c.Path()) {
			return problem.Write(c, faceProblem(appErr.HTTPStatus, appErr.Message))
		}
		return response.Error(c, appErr)
	}
	if fe, ok := err.(*fiber.Error); ok && face.IsPath(c.Path()) {
		return problem.Write(c, faceProblem(fe.Code, fe.Message))
	}

	slog.Error("Unhandled error", "error", err, "method", c.Method(), "path", c.Path())
	if face.IsPath(c.Path()) {
		return problem.WriteCode(c, problem.CodeInternalError, "")
	}
	return response.Error(c, errors.ErrInternal(""))
}

func faceProblem(status int, detail string) *problem.Problem {
	switch status {
	case fiber.StatusNotFound:
		return problem.New(problem.CodeNotFound, detail)
	case fiber.StatusMethodNotAllowed:
		return problem.New(problem.CodeMethodNotAllowed, detail)
	case fiber.StatusBadRequest:
		return problem.New(problem.CodeInvalidParameter, detail)
	case fiber.StatusServiceUnavailable:
		return problem.New(problem.CodeServiceUnavailable, detail)
	default:
		return problem.New(problem.CodeInternalError, detail)
	}
}
