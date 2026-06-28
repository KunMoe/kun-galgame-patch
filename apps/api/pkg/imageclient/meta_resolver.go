package imageclient

import (
	"context"
	"sync"
	"time"
)

// MetaResolver wraps Client.MetaBatch with a permanent in-process cache.
//
// Image metadata (dimensions + ThumbHash) is immutable per content-addressed
// hash, so positive results are cached forever. Built for the markdown render
// path (server-rendered comments / notes / intro): the renderer calls
// Resolve(hashes) SYNCHRONOUSLY, so the cache keeps warm renders network-free
// and a tight per-call timeout bounds the cold path.
//
// Misses (hashes image_service doesn't know) AND partial hits (a hash that is
// known but whose ThumbHash hasn't been computed/backfilled yet — width/height
// present, thumbhash empty) are deliberately NOT cached, so they light up on a
// later render once the backfill fills them in. Only complete entries (with a
// ThumbHash) cache, and those cache forever (immutable per content hash). After
// the backfill completes every referenced hash is complete → cached, and
// renders stop touching the network entirely.
type MetaResolver struct {
	client  *Client
	timeout time.Duration
	mu      sync.RWMutex
	cache   map[string]ImageMeta
}

// NewMetaResolver builds a resolver over this client. timeout bounds the
// synchronous meta-batch call on the cold path (<= 0 → 3s default) so a slow or
// unreachable image_service can never stall a render for long.
func (c *Client) NewMetaResolver(timeout time.Duration) *MetaResolver {
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	return &MetaResolver{client: c, timeout: timeout, cache: map[string]ImageMeta{}}
}

// Resolve returns metadata for the given hashes, serving cached entries and
// fetching the rest via one MetaBatch. Best-effort: on any error (client
// unconfigured, network, timeout) it returns whatever is cached and omits the
// rest, so the caller renders a plain <img> (no blur-up) rather than failing.
func (r *MetaResolver) Resolve(hashes []string) map[string]ImageMeta {
	out := make(map[string]ImageMeta, len(hashes))
	var miss []string

	r.mu.RLock()
	for _, h := range hashes {
		if m, ok := r.cache[h]; ok {
			out[h] = m
		} else {
			miss = append(miss, h)
		}
	}
	r.mu.RUnlock()

	if len(miss) == 0 || !r.client.Configured() {
		return out
	}

	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()
	fetched, err := r.client.MetaBatch(ctx, dedupHashes(miss))
	if err != nil {
		return out // best-effort: keep cached, skip the rest
	}

	r.mu.Lock()
	for h, m := range fetched {
		out[h] = m
		// Cache COMPLETE entries only. A result with an empty ThumbHash is a
		// partial hit — the hash is known (dimensions set) but its thumbhash
		// hasn't been computed/backfilled yet. The cache has no TTL, so caching
		// it would pin the empty placeholder forever and the image would never
		// blur-up even after the backfill fills it in. Return it for its
		// dimensions but DON'T cache, so a later render re-resolves and lights
		// it up — same rationale as the not-caching-misses rule above.
		if m.Thumbhash != "" {
			r.cache[h] = m
		}
	}
	r.mu.Unlock()
	return out
}

// Put pre-seeds the cache (e.g. from an upload result, which already carries the
// metadata) so the first render of a freshly uploaded image needs no network.
func (r *MetaResolver) Put(hash string, m ImageMeta) {
	if hash == "" {
		return
	}
	r.mu.Lock()
	r.cache[hash] = m
	r.mu.Unlock()
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
