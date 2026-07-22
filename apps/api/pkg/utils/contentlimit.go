package utils

import "github.com/gofiber/fiber/v3"

// galgame NSFW filter values per docs/galgame_wiki/00-handbook-for-downstream.md §16.
const (
	ContentLimitSFW  = "sfw"
	ContentLimitNSFW = "nsfw"
	ContentLimitAll  = "all"
)

// ContentLimitFromQuery parses the `content_limit` query parameter and returns
// one of {"sfw","nsfw","all"} when recognized, or "" otherwise.
//
// Returning "" for unrecognized / missing values is intentional: it leaves the
// "what's the default" decision to the caller (handler) per endpoint, exactly
// the way galgame itself does (§16.1 — "未识别值落到端点默认...是 safe-by-default
// 保证"). The handler then passes the resolved value down to galgame/enricher, who
// in turn forward "" as "omit the param entirely".
//
// Casing is strict (galgame spec §16.1 explicitly does NOT identify uppercase) —
// "SFW" / "Sfw" / "NSFW" all fall through to "" so a typo never silently
// upgrades to "all".
func ContentLimitFromQuery(c fiber.Ctx) string {
	switch c.Query("content_limit") {
	case ContentLimitSFW, ContentLimitNSFW, ContentLimitAll:
		return c.Query("content_limit")
	default:
		return ""
	}
}

// ContentLimitForListBrowse resolves the content_limit for browse-style list
// endpoints (home / list / ranking / user patches / etc.). The query value
// wins when valid; otherwise we default to "sfw".
//
// This is stricter than the galgame batch default ("don't filter"): moyu list
// endpoints fetch a moyu-owned set of patch IDs and then ask galgame to enrich
// them, but the *list semantics* belong to moyu (SEO safe-by-default beats
// galgame's batch-is-explicit default). See docs/galgame_wiki/00-handbook §16.2
// — galgame gives list endpoints a sfw default; we mirror that here for moyu
// list endpoints that go through the batch enrichment path.
func ContentLimitForListBrowse(c fiber.Ctx) string {
	if v := ContentLimitFromQuery(c); v != "" {
		return v
	}
	return ContentLimitSFW
}

// IncludeEmptyGalgames reports whether a browse-list caller opted in to seeing
// galgames that have no patch resources (patch.resource_count = 0), via the
// `include_empty` query parameter. Default (absent / unparseable / false) →
// false → such games are hidden, so listings only surface games with patches.
//
// Driven by the frontend "显示设置 → 显示无补丁资源的游戏" toggle, which
// apps/web composables/useApi.ts forwards (include_empty=true) on every request
// when enabled. Every moyu-owned galgame-list endpoint (galgame list / home /
// ranking / a user's patches / favorites / contributions) applies
// `resource_count > 0` unless this returns true. resource_count is maintained
// on resource create/delete (patch/service UpdateCount), so > 0 == "has at
// least one patch resource". galgame-delegated lists (tag / search) are NOT moyu
// owned and intentionally don't honor this.
func IncludeEmptyGalgames(c fiber.Ctx) bool {
	return fiber.Query(c, "include_empty", false)
}
