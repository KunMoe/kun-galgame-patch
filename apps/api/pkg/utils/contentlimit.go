package utils

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"
)

const (
	ContentLimitSFW  = "sfw"
	ContentLimitNSFW = "nsfw"
	ContentLimitAll  = "all"
)

func ContentLimitFromQuery(c fiber.Ctx) string {
	switch c.Query("content_limit") {
	case ContentLimitSFW, ContentLimitNSFW, ContentLimitAll:
		return c.Query("content_limit")
	default:
		return ""
	}
}

func ContentLimitForListBrowse(c fiber.Ctx) string {
	if v := ContentLimitFromQuery(c); v != "" {
		return v
	}
	return ContentLimitSFW
}

func GallerySexualOptIn(c fiber.Ctx) []int {
	var levels []int
	for _, v := range strings.Split(c.Query("gallery_sexual"), ",") {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			levels = append(levels, n)
		}
	}
	return levels
}

func IncludeEmptyGalgames(c fiber.Ctx) bool {
	return fiber.Query(c, "include_empty", false)
}

// LiveWorkPage and LiveWorkResource keep a work catalog deleted (051) off a
// public read, for a page or a resource row under the given alias. Its rows stay
// in the tables and come back with the work.
func LiveWorkPage(alias string) string {
	return alias + ".catalog_deleted_at IS NULL"
}

func LiveWorkResource(alias string) string {
	return "NOT EXISTS (SELECT 1 FROM patch deleted_work WHERE deleted_work.id = " + alias +
		".galgame_id AND deleted_work.catalog_deleted_at IS NOT NULL)"
}

// ScopeListablePatches narrows a patch query to the rows that survive
// enrichment for this reader, so a page and its COUNT agree. Apply it to the
// builder that produces both, or the pager keeps counting rows the page no
// longer shows.
//
// What actually hides a work is catalog's own gate, applied when the page is
// hydrated; patch.content_limit (032) only decides which ids get that far. NULL
// means "not mirrored yet" and passes — which is why the column could ship
// ahead of its sync, and why a stale value costs a short row and never a leak.
// A work catalog deleted (051) hydrates under no gate at all.
func ScopeListablePatches(q *gorm.DB, contentLimit string) *gorm.DB {
	q = q.Where(LiveWorkPage("patch"))
	switch contentLimit {
	case ContentLimitSFW, ContentLimitNSFW:
		return q.Where("patch.content_limit IS NULL OR patch.content_limit = ?", contentLimit)
	}
	return q
}
