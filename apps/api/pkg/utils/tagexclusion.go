package utils

import (
	"strconv"
	"strings"

	"kun-galgame-patch-api/pkg/catalogv2"
	"kun-galgame-patch-api/pkg/errors"

	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"
)

func TagExclusionFromQuery(c fiber.Ctx) (catalogv2.TagExclusion, *errors.AppError) {
	e, err := catalogv2.ParseTagExclusion(c.Query("exclude_tag_ids"), c.Query("exclude_tag_trees"))
	if err != nil {
		return catalogv2.TagExclusion{}, errors.ErrBadRequest("最多屏蔽 100 个标签")
	}
	return e, nil
}

// Not `IN ?`: GORM binds one parameter per id, and one tag tree alone excludes
// 48,620 works against Postgres's ceiling of 65,535 parameters per statement.
// Not NOT (col = ANY(?)) either: once pgx's cached statement runs on a generic
// plan, that compares every row against the whole array — 100k rows against
// 49k ids took 5.9 s; NOT IN (SELECT unnest) hashes them in 12 ms.
func ScopeExcludedIDs(q *gorm.DB, column string, ids []int) *gorm.DB {
	if len(ids) == 0 {
		return q
	}
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.Itoa(id)
	}
	return q.Where(column+" NOT IN (SELECT unnest(?::int[]))", "{"+strings.Join(parts, ",")+"}")
}
