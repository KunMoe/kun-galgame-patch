// Package activitypush projects moyu's public activity into NextMoe
// community's following feed (contract docs/community/01 "Activities and the
// following feed").
//
// Migration 043 enqueues a key in the transaction that changed it; the drainer
// reads each key's current state and pushes it, and a daily reconcile repairs
// whatever changed without touching a local row (a catalog rename, a rating).
//
// Comments are NOT pushed as activities: community writes them itself, under
// the `community:` key prefix it reserves and refuses from a site. What it
// needs from moyu is which page each comment wall is on (presentation.go).
// Likes are not pushed either — no moyu page shows who liked a resource.
package activitypush

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	prefixResource = "patch_resource:"
	prefixEdit     = "patch_resource_edit:"

	objectKind  = "patch_resource"
	objectLabel = "Galgame 补丁"

	verbPublish = "publish"
	verbEdit    = "edit"
)

// beijing is the day community groups a feed by. An edit key's day must be the
// same day, or one day's edits land in two groups.
var beijing = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("CST", 8*3600)
	}
	return loc
}()

type keyKind uint8

const (
	kindResource keyKind = iota + 1
	kindEdit
)

type parsedKey struct {
	kind       keyKind
	resourceID int
	actorID    int
	day        string // YYYYMMDD, edits only
}

func resourceKey(id int) string { return prefixResource + strconv.Itoa(id) }

func editKey(resourceID, actorID int, at time.Time) string {
	return fmt.Sprintf("%s%d:%d:%s", prefixEdit, resourceID, actorID, beijingDay(at))
}

func beijingDay(t time.Time) string { return t.In(beijing).Format("20060102") }

func parseKey(key string) (parsedKey, bool) {
	if rest, ok := strings.CutPrefix(key, prefixEdit); ok {
		parts := strings.Split(rest, ":")
		if len(parts) != 3 || len(parts[2]) != 8 {
			return parsedKey{}, false
		}
		rid, err1 := strconv.Atoi(parts[0])
		aid, err2 := strconv.Atoi(parts[1])
		if _, err3 := strconv.Atoi(parts[2]); err1 != nil || err2 != nil || err3 != nil || rid <= 0 || aid <= 0 {
			return parsedKey{}, false
		}
		return parsedKey{kind: kindEdit, resourceID: rid, actorID: aid, day: parts[2]}, true
	}
	if rest, ok := strings.CutPrefix(key, prefixResource); ok {
		rid, err := strconv.Atoi(rest)
		if err != nil || rid <= 0 {
			return parsedKey{}, false
		}
		return parsedKey{kind: kindResource, resourceID: rid}, true
	}
	return parsedKey{}, false
}
