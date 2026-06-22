package app

import (
	"time"

	"kun-galgame-patch-api/internal/middleware"

	"github.com/gofiber/fiber/v2"
)

func (a *App) RegisterRoutes() {
	// Liveness probe — root /healthz, used by container HEALTHCHECK (no auth, no
	// DB touch). The `server healthcheck` subcommand GETs this and exits 0/1 —
	// see cmd/server/main.go + pkg/health. Unified to /healthz across services.
	a.Fiber.Get("/healthz", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

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
	// OAuth display-layer proxy (docs/oauth/02-user-profile.md §身份操作 vs
	// 展示操作). ONLY name / bio / avatar are proxied here — these are
	// "展示层" per the 2026-05-23 policy and downstream sites are free to
	// expose their own UI for them.
	//
	// Identity-layer ops (改密码 / 改邮箱 / 2FA / 注销账号) MUST stay on
	// OAuth's own profile UI and are NOT proxied — the moyu frontend uses
	// a jump button to https://oauth.kungal.com/profile?return=<url>.
	// Reason: security audit, future 2FA / anomaly-notify rollouts, and
	// avoiding email-hijack attack surface across multiple sites.
	authRoutes.Patch("/me", auth, a.AuthHandler.UpdateMe)
	authRoutes.Post("/me/avatar", auth, a.AuthHandler.UploadAvatar)

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
	patchRoutes.Get("/comment/:commentId/locate", optionalAuth, a.PatchHandler.LocateComment)

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
	// 30/min per userID (or per IP when anonymous) keeps legitimate browsing
	// (one user opens 10 patch resource pages = ~10-30 calls) untouched
	// while breaking automated scraping. Returns 429 on overflow.
	// optionalAuth runs BEFORE the limiter so it can key by userID for
	// logged-in callers (the documented "30/min per userID, per IP when
	// anonymous"). Without it the user context is never populated and the
	// limiter always falls back to IP — collectively throttling logged-in
	// users behind a shared NAT/proxy.
	patchRoutes.Get(
		"/resource/:resourceId/link",
		optionalAuth,
		middleware.RateLimit(a.RDB, "resource-link", 30, time.Minute),
		a.PatchHandler.GetResourceDownloadInfo,
	)
	// Public counter, but rate-limited (audit F069): without a cap anyone can
	// script this to inflate a resource's/patch's download count (pollutes
	// rankings + sort). optionalAuth lets it key by userID when logged in,
	// else per IP — same shape as the /link limiter above.
	patchRoutes.Put(
		"/resource/:resourceId/download",
		optionalAuth,
		middleware.RateLimit(a.RDB, "resource-download", 60, time.Minute),
		a.PatchHandler.IncrementResourceDownload,
	)
	patchRoutes.Put("/resource/:resourceId/like", auth, a.PatchHandler.ToggleResourceLike)
	// Per-resource subscription: notified (patchResourceUpdate) when this
	// resource's file/link changes. Distinct from /like and the galgame /favorite.
	patchRoutes.Put("/resource/:resourceId/favorite", auth, a.PatchHandler.ToggleResourceFavorite)
	// Public per-field edit history (diff) for one resource (anyone, incl.
	// anonymous). Rate-limited like the other id-keyed resource reads. Changes
	// are secret-free (service strips download links / codes) — distinct from
	// the admin-only /admin/resource/:id/history file-replacement audit.
	patchRoutes.Get(
		"/resource/:resourceId/revisions",
		middleware.RateLimit(a.RDB, "resource-revisions", 60, time.Minute),
		a.PatchHandler.GetResourceRevisions,
	)
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
	// Wiki contributor list / removal is no longer surfaced. moyu only
	// edits the galgame's metadata (creator can update / admins can
	// moderate); contributors are an attribution attribute owned by Wiki
	// and not editable from the moyu side. The local /patch/:id/contributor
	// route above is a different concept (people who uploaded patch
	// resources on moyu) — that one stays.

	// ===== User Routes =====
	//
	// Profile mutations (username/bio/password/email/avatar) live on OAuth and
	// are intentionally absent here. The frontend either redirects to
	// oauth.kungal.com/profile or proxies PATCH /auth/me to OAuth itself.
	userRoutes := api.Group("/user")

	userRoutes.Post("/image", auth, a.UserHandler.UploadImage)
	userRoutes.Post("/check-in", auth, a.UserHandler.CheckIn)
	userRoutes.Get("/search", auth, a.UserHandler.SearchUsers)
	// Self-service moemoepoint ledger (own records only; id from session).
	// Registered BEFORE /:id so Fiber doesn't match "moemoepoint" as a :id.
	userRoutes.Get("/moemoepoint/log", auth, a.UserHandler.GetMoemoepointLog)

	// Creator-role application: moyu checks its eligibility (wiki PR stats +
	// own published patch resources), then files on the central OAuth queue.
	// Fixed paths, registered BEFORE /:id. See docs/auth/01-creator-role-design.md.
	userRoutes.Get("/creator/status", auth, a.UserHandler.CreatorStatus)
	userRoutes.Post("/creator/apply", auth, a.UserHandler.CreatorApply)

	// Public user profiles
	userRoutes.Get("/:id", optionalAuth, a.UserHandler.GetUserInfo)
	userRoutes.Get("/:id/floating", a.UserHandler.GetUserFloating)
	userRoutes.Get("/:id/patch", a.UserHandler.GetUserPatches)
	userRoutes.Get("/:id/resource", a.UserHandler.GetUserResources)
	userRoutes.Get("/:id/favorite", a.UserHandler.GetUserFavorites)
	userRoutes.Get("/:id/comment", a.UserHandler.GetUserComments)
	userRoutes.Get("/:id/contribute", a.UserHandler.GetUserContributions)
	userRoutes.Get("/:id/follower", optionalAuth, a.UserHandler.GetFollowers)
	userRoutes.Get("/:id/following", optionalAuth, a.UserHandler.GetFollowing)

	// Follow/Unfollow
	userRoutes.Put("/:id/follow", auth, a.UserHandler.Follow)
	userRoutes.Delete("/:id/follow", auth, a.UserHandler.Unfollow)

	// ===== Message Routes =====
	msgRoutes := api.Group("/message", auth)
	msgRoutes.Get("/", a.MessageHandler.GetMessages)
	msgRoutes.Get("/all", a.MessageHandler.GetAllMessages)
	msgRoutes.Get("/unread", a.MessageHandler.GetUnreadTypes)
	// NOTE: POST /message was removed (API audit 2026-05-29). It let ANY
	// authenticated user write an arbitrary notification (client-controlled
	// recipient_id / type / content / link, no rate limit) into ANY other
	// user's inbox — a spam/phishing primitive. It had no frontend caller;
	// all legitimate notifications are created server-side via the patch
	// service's createDedupMessage. Re-add only with recipient restricted to
	// an existing relationship + enum-validated type + rate limiting.
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
	// Approve a pending (comment-verify) comment → visible. PatchHandler owns it
	// because the approval reuses PatchService's comment side-effect logic
	// (count / moemoepoint / contributor / notifications).
	adminRoutes.Put("/comment/:id/approve", a.PatchHandler.ApproveComment)

	// Resources
	adminRoutes.Get("/resource", a.AdminHandler.GetResources)
	adminRoutes.Put("/resource/:id", a.AdminHandler.UpdateResource)
	adminRoutes.Delete("/resource/:id", a.AdminHandler.DeleteResource)
	// MOYU-PR5 / M3 — append-only file-replacement audit trail for one resource.
	adminRoutes.Get("/resource/:id/history", a.AdminHandler.GetResourceFileHistory)

	// User purge (anti-spam): wipe all moyu-side traces of one user. Account-
	// level destruction → admin-only (stricter than the moderator-level single
	// comment/resource deletes above). Preview is a dry run; purge executes.
	adminRoutes.Get("/user/:id/purge-preview", adminAuth, a.AdminHandler.GetUserPurgePreview)
	adminRoutes.Post("/user/:id/purge", adminAuth, a.AdminHandler.PurgeUser)

	// Settings
	adminRoutes.Get("/setting/comment-verify", a.AdminHandler.GetCommentVerify)
	adminRoutes.Put("/setting/comment-verify", adminAuth, a.AdminHandler.SetCommentVerify)
	adminRoutes.Get("/setting/creator-only", a.AdminHandler.GetCreatorOnly)
	adminRoutes.Put("/setting/creator-only", adminAuth, a.AdminHandler.SetCreatorOnly)
	// NOTE: the "禁止注册" (disable-register) setting was removed — registration
	// is unified on the OAuth server (local register flow is gone), so the toggle
	// belongs there, not here.

	// Stats & Logs
	adminRoutes.Get("/stats", a.AdminHandler.GetStats)
	adminRoutes.Get("/stats/sum", a.AdminHandler.GetStatsSum)
	adminRoutes.Get("/log", a.AdminHandler.GetLogs)

	// All patches (admin browse, paginated, optional vndb_id search)
	adminRoutes.Get("/galgame", a.AdminHandler.GetGalgame)

	// D12: "orphan patches" whose galgame is missing in Wiki, for admin manual handling
	adminRoutes.Get("/patch/orphans", a.AdminHandler.GetOrphanPatches)

	// Doc management (migration 016; unified about+blog). Create/edit/delete
	// docs; list includes drafts. Banners / inline images come from
	// image_service (uploaded via /upload/image-service). moderator+ (group default).
	adminRoutes.Get("/doc", a.DocHandler.AdminListPosts)
	adminRoutes.Get("/doc/:id", a.DocHandler.AdminGetPost)
	adminRoutes.Post("/doc", a.DocHandler.CreatePost)
	adminRoutes.Put("/doc/:id", a.DocHandler.UpdatePost)
	adminRoutes.Delete("/doc/:id", a.DocHandler.DeletePost)

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
	// /tag/:name and /official/:name carry an associated `galgame` list —
	// we rewrite the response in WikiTaxonomyDetailProxy so each entry
	// has moyu's enriched GalgameCard shape (per-patch counts, KunLanguage
	// name etc.), letting the FE render the same <GalgameCard> as home /
	// galgame index. Other tag/official endpoints stay generic passthrough.
	api.Get("/tag/:name", a.PatchHandler.WikiTaxonomyDetailProxy)

	api.Get("/official", a.PatchHandler.WikiEditProxy)
	api.Get("/official/search", a.PatchHandler.WikiEditProxy)
	api.Post("/official", auth, a.PatchHandler.WikiEditProxy)
	api.Put("/official", auth, a.PatchHandler.WikiEditProxy)
	api.Delete("/official/:id", auth, a.PatchHandler.WikiEditProxy)
	api.Get("/official/:name", a.PatchHandler.WikiTaxonomyDetailProxy)

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
	// Rate-limited (audit GPT-M03): this endpoint returns the main resource's
	// real download payload (content/code/password), so an unthrottled id-walk
	// could harvest every resource's links — the same scraping vector the
	// /patch/resource/:id/link limiter exists to stop. 60/min/(user|IP) keeps
	// normal browsing untouched.
	api.Get("/resource/:id",
		optionalAuth,
		middleware.RateLimit(a.RDB, "resource-detail", 60, time.Minute),
		a.CommonHandler.GetResourceDetail,
	)

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

	// ===== Upload Routes (server-driven presigned upload via the artifact service) =====
	// One flow: init → (single PUT | multipart parts) → complete; abort cancels.
	uploadRoutes := api.Group("/upload", auth)
	uploadRoutes.Post("/init", a.UploadHandler.Init)
	uploadRoutes.Post("/complete", a.UploadHandler.Complete)
	uploadRoutes.Post("/abort", a.UploadHandler.Abort)
	// W2 / PR3b: multipart file → image_service → hash + variant URLs.
	// Used by the screenshot editor (Wiki accepts no multipart for those).
	uploadRoutes.Post("/image-service", a.UploadHandler.UploadImageService)

	// Full-text search (Meilisearch)
	api.Post("/search", a.SearchHandler.Search)

	// External APIs.
	// Hikari is a public partner API (Hikarinagi / ShionLib / TouchGal / …):
	// its own CORS domain allowlist (via Use so the OPTIONS preflight is
	// answered too) + a generous 10000/min/IP rate limit. The handler returns
	// only public metadata — no uploader identity, no download secrets.
	api.Use("/hikari", middleware.HikariCORS())
	api.Get("/hikari", middleware.RateLimit(a.RDB, "hikari", 10000, time.Minute), a.CommonHandler.GetHikari)
	api.Get("/moyu/patch/has-patch", a.CommonHandler.GetMoyuHasPatch)

	// Doc (public, published-only; migration 016 — unified about+blog). Posts
	// list + category tree; detail renders markdown and derives the
	// image_service banner URL. View counter is anonymous.
	api.Get("/doc/posts", a.DocHandler.ListPosts)
	api.Get("/doc/pinned", a.DocHandler.ListPinnedPosts)
	api.Get("/doc/post", a.DocHandler.GetPost)
	api.Put("/doc/view", a.DocHandler.IncrementView)
}
