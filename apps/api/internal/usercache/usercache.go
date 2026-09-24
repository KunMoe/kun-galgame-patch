// Package usercache keeps what catalog answered through a reader's own token.
//
// Every /v2/me call spends that person's catalog allowance: 100 requests a
// minute and 10,000 a day, one bucket shared by every NextMoe site they use and
// counting reads the same as writes. A page that asks catalog about the reader
// on every view spends their forum's allowance too, so an answer that can wait
// a few minutes is kept here instead.
package usercache

import (
	"context"
	"encoding/json"
	"log/slog"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/xid"
	"golang.org/x/sync/singleflight"
)

const (
	keyPrefix = "moyu:me:"
	// Outlives every answer cached under a generation, so a generation that
	// expires and restarts from empty can never name an answer still alive.
	genTTL = time.Hour
)

type Cache struct {
	rdb     *redis.Client
	flights singleflight.Group
}

func New(rdb *redis.Client) *Cache { return &Cache{rdb: rdb} }

// Slot is one cached answer. A per-reader slot is pinned to the generation it
// was opened under, so a fill that started before Forget lands where nobody
// reads any more instead of overwriting what the write just made true.
type Slot struct {
	c   *Cache
	key string
}

func (c *Cache) Slot(ctx context.Context, uid int, scope, name string) Slot {
	return Slot{c: c, key: keyPrefix + scope + ":" + strconv.Itoa(uid) + ":" + c.gen(ctx, uid, scope) + ":" + name}
}

// Key is a slot shared by every reader, for an answer whose key already names
// its version, so nothing ever has to forget it.
func (c *Cache) Key(key string) Slot {
	return Slot{c: c, key: keyPrefix + key}
}

// Forget retires every answer cached for the reader under scope. Call it after
// a write this site made on the reader's behalf.
func (c *Cache) Forget(ctx context.Context, uid int, scope string) {
	if c == nil || c.rdb == nil {
		return
	}
	if err := c.rdb.Set(context.WithoutCancel(ctx), genKey(uid, scope), xid.New().String(), genTTL).Err(); err != nil {
		slog.Warn("usercache: forget failed, the reader sees the old answer until it expires",
			"uid", uid, "scope", scope, "error", err)
	}
}

func (c *Cache) gen(ctx context.Context, uid int, scope string) string {
	if c == nil || c.rdb == nil {
		return ""
	}
	gen, _ := c.rdb.Get(ctx, genKey(uid, scope)).Result()
	return gen
}

func genKey(uid int, scope string) string {
	return keyPrefix + scope + ":" + strconv.Itoa(uid) + ":gen"
}

// Fetch answers out of the slot, or runs fill once for every concurrent
// caller of the same slot and keeps its answer for ttl. A failed fill is
// never kept.
func Fetch[T any](ctx context.Context, s Slot, ttl time.Duration, fill func(context.Context) (T, error)) (T, error) {
	if v, ok := peek[T](ctx, s); ok {
		return v, nil
	}
	if s.c == nil {
		return fill(ctx)
	}
	got, err, _ := s.c.flights.Do(s.key, func() (any, error) {
		detached := context.WithoutCancel(ctx)
		v, err := fill(detached)
		if err == nil {
			s.Store(detached, v, ttl)
		}
		return v, err
	})
	if err != nil {
		var zero T
		return zero, err
	}
	return got.(T), nil
}

func (s Slot) Store(ctx context.Context, v any, ttl time.Duration) {
	if s.c == nil || s.c.rdb == nil {
		return
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return
	}
	s.c.rdb.Set(ctx, s.key, raw, ttl)
}

func peek[T any](ctx context.Context, s Slot) (T, bool) {
	var v T
	if s.c == nil || s.c.rdb == nil {
		return v, false
	}
	raw, err := s.c.rdb.Get(ctx, s.key).Bytes()
	if err != nil || json.Unmarshal(raw, &v) != nil {
		return v, false
	}
	return v, true
}
