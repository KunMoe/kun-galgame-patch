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

	// NOTE: We do NOT rate-limit /user/check-in via Redis.
	// "Once per day" is enforced by user.daily_check_in plus the daily cron
	// reset at 00:00 (see internal/infrastructure/cron/cron.go). A separate
	// rolling-24h Redis limiter would create a dead window every day after
	// midnight when the DB flag is already cleared but the Redis key has
	// not yet expired, causing spurious 429s. The DB flag is the single
	// source of truth -- the service rejects with "already checked in today".

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
	// MOYU-PR8 (M6) — rate-limit the link-reveal endpoint to deter mass
	// scraping. Original plan §4.6 proposed size-scaled presigned-URL TTL,
	// but the current architecture serves downloads as public S3 URLs (no
	// presigned URL involved), so TTL scaling doesn't apply. The actual
	// abuse surface is bulk-fetching `/link` to harvest URLs — capping at
	// 30/min per uid (or per IP when anonymous) keeps legitimate browsing
	// (one user opens 10 patch resource pages = ~10-30 calls) untouched
	// while breaking automated scraping. Returns 429 on overflow.
	patchRoutes.Get(
		"/resource/:resourceId/link",
		middleware.RateLimit(a.RDB, "resource-link", 30, time.Minute),
		a.PatchHandler.GetResourceDownloadInfo,
	)
	patchRoutes.Put("/resource/:resourceId/download", a.PatchHandler.IncrementResourceDownload)
	patchRoutes.Put("/resource/:resourceId/like", auth, a.PatchHandler.ToggleResourceLike)
	patchRoutes.Put("/:id/favorite", auth, a.PatchHandler.ToggleFavorite)

	// Galgame metadata edit (proxy to Galgame Wiki PUT /galgame/:gid).
	// Lives on /galgame/:gid to match the Wiki path verbatim, even though the
	// patch.id and galgame.id are aligned. The Wiki Service enforces creator/
	// admin authorization itself — we just forward the user's access_token.
	api.Put("/galgame/:gid", auth, a.PatchHandler.UpdateGalgame)

	// ===== Wiki submission proxies (docs/galgame_wiki/07-submission.md) =====
	//
	// User-facing endpoints for the new publish-galgame flow. Each one
	// forwards the user's OAuth access_token to Wiki and surfaces Wiki's
	// business errors (20003 / 20004 / 20006 / 20007 / 20008 / 20009) verbatim.
	//
	// IMPORTANT: order matters here. /galgame/mine, /galgame/submit,
	// /galgame/search/publish, /galgame/messages/* must be registered BEFORE
	// the parameterized /galgame/:gid routes below so Fiber doesn't match
	// "mine"/"submit"/etc. as a :gid value.
	api.Get("/galgame/mine", auth, a.PatchHandler.ListMyGalgames)
	api.Get("/galgame/search/publish", auth, a.PatchHandler.SearchGalgameForPublish)
	api.Get("/galgame/messages/mine", auth, a.PatchHandler.GetMyWikiMessages)
	api.Get("/galgame/messages/read-state", auth, a.PatchHandler.GetWikiMessagesReadState)
	api.Put("/galgame/messages/read-state", auth, a.PatchHandler.UpdateWikiMessagesReadState)
	api.Post("/galgame/submit", auth, a.PatchHandler.SubmitGalgame)
	api.Post("/galgame/:gid/claim", auth, a.PatchHandler.ClaimGalgame)
	api.Patch("/galgame/:gid", auth, a.PatchHandler.PatchGalgameDraft)
	api.Delete("/galgame/:gid", auth, a.PatchHandler.DeleteGalgameDraft)

	// ===== Galgame editing surface (handbook §15, MANDATORY full proxy) =====
	//
	// docs/galgame_wiki/00-handbook-for-downstream.md §15 REVOKES the old
	// "wiki-only, downstream doesn't implement editing" stance: moyu must now
	// fully proxy Wiki's revision / PR / relation editing (back end + UI).
	// Every route below is a verbatim pass-through (a.PatchHandler.WikiEditProxy
	// / WikiPRSubmit) that mirrors the Wiki path 1:1. Reads use optionalAuth
	// (token forwarded only if logged in); writes use auth so a Bearer exists.
	// Wiki enforces creator/admin; we forward its code+message verbatim.
	//
	// Registered AFTER the literal /galgame/{mine,submit,search,messages,:gid}
	// routes above so Fiber's order-based matching keeps them intact.
	api.Get("/galgame/:gid/revisions", optionalAuth, a.PatchHandler.WikiEditProxy)
	api.Get("/galgame/:gid/revisions/:rev", optionalAuth, a.PatchHandler.WikiEditProxy)
	api.Get("/galgame/:gid/revisions/:rev/diff", optionalAuth, a.PatchHandler.WikiEditProxy)
	api.Post("/galgame/:gid/revert", auth, a.PatchHandler.WikiEditProxy)
	api.Get("/galgame/:gid/prs", optionalAuth, a.PatchHandler.WikiEditProxy)
	api.Get("/galgame/:gid/prs/:prid", optionalAuth, a.PatchHandler.WikiEditProxy)
	api.Post("/galgame/:gid/prs", auth, a.PatchHandler.WikiPRSubmit)
	api.Put("/galgame/:gid/prs/:prid/merge", auth, a.PatchHandler.WikiEditProxy)
	api.Put("/galgame/:gid/prs/:prid/decline", auth, a.PatchHandler.WikiEditProxy)
	api.Get("/galgame/:gid/links", optionalAuth, a.PatchHandler.WikiEditProxy)
	api.Post("/galgame/:gid/links", auth, a.PatchHandler.WikiEditProxy)
	api.Delete("/galgame/:gid/links", auth, a.PatchHandler.WikiEditProxy)
	api.Get("/galgame/:gid/aliases", optionalAuth, a.PatchHandler.WikiEditProxy)
	api.Post("/galgame/:gid/aliases", auth, a.PatchHandler.WikiEditProxy)
	api.Delete("/galgame/:gid/aliases", auth, a.PatchHandler.WikiEditProxy)
	api.Get("/galgame/:gid/contributors", optionalAuth, a.PatchHandler.WikiEditProxy)
	api.Delete("/galgame/:gid/contributors/:uid", auth, a.PatchHandler.WikiEditProxy)

	// ===== User Routes =====
	//
	// Profile mutations (username/bio/password/email/avatar) live on OAuth and
	// are intentionally absent here. The frontend either redirects to
	// oauth.kungal.com/profile or proxies PATCH /auth/me to OAuth itself.
	userRoutes := api.Group("/user")

	userRoutes.Post("/image", auth, a.UserHandler.UploadImage)
	userRoutes.Post("/check-in", auth, a.UserHandler.CheckIn)
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
	// MOYU-PR5 / M3 — append-only file-replacement audit trail for one resource.
	adminRoutes.Get("/resource/:id/history", a.AdminHandler.GetResourceFileHistory)

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

	// ===== Galgame taxonomy proxy (handbook §15, MANDATORY full proxy) =====
	//
	// SUPERSEDES the old D11 note ("frontend calls Wiki /tag /official directly,
	// downstream skips tag/company"): handbook §15 now REQUIRES moyu to fully
	// proxy + build UI for tag / official / engine / series CRUD — including the
	// new POST creators (any logged-in user may add a tag/official/engine for an
	// original/doujin work VNDB lacks; same permission model as POST /series).
	// Pure pass-through; Wiki enforces role (GET public; POST any logged-in
	// user; PUT/DELETE admin/moderator) and we forward its code+message.
	// Literal sub-paths are registered before :name/:id params so Fiber's
	// order-based matcher resolves /tag/search before /tag/:name, etc.
	api.Get("/tag", a.PatchHandler.WikiEditProxy)
	api.Get("/tag/search", a.PatchHandler.WikiEditProxy)
	api.Get("/tag/multi", a.PatchHandler.WikiEditProxy)
	api.Post("/tag", auth, a.PatchHandler.WikiEditProxy)
	api.Put("/tag", auth, a.PatchHandler.WikiEditProxy)
	api.Delete("/tag/:id", auth, a.PatchHandler.WikiEditProxy)
	api.Get("/tag/:name", a.PatchHandler.WikiEditProxy)

	api.Get("/official", a.PatchHandler.WikiEditProxy)
	api.Get("/official/search", a.PatchHandler.WikiEditProxy)
	api.Post("/official", auth, a.PatchHandler.WikiEditProxy)
	api.Put("/official", auth, a.PatchHandler.WikiEditProxy)
	api.Delete("/official/:id", auth, a.PatchHandler.WikiEditProxy)
	api.Get("/official/:name", a.PatchHandler.WikiEditProxy)

	api.Get("/engine", a.PatchHandler.WikiEditProxy)
	api.Post("/engine", auth, a.PatchHandler.WikiEditProxy)
	api.Put("/engine", auth, a.PatchHandler.WikiEditProxy)
	api.Delete("/engine/:id", auth, a.PatchHandler.WikiEditProxy)
	api.Get("/engine/:name", a.PatchHandler.WikiEditProxy)

	api.Get("/series", a.PatchHandler.WikiEditProxy)
	api.Get("/series/search", a.PatchHandler.WikiEditProxy)
	api.Post("/series/modal", auth, a.PatchHandler.WikiEditProxy)
	api.Post("/series", auth, a.PatchHandler.WikiEditProxy)
	api.Put("/series/:id", auth, a.PatchHandler.WikiEditProxy)
	api.Delete("/series/:id", auth, a.PatchHandler.WikiEditProxy)
	api.Get("/series/:id", a.PatchHandler.WikiEditProxy)

	// ===== Taxonomy 修订历史 / 回滚（W3 / Wiki U3 PR4，12 条）=====
	// 4 实体 × 3 端点；都是纯透传到 Wiki，鉴权 Wiki 自己强制
	// （GET 公开，revert 需 admin/moderator —— 我们只挂 auth 拿 Bearer）。
	// Fiber 按段数匹配，/<entity>/:name (2段) 与 /<entity>/:id/revisions (3段)
	// 不冲突，顺序无关；放在 taxonomy 块尾保持归类清晰。
	for _, e := range []string{"tag", "official", "engine", "series"} {
		api.Get("/"+e+"/:id/revisions", a.PatchHandler.WikiEditProxy)
		api.Get("/"+e+"/:id/revisions/:rev", a.PatchHandler.WikiEditProxy)
		api.Post("/"+e+"/:id/revert", auth, a.PatchHandler.WikiEditProxy)
	}

	// ===== Common Routes =====
	api.Get("/home", a.CommonHandler.GetHome)
	api.Get("/home/random", a.PatchHandler.GetRandomPatch)
	api.Get("/galgame", a.CommonHandler.GetGalgameList)
	api.Get("/comment", a.CommonHandler.GetGlobalComments)
	api.Get("/resource", a.CommonHandler.GetGlobalResources)
	// optionalAuth so the detail page can reflect the viewer's like state.
	api.Get("/resource/:id", optionalAuth, a.CommonHandler.GetResourceDetail)

	// Rankings (public).
	api.Get("/ranking/user", a.CommonHandler.GetUserRanking)
	api.Get("/ranking/patch", a.CommonHandler.GetPatchRanking)

	// ===== Chat Routes (D9: REST only, no WebSocket) =====
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

	// ===== Upload Routes (D10: minio-go presigned URL direct upload) =====
	uploadRoutes := api.Group("/upload", auth)
	uploadRoutes.Post("/small/init", a.UploadHandler.InitSmall)
	uploadRoutes.Post("/small/complete", a.UploadHandler.CompleteSmall)
	uploadRoutes.Post("/multipart/init", a.UploadHandler.InitMultipart)
	uploadRoutes.Post("/multipart/complete", a.UploadHandler.CompleteMultipart)
	uploadRoutes.Post("/multipart/abort", a.UploadHandler.AbortMultipart)
	// W2 / PR3b: multipart file → image_service → hash + variant URLs.
	// Used by the screenshot editor (Wiki accepts no multipart for those).
	uploadRoutes.Post("/image-service", a.UploadHandler.UploadImageService)

	// Full-text search (Meilisearch)
	api.Post("/search", a.SearchHandler.Search)

	// External APIs
	api.Get("/hikari", a.CommonHandler.GetHikari)
	api.Get("/moyu/patch/has-patch", a.CommonHandler.GetMoyuHasPatch)

	// About / docs (static .mdx posts).
	api.Get("/about/posts", a.AboutHandler.ListPosts)
	api.Get("/about/post", a.AboutHandler.GetPost)
}
