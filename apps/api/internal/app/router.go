package app

import (
	"time"

	"kun-galgame-patch-api/internal/middleware"
)

func (a *App) RegisterRoutes() {
	// Versioned API prefix, aligned with the frontend apiBase=http://host/api/v1
	api := a.Fiber.Group("/api/v1")

	auth := middleware.Auth(a.RDB, a.Config.OAuth)
	optionalAuth := middleware.OptionalAuth(a.RDB, a.Config.OAuth)
	// OAuth role mapping (see docs/user-migration/02-data-mapping.md §7):
	//   moyu super-admin (legacy role 4) -> "admin"
	//   moyu/kungal admin (legacy role 3) -> "moderator"
	moderatorAuth := middleware.RequireRole("admin", "moderator")
	adminAuth := middleware.RequireRole("admin")

	// Rate limits (Redis-backed per-user/per-IP rolling window)
	checkInRL := middleware.RateLimit(a.RDB, "checkin", 1, 24*time.Hour)

	// ===== Auth Routes =====
	authRoutes := api.Group("/auth")
	authRoutes.Post("/oauth/callback", a.AuthHandler.OAuthCallback)
	authRoutes.Post("/logout", a.AuthHandler.Logout)
	authRoutes.Get("/me", auth, a.AuthHandler.Me)

	// ===== Patch Routes =====
	patchRoutes := api.Group("/patch")

	// Create (after D12, simplified to JSON { vndb_id })
	patchRoutes.Post("/", auth, a.PatchHandler.CreatePatch)

	// Public / optional auth
	patchRoutes.Get("/duplicate", auth, a.PatchHandler.CheckDuplicate)
	patchRoutes.Get("/:id", optionalAuth, a.PatchHandler.GetPatch)
	patchRoutes.Get("/:id/detail", optionalAuth, a.PatchHandler.GetPatchDetail)
	patchRoutes.Get("/:id/comment", optionalAuth, a.PatchHandler.GetComments)
	patchRoutes.Get("/:id/resource", optionalAuth, a.PatchHandler.GetResources)
	patchRoutes.Get("/:id/contributor", a.PatchHandler.GetContributors)
	patchRoutes.Put("/:id/view", a.PatchHandler.IncrementView)
	patchRoutes.Get("/comment/:commentId/markdown", a.PatchHandler.GetCommentMarkdown)

	// Authenticated
	patchRoutes.Put("/:id", auth, a.PatchHandler.UpdatePatch)
	patchRoutes.Delete("/:id", auth, a.PatchHandler.DeletePatch)
	patchRoutes.Post("/:id/comment", auth, a.PatchHandler.CreateComment)
	patchRoutes.Put("/comment/:commentId", auth, a.PatchHandler.UpdateComment)
	patchRoutes.Delete("/comment/:commentId", auth, a.PatchHandler.DeleteComment)
	patchRoutes.Put("/comment/:commentId/like", auth, a.PatchHandler.ToggleCommentLike)
	patchRoutes.Post("/:id/resource", auth, a.PatchHandler.CreateResource)
	patchRoutes.Put("/resource/:resourceId", auth, a.PatchHandler.UpdateResource)
	patchRoutes.Delete("/resource/:resourceId", auth, a.PatchHandler.DeleteResource)
	patchRoutes.Put("/resource/:resourceId/disable", auth, a.PatchHandler.ToggleResourceDisable)
	patchRoutes.Put("/resource/:resourceId/download", a.PatchHandler.IncrementResourceDownload)
	patchRoutes.Put("/resource/:resourceId/like", auth, a.PatchHandler.ToggleResourceLike)
	patchRoutes.Put("/:id/favorite", auth, a.PatchHandler.ToggleFavorite)

	// Galgame metadata edit (proxy to Galgame Wiki PUT /galgame/:gid).
	// Lives on /galgame/:gid to match the Wiki path verbatim, even though the
	// patch.id and galgame.id are aligned. The Wiki Service enforces creator/
	// admin authorization itself — we just forward the user's access_token.
	api.Put("/galgame/:gid", auth, a.PatchHandler.UpdateGalgame)

	// ===== User Routes =====
	//
	// Profile mutations (username/bio/password/email/avatar) live on OAuth and
	// are intentionally absent here. The frontend either redirects to
	// oauth.kungal.com/profile or proxies PATCH /auth/me to OAuth itself.
	userRoutes := api.Group("/user")

	userRoutes.Post("/image", auth, a.UserHandler.UploadImage)
	userRoutes.Post("/check-in", auth, checkInRL, a.UserHandler.CheckIn)
	userRoutes.Get("/search", auth, a.UserHandler.SearchUsers)

	// Public user profiles
	userRoutes.Get("/:uid", optionalAuth, a.UserHandler.GetUserInfo)
	userRoutes.Get("/:uid/floating", a.UserHandler.GetUserFloating)
	userRoutes.Get("/:uid/patch", a.UserHandler.GetUserPatches)
	userRoutes.Get("/:uid/resource", a.UserHandler.GetUserResources)
	userRoutes.Get("/:uid/favorite", a.UserHandler.GetUserFavorites)
	userRoutes.Get("/:uid/comment", a.UserHandler.GetUserComments)
	userRoutes.Get("/:uid/contribute", a.UserHandler.GetUserContributions)
	userRoutes.Get("/:uid/follower", optionalAuth, a.UserHandler.GetFollowers)
	userRoutes.Get("/:uid/following", optionalAuth, a.UserHandler.GetFollowing)

	// Follow/Unfollow
	userRoutes.Put("/:uid/follow", auth, a.UserHandler.Follow)
	userRoutes.Delete("/:uid/follow", auth, a.UserHandler.Unfollow)

	// ===== Message Routes =====
	msgRoutes := api.Group("/message", auth)
	msgRoutes.Get("/", a.MessageHandler.GetMessages)
	msgRoutes.Get("/all", a.MessageHandler.GetAllMessages)
	msgRoutes.Get("/unread", a.MessageHandler.GetUnreadTypes)
	msgRoutes.Post("/", a.MessageHandler.CreateMessage)
	msgRoutes.Put("/read", a.MessageHandler.MarkAsRead)

	// ===== Admin Routes =====
	//
	// User management (/admin/user/*), creator-application approvals
	// (/admin/creator/*), and the creator-only setting were removed when
	// identity moved to OAuth and the creator role was retired.
	adminRoutes := api.Group("/admin", auth, moderatorAuth)

	// Comments
	adminRoutes.Get("/comment", a.AdminHandler.GetComments)
	adminRoutes.Put("/comment/:id", a.AdminHandler.UpdateComment)
	adminRoutes.Delete("/comment/:id", a.AdminHandler.DeleteComment)

	// Resources
	adminRoutes.Get("/resource", a.AdminHandler.GetResources)
	adminRoutes.Put("/resource/:id", a.AdminHandler.UpdateResource)
	adminRoutes.Delete("/resource/:id", a.AdminHandler.DeleteResource)

	// Settings
	adminRoutes.Get("/setting/comment-verify", a.AdminHandler.GetCommentVerify)
	adminRoutes.Put("/setting/comment-verify", adminAuth, a.AdminHandler.SetCommentVerify)
	adminRoutes.Get("/setting/register", a.AdminHandler.GetRegisterDisabled)
	adminRoutes.Put("/setting/register", adminAuth, a.AdminHandler.SetRegisterDisabled)

	// Stats & Logs
	adminRoutes.Get("/stats", a.AdminHandler.GetStats)
	adminRoutes.Get("/stats/sum", a.AdminHandler.GetStatsSum)
	adminRoutes.Get("/log", a.AdminHandler.GetLogs)

	// All patches (admin browse, paginated, optional vndb_id search)
	adminRoutes.Get("/galgame", a.AdminHandler.GetGalgame)

	// D12: "orphan patches" whose galgame is missing in Wiki, for admin manual handling
	adminRoutes.Get("/patch/orphans", a.AdminHandler.GetOrphanPatches)

	// NOTE: /tag/* and /company/* routes are deprecated per D11 (2026-04-21).
	// tag / company metadata is fully owned by the Galgame Wiki Service;
	// the frontend calls Wiki endpoints like /tag /tag/search /official /official/search directly.
	// "Find patches by tag/company" is served via /api/search with tag_ids/official_ids params.

	// ===== Common Routes =====
	api.Get("/home", a.CommonHandler.GetHome)
	api.Get("/home/random", a.PatchHandler.GetRandomPatch)
	api.Get("/galgame", a.CommonHandler.GetGalgameList)
	api.Get("/comment", a.CommonHandler.GetGlobalComments)
	api.Get("/resource", a.CommonHandler.GetGlobalResources)
	api.Get("/resource/:id", a.CommonHandler.GetResourceDetail)

	// Rankings (public).
	api.Get("/ranking/user", a.CommonHandler.GetUserRanking)
	api.Get("/ranking/patch", a.CommonHandler.GetPatchRanking)

	// ===== Chat Routes (D9: REST only, no WebSocket) =====
	chatRoutes := api.Group("/chat", auth)
	chatRoutes.Get("/room", a.ChatHandler.ListRooms)
	chatRoutes.Post("/room", a.ChatHandler.CreateRoom)
	chatRoutes.Post("/room/join", a.ChatHandler.JoinRoom)
	chatRoutes.Get("/room/:link", a.ChatHandler.GetRoomDetail)
	chatRoutes.Get("/room/:link/message", a.ChatHandler.ListMessages)
	chatRoutes.Post("/room/:link/message", a.ChatHandler.CreateMessage)
	chatRoutes.Put("/room/:link/seen", a.ChatHandler.MarkSeen)
	chatRoutes.Put("/message/:id", a.ChatHandler.UpdateMessage)
	chatRoutes.Delete("/message/:id", a.ChatHandler.DeleteMessage)
	chatRoutes.Post("/message/:id/reaction", a.ChatHandler.ToggleReaction)

	// ===== Upload Routes (D10: minio-go presigned URL direct upload) =====
	uploadRoutes := api.Group("/upload", auth)
	uploadRoutes.Post("/small/init", a.UploadHandler.InitSmall)
	uploadRoutes.Post("/small/complete", a.UploadHandler.CompleteSmall)
	uploadRoutes.Post("/multipart/init", a.UploadHandler.InitMultipart)
	uploadRoutes.Post("/multipart/complete", a.UploadHandler.CompleteMultipart)
	uploadRoutes.Post("/multipart/abort", a.UploadHandler.AbortMultipart)

	// Full-text search (Meilisearch)
	api.Post("/search", a.SearchHandler.Search)

	// External APIs
	api.Get("/hikari", a.CommonHandler.GetHikari)
	api.Get("/moyu/patch/has-patch", a.CommonHandler.GetMoyuHasPatch)

	// About / docs (static .mdx posts).
	api.Get("/about/posts", a.AboutHandler.ListPosts)
	api.Get("/about/post", a.AboutHandler.GetPost)
}
