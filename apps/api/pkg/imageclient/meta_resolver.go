package imageclient

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"kun-galgame-patch-api/pkg/upstream"
)

const (
	metaCacheMax = 50_000
	// An empty thumbhash is a backfill that has not run yet, not a stable answer:
	// caching it forever kept blur-up from ever appearing once the backfill ran.
	metaRetryAfter = 10 * time.Minute
	// Every uncached render used to wait out the whole timeout while the image
	// service was down; after one failure the resolver stops asking for a while.
	metaOutagePause = 30 * time.Second
)

type metaEntry struct {
	meta    ImageMeta
	found   bool
	expires time.Time
}

type MetaResolver struct {
	client  *Client
	timeout time.Duration
	now     func() time.Time

	mu          sync.Mutex
	cache       map[string]metaEntry
	pausedUntil time.Time
}

func (c *Client) NewMetaResolver(timeout time.Duration) *MetaResolver {
	return &MetaResolver{client: c, timeout: timeout, now: time.Now, cache: map[string]metaEntry{}}
}

func (r *MetaResolver) Resolve(hashes []string) map[string]ImageMeta {
	now := r.now()
	out := make(map[string]ImageMeta, len(hashes))
	var miss []string

	r.mu.Lock()
	for _, h := range dedupHashes(hashes) {
		e, ok := r.cache[h]
		if !ok || (!e.expires.IsZero() && !now.Before(e.expires)) {
			miss = append(miss, h)
			continue
		}
		if e.found {
			out[h] = e.meta
		}
	}
	paused := now.Before(r.pausedUntil)
	r.mu.Unlock()

	if len(miss) == 0 || paused || !r.client.Configured() {
		return out
	}

	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()
	fetched, err := r.client.MetaBatch(ctx, miss)

	r.mu.Lock()
	defer r.mu.Unlock()
	if err != nil {
		r.pausedUntil = now.Add(metaOutagePause)
		level := slog.LevelWarn
		if upstream.KindOf(err) == upstream.Internal {
			level = slog.LevelError
		}
		slog.Log(context.Background(), level, "image meta lookup failed; content images render without dimensions",
			"hashes", len(miss), "paused_for", metaOutagePause, "error", err)
		return out
	}
	r.makeRoom(len(miss))
	for _, h := range miss {
		m, ok := fetched[h]
		switch {
		case !ok:
			r.cache[h] = metaEntry{expires: now.Add(metaRetryAfter)}
		case m.Thumbhash == "":
			out[h] = m
			r.cache[h] = metaEntry{meta: m, found: true, expires: now.Add(metaRetryAfter)}
		default:
			out[h] = m
			r.cache[h] = metaEntry{meta: m, found: true}
		}
	}
	return out
}

func (r *MetaResolver) makeRoom(n int) {
	for h := range r.cache {
		if len(r.cache)+n <= metaCacheMax {
			return
		}
		delete(r.cache, h)
	}
}

func dedupHashes(in []string) []string {
	if len(in) < 2 {
		return in
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
