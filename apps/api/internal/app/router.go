package app

import (
	"time"

	"kun-galgame-patch-api/internal/middleware"

	"github.com/gofiber/fiber/v3"
)

func (a *App) RegisterRoutes() {
	a.Fiber.Get("/healthz", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	api := a.Fiber.Group("/api/v1")

	auth := middleware.Auth(a.RDB, a.Config.OAuth)
	optionalAuth := middleware.OptionalAuth(a.RDB, a.Config.OAuth)
	moderatorAuth := middleware.RequireRole(middleware.ModeratorRoles...)
	adminAuth := middleware.RequireRole(middleware.SuperAdminRoles...)

	api.Post("/trust/callback", a.TrustHandler.Callback)

	reportRoutes := api.Group("/report", auth)
	reportRoutes.Get("/reasons", a.TrustHandler.GetReasons)
	reportRoutes.Post("/submit", a.TrustHandler.SubmitReport)

	authRoutes := api.Group("/auth")
	authRoutes.Post("/oauth/callback", a.AuthHandler.OAuthCallback)
	authRoutes.Get("/oauth/ecosystem", a.AuthHandler.Ecosystem)
	authRoutes.Post("/logout", a.AuthHandler.Logout)
	authRoutes.Get("/me", auth, a.AuthHandler.Me)
	authRoutes.Patch("/me", auth, a.AuthHandler.UpdateMe)
	authRoutes.Post("/me/avatar", auth, a.AuthHandler.UploadAvatar)

	patchRoutes := api.Group("/patch")

	patchRoutes.Post("/", auth, a.PatchHandler.CreatePatch)

	patchRoutes.Get("/:id", optionalAuth, a.PatchHandler.GetPatch)
	patchRoutes.Get("/:id/detail", optionalAuth, a.PatchHandler.GetPatchDetail)
	patchRoutes.Get("/:id/comment", optionalAuth, a.PatchHandler.GetComments)
	patchRoutes.Get("/:id/resource", optionalAuth, a.PatchHandler.GetResources)
	patchRoutes.Get("/:id/contributor", a.PatchHandler.GetContributors)
	patchRoutes.Put("/:id/view", a.PatchHandler.IncrementView)
	patchRoutes.Get("/comment/:commentId/markdown", a.PatchHandler.GetCommentMarkdown)
	patchRoutes.Get("/comment/:commentId/locate", optionalAuth, a.PatchHandler.LocateComment)
	patchRoutes.Get("/resource/:resourceId/comment", optionalAuth, a.PatchHandler.GetResourceComments)

	patchRoutes.Put("/:id", auth, a.PatchHandler.UpdatePatch)
	patchRoutes.Delete("/:id", auth, a.PatchHandler.DeletePatch)
	patchRoutes.Post("/:id/comment", auth, a.PatchHandler.CreateComment)
	patchRoutes.Put("/comment/:commentId", auth, a.PatchHandler.UpdateComment)
	patchRoutes.Delete("/comment/:commentId", auth, a.PatchHandler.DeleteComment)
	patchRoutes.Put("/comment/:commentId/like", auth, a.PatchHandler.ToggleCommentLike)
	patchRoutes.Post("/resource/:resourceId/comment", auth, a.PatchHandler.CreateResourceComment)
	patchRoutes.Post("/:id/resource", auth, a.PatchHandler.CreateResource)
	patchRoutes.Put("/resource/:resourceId", auth, a.PatchHandler.UpdateResource)
	patchRoutes.Delete("/resource/:resourceId", auth, a.PatchHandler.DeleteResource)
	patchRoutes.Put("/resource/:resourceId/disable", auth, a.PatchHandler.ToggleResourceDisable)
	patchRoutes.Get(
		"/resource/:resourceId/link",
		optionalAuth,
		middleware.RateLimit(a.RDB, "resource-link", 30, time.Minute),
		a.PatchHandler.GetResourceDownloadInfo,
	)
	patchRoutes.Put(
		"/resource/:resourceId/download",
		optionalAuth,
		middleware.RateLimit(a.RDB, "resource-download", 60, time.Minute),
		a.PatchHandler.IncrementResourceDownload,
	)
	patchRoutes.Put("/resource/:resourceId/like", auth, a.PatchHandler.ToggleResourceLike)
	patchRoutes.Put("/resource/:resourceId/favorite", auth, a.PatchHandler.ToggleResourceFavorite)
	patchRoutes.Get(
		"/resource/:resourceId/revisions",
		middleware.RateLimit(a.RDB, "resource-revisions", 60, time.Minute),
		a.PatchHandler.GetResourceRevisions,
	)
	patchRoutes.Put("/:id/favorite", auth, a.PatchHandler.ToggleFavorite)

	patchRoutes.Get("/:id/catalog-edit", auth, a.PatchHandler.CatalogEditBootstrap)
	patchRoutes.Post("/:id/catalog-edit", auth, a.PatchHandler.CatalogEditSubmit)
	patchRoutes.Get("/:id/catalog-edit/proposals", auth, a.PatchHandler.CatalogEditProposals)
	api.Post("/catalog-proposal/:id/withdraw", auth, a.PatchHandler.CatalogProposalWithdraw)

	api.Get("/galgame/calendar", optionalAuth, a.CommonHandler.GetGalgameCalendar)
	api.Get(
		"/galgame/character/:id",
		middleware.RateLimit(a.RDB, "galgame-entity", 120, time.Minute),
		a.CommonHandler.GetGalgameCharacter,
	)
	api.Get(
		"/galgame/staff/:id",
		middleware.RateLimit(a.RDB, "galgame-entity", 120, time.Minute),
		a.CommonHandler.GetGalgameStaff,
	)
	api.Get(
		"/galgame/series/:id",
		middleware.RateLimit(a.RDB, "galgame-entity", 120, time.Minute),
		a.CommonHandler.GetGalgameSeries,
	)
	api.Get("/galgame/mine", auth, a.PatchHandler.ListMyGalgames)
	api.Get("/galgame/search/publish", auth, a.PatchHandler.SearchGalgameForPublish)
	api.Post("/galgame/submit", auth, a.PatchHandler.SubmitGalgame)
	api.Post("/galgame/:gid/claim", auth, a.PatchHandler.ClaimGalgame)
	api.Delete("/galgame/:gid", auth, a.PatchHandler.WithdrawGalgameSubmission)

	userRoutes := api.Group("/user")

	userRoutes.Post("/check-in", auth, a.UserHandler.CheckIn)
	userRoutes.Get("/search", auth, a.UserHandler.SearchUsers)
	userRoutes.Get("/moemoepoint/log", auth, a.UserHandler.GetMoemoepointLog)

	userRoutes.Get("/creator/status", auth, a.UserHandler.CreatorStatus)
	userRoutes.Post("/creator/apply", auth, a.UserHandler.CreatorApply)

	userRoutes.Get("/:id", optionalAuth, a.UserHandler.GetUserInfo)
	userRoutes.Get("/:id/floating", a.UserHandler.GetUserFloating)
	userRoutes.Get("/:id/patch", a.UserHandler.GetUserPatches)
	userRoutes.Get("/:id/resource", a.UserHandler.GetUserResources)
	userRoutes.Get("/:id/favorite", a.UserHandler.GetUserFavorites)
	userRoutes.Get("/:id/comment", a.UserHandler.GetUserComments)
	userRoutes.Get("/:id/contribute", a.UserHandler.GetUserContributions)
	userRoutes.Get("/:id/follower", optionalAuth, a.UserHandler.GetFollowers)
	userRoutes.Get("/:id/following", optionalAuth, a.UserHandler.GetFollowing)

	userRoutes.Put("/:id/follow", auth, a.UserHandler.Follow)
	userRoutes.Delete("/:id/follow", auth, a.UserHandler.Unfollow)

	msgRoutes := api.Group("/message", auth)
	msgRoutes.Get("/", a.MessageHandler.GetMessages)
	msgRoutes.Get("/all", a.MessageHandler.GetAllMessages)
	msgRoutes.Get("/unread", a.MessageHandler.GetUnreadTypes)
	msgRoutes.Put("/read", a.MessageHandler.MarkAsRead)

	adminRoutes := api.Group("/admin", auth, moderatorAuth)

	adminRoutes.Get("/comment", a.AdminHandler.GetComments)
	adminRoutes.Put("/comment/:id", a.AdminHandler.UpdateComment)
	adminRoutes.Delete("/comment/:id", a.AdminHandler.DeleteComment)
	adminRoutes.Put("/comment/:id/approve", a.PatchHandler.ApproveComment)

	adminRoutes.Get("/resource", a.AdminHandler.GetResources)
	adminRoutes.Put("/resource/:id", a.AdminHandler.UpdateResource)
	adminRoutes.Delete("/resource/:id", a.AdminHandler.DeleteResource)
	adminRoutes.Get("/resource/:id/history", a.AdminHandler.GetResourceFileHistory)

	adminRoutes.Get("/user/:id/purge-preview", adminAuth, a.AdminHandler.GetUserPurgePreview)
	adminRoutes.Post("/user/:id/purge", adminAuth, a.AdminHandler.PurgeUser)

	adminRoutes.Get("/setting/comment-verify", a.AdminHandler.GetCommentVerify)
	adminRoutes.Put("/setting/comment-verify", adminAuth, a.AdminHandler.SetCommentVerify)
	adminRoutes.Get("/setting/creator-only", a.AdminHandler.GetCreatorOnly)
	adminRoutes.Put("/setting/creator-only", adminAuth, a.AdminHandler.SetCreatorOnly)

	adminRoutes.Get("/trust/review-items", a.TrustHandler.ListReviewItems)
	adminRoutes.Get("/trust/review-items/:id", a.TrustHandler.GetReviewItem)
	adminRoutes.Post("/trust/review-items/:id/claim", a.TrustHandler.ClaimReviewItem)
	adminRoutes.Post("/trust/review-items/:id/decide", a.TrustHandler.DecideReviewItem)

	adminRoutes.Get("/stats", a.AdminHandler.GetStats)
	adminRoutes.Get("/stats/sum", a.AdminHandler.GetStatsSum)
	adminRoutes.Get("/log", a.AdminHandler.GetLogs)

	adminRoutes.Get("/galgame", a.AdminHandler.GetGalgame)

	adminRoutes.Get("/patch/orphans", a.AdminHandler.GetOrphanPatches)

	adminRoutes.Get("/doc", a.DocHandler.AdminListPosts)
	adminRoutes.Get("/doc/:id", a.DocHandler.AdminGetPost)
	adminRoutes.Post("/doc", a.DocHandler.CreatePost)
	adminRoutes.Put("/doc/:id", a.DocHandler.UpdatePost)
	adminRoutes.Delete("/doc/:id", a.DocHandler.DeletePost)

	api.Get("/tag/:name", a.PatchHandler.GalgameTaxonomyDetailProxy)
	api.Get("/official/:name", a.PatchHandler.GalgameTaxonomyDetailProxy)

	api.Get("/taxonomy/resolve/:kind/:id", a.PatchHandler.ResolveTaxonomyID)

	api.Get("/home", a.CommonHandler.GetHome)
	api.Get("/home/random", a.PatchHandler.GetRandomPatch)
	api.Get("/galgame", a.CommonHandler.GetGalgameList)
	api.Get("/comment", a.CommonHandler.GetGlobalComments)
	api.Get("/resource", a.CommonHandler.GetGlobalResources)
	api.Get("/resource/:id",
		optionalAuth,
		middleware.RateLimit(a.RDB, "resource-detail", 60, time.Minute),
		a.CommonHandler.GetResourceDetail,
	)

	api.Get("/ranking/user", a.CommonHandler.GetUserRanking)
	api.Get("/ranking/patch", a.CommonHandler.GetPatchRanking)

	chatRoutes := api.Group("/chat", auth)
	chatRoutes.Get("/room", a.ChatHandler.ListRooms)
	chatRoutes.Post("/room", a.ChatHandler.CreateRoom)
	chatRoutes.Post("/room/join", a.ChatHandler.JoinRoom)
	chatRoutes.Post("/room/private", a.ChatHandler.StartPrivate)
	chatRoutes.Get("/room/:link", a.ChatHandler.GetRoomDetail)
	chatRoutes.Get("/room/:link/message", a.ChatHandler.ListMessages)
	chatRoutes.Post("/room/:link/message", a.ChatHandler.CreateMessage)
	chatRoutes.Put("/room/:link/seen", a.ChatHandler.MarkSeen)
	chatRoutes.Put("/message/:id", a.ChatHandler.UpdateMessage)
	chatRoutes.Delete("/message/:id", a.ChatHandler.DeleteMessage)
	chatRoutes.Post("/message/:id/reaction", a.ChatHandler.ToggleReaction)

	uploadRoutes := api.Group("/upload", auth)
	uploadRoutes.Post("/init", a.UploadHandler.Init)
	uploadRoutes.Post("/complete", a.UploadHandler.Complete)
	uploadRoutes.Post("/resume", a.UploadHandler.Resume)
	uploadRoutes.Post("/abort", a.UploadHandler.Abort)
	uploadRoutes.Post("/image-service", a.UploadHandler.UploadImageService)

	api.Post("/search", a.SearchHandler.Search)

	api.Use("/hikari", middleware.HikariCORS())
	api.Get("/hikari", middleware.RateLimit(a.RDB, "hikari", 10000, time.Minute), a.CommonHandler.GetHikari)
	api.Get("/moyu/patch/has-patch", a.CommonHandler.GetMoyuHasPatch)

	api.Get("/doc/posts", a.DocHandler.ListPosts)
	api.Get("/doc/pinned", a.DocHandler.ListPinnedPosts)
	api.Get("/doc/post", a.DocHandler.GetPost)
	api.Put("/doc/view", a.DocHandler.IncrementView)
}
