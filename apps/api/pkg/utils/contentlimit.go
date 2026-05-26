package utils

import "github.com/gofiber/fiber/v2"

// Wiki NSFW filter values per docs/galgame_wiki/00-handbook-for-downstream.md §16.
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
// the way wiki itself does (§16.1 — "未识别值落到端点默认...是 safe-by-default
// 保证"). The handler then passes the resolved value down to wiki/enricher, who
// in turn forward "" as "omit the param entirely".
//
// Casing is strict (wiki spec §16.1 explicitly does NOT identify uppercase) —
// "SFW" / "Sfw" / "NSFW" all fall through to "" so a typo never silently
// upgrades to "all".
func ContentLimitFromQuery(c *fiber.Ctx) string {
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
// This is stricter than the wiki batch default ("don't filter"): moyu list
// endpoints fetch a moyu-owned set of patch IDs and then ask wiki to enrich
// them, but the *list semantics* belong to moyu (SEO safe-by-default beats
// wiki's batch-is-explicit default). See docs/galgame_wiki/00-handbook §16.2
// — wiki gives list endpoints a sfw default; we mirror that here for moyu
// list endpoints that go through the batch enrichment path.
func ContentLimitForListBrowse(c *fiber.Ctx) string {
	if v := ContentLimitFromQuery(c); v != "" {
		return v
	}
	return ContentLimitSFW
}
