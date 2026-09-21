package catalogv2

import (
	"context"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	readCacheKeyPrefix   = "nmcache:moyu:v2:"
	readCacheMaxBody     = 512 << 10
	readCacheDetailTTL   = 15 * time.Second
	readCacheListTTL     = 60 * time.Second
	readCacheCalendarTTL = time.Hour
)

// The public read faces, and only those. An allowlist rather than a list of
// exclusions, because the lanes that must never be cached are exactly the ones
// a later reader would forget to exclude: /v2/catalog/changes and
// /v2/catalog/claim-events drive incremental cursor loops where a stale page
// silently skips a window, and /v2/catalog/proposals is what an editor reloads
// to see the edit they just submitted.
//
// Safe only because the S2S GET path sends no request header that varies by
// caller — the URL alone determines the response, and every face spells the
// content limit into the query (nsfw=true / nsfw=false, always explicit), so
// two readers with different limits can never share a key.
var readCachePaths = []string{
	"/v2/catalog/works",
	"/v2/catalog/calendar",
	"/v2/catalog/search",
	"/v2/catalog/companies",
	"/v2/catalog/credit-names",
	"/v2/catalog/characters",
	"/v2/catalog/series",
	"/v2/catalog/tags",
	"/v2/catalog/schemas",
}

// WithRedis turns on the shared read cache. A client without it behaves exactly
// as before, which is what the tests and the one-off tools get.
func (c *Client) WithRedis(rdb *redis.Client) *Client {
	c.rdb = rdb
	return c
}

func readCacheable(path string) bool {
	route, _, _ := strings.Cut(path, "?")
	for _, p := range readCachePaths {
		if route == p || strings.HasPrefix(route, p+"/") {
			return true
		}
	}
	return false
}

func readCacheTTL(path string) time.Duration {
	route, _, _ := strings.Cut(path, "?")
	if route == "/v2/catalog/calendar" || strings.HasPrefix(route, "/v2/catalog/calendar/") {
		return readCacheCalendarTTL
	}
	for _, seg := range strings.Split(route, "/") {
		if isNumericSegment(seg) {
			return readCacheDetailTTL
		}
	}
	return readCacheListTTL
}

func isNumericSegment(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// A nil receiver is a supported state, not a bug: Configured() is written
// `c != nil && ...` precisely so an unconfigured deployment can carry a nil
// client and have every read come back ErrNotConfigured. Dereferencing c here
// turned that into a panic inside the fiber handler.
func (c *Client) cacheGet(ctx context.Context, path string) ([]byte, bool) {
	if c == nil || c.rdb == nil || !readCacheable(path) {
		return nil, false
	}
	body, err := c.rdb.Get(ctx, readCacheKeyPrefix+path).Bytes()
	if err != nil {
		return nil, false
	}
	return body, true
}

func (c *Client) cacheSet(ctx context.Context, path string, body []byte) {
	if c == nil || c.rdb == nil || !readCacheable(path) || len(body) == 0 || len(body) > readCacheMaxBody {
		return
	}
	c.rdb.Set(ctx, readCacheKeyPrefix+path, body, readCacheTTL(path))
}
