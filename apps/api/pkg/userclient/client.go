package userclient

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"kun-galgame-patch-api/pkg/upstream"

	"golang.org/x/sync/singleflight"
)

// OAuth owns these profile fields; moyu stores user IDs, not profile truth.
type Brief struct {
	ID              uint       `json:"id"`
	UUID            string     `json:"uuid"`
	Name            string     `json:"name"`
	Avatar          string     `json:"avatar"`
	AvatarImageHash string     `json:"avatar_image_hash"`
	Bio             string     `json:"bio"`
	Status          int        `json:"status"`
	Roles           []string   `json:"roles"`
	SiteRoles       []string   `json:"site_roles"`
	Cosmetics       *Cosmetics `json:"cosmetics,omitempty"`
}

type Cosmetics struct {
	AvatarFrame       *Decoration `json:"avatar_frame,omitempty"`
	ProfileBackground *Decoration `json:"profile_background,omitempty"`
}

type Decoration struct {
	ItemID      int64  `json:"item_id"`
	Name        string `json:"name"`
	StaticURL   string `json:"static_url"`
	AnimatedURL string `json:"animated_url,omitempty"`
}

const (
	batchMaxIDs        = 100
	defaultCacheTTL    = 10 * time.Minute
	defaultNotFoundTTL = 1 * time.Minute
	defaultTimeout     = 5 * time.Second

	// OAuth's /users/search answers 400 past 50 runes (infra
	// user_batch_handler.go Search); moyu's search box allows more.
	searchMaxRunes = 50
)

type Config struct {
	BaseURL      string
	ClientID     string
	ClientSecret string
	CacheTTL     time.Duration
	NotFoundTTL  time.Duration
	HTTPClient   *http.Client
}

type Client struct {
	baseURL     string
	authHeader  string
	http        *http.Client
	cacheTTL    time.Duration
	notFoundTTL time.Duration

	cache     sync.Map
	notFound  sync.Map
	lastSweep atomic.Int64
	sf        singleflight.Group
}

type cacheEntry struct {
	brief   *Brief
	expires time.Time
}

func New(cfg Config) *Client {
	if cfg.CacheTTL == 0 {
		cfg.CacheTTL = defaultCacheTTL
	}
	if cfg.NotFoundTTL == 0 {
		cfg.NotFoundTTL = defaultNotFoundTTL
	}
	if cfg.HTTPClient == nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.MaxIdleConnsPerHost = 32
		cfg.HTTPClient = &http.Client{Timeout: defaultTimeout, Transport: transport}
	}
	creds := cfg.ClientID + ":" + cfg.ClientSecret
	return &Client{
		baseURL:     strings.TrimRight(cfg.BaseURL, "/"),
		authHeader:  "Basic " + base64.StdEncoding.EncodeToString([]byte(creds)),
		http:        cfg.HTTPClient,
		cacheTTL:    cfg.CacheTTL,
		notFoundTTL: cfg.NotFoundTTL,
	}
}

func (c *Client) Users(ctx context.Context, ids []uint) (map[uint]*Brief, error) {
	out := make(map[uint]*Brief, len(ids))
	if len(ids) == 0 {
		return out, nil
	}

	now := time.Now()
	missing := make([]uint, 0, len(ids))
	seen := make(map[uint]struct{}, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}

		if v, ok := c.notFound.Load(id); ok {
			if v.(time.Time).After(now) {
				continue
			}
			c.notFound.Delete(id)
		}
		if v, ok := c.cache.Load(id); ok {
			e := v.(cacheEntry)
			if e.expires.After(now) {
				out[id] = e.brief
				continue
			}
			c.cache.Delete(id)
		}
		missing = append(missing, id)
	}

	if len(missing) == 0 {
		return out, nil
	}

	c.sweep(now)
	slices.Sort(missing)

	expires := now.Add(c.cacheTTL)
	notFoundUntil := now.Add(c.notFoundTTL)

	for _, batch := range chunk(missing, batchMaxIDs) {
		fetched, notFound, err := c.fetchBatch(ctx, batch)
		if err != nil {
			return out, err
		}
		for id, brief := range fetched {
			c.cache.Store(id, cacheEntry{brief: brief, expires: expires})
			out[id] = brief
		}
		for _, id := range notFound {
			c.notFound.Store(id, notFoundUntil)
		}
	}
	return out, nil
}

// sweep drops expired entries at most once per TTL. A lookup only evicts the
// entry it reads, so without it every user ever seen stayed in memory.
func (c *Client) sweep(now time.Time) {
	last := c.lastSweep.Load()
	if now.UnixNano()-last < int64(c.cacheTTL) || !c.lastSweep.CompareAndSwap(last, now.UnixNano()) {
		return
	}
	c.cache.Range(func(k, v any) bool {
		if !v.(cacheEntry).expires.After(now) {
			c.cache.Delete(k)
		}
		return true
	})
	c.notFound.Range(func(k, v any) bool {
		if !v.(time.Time).After(now) {
			c.notFound.Delete(k)
		}
		return true
	})
}

func (c *Client) User(ctx context.Context, id uint) (*Brief, error) {
	m, err := c.Users(ctx, []uint{id})
	if err != nil {
		return nil, err
	}
	return m[id], nil
}

// Search is never cached (C6). The query is trimmed and cut to what OAuth
// accepts, so a long keyword searches its first 50 runes instead of failing.
func (c *Client) Search(ctx context.Context, q string, limit int) ([]*Brief, error) {
	q = strings.TrimSpace(q)
	if utf8.RuneCountInString(q) > searchMaxRunes {
		q = strings.TrimSpace(string([]rune(q)[:searchMaxRunes]))
	}
	if q == "" {
		return []*Brief{}, nil
	}
	u := fmt.Sprintf("%s/users/search?q=%s", c.baseURL, url.QueryEscape(q))
	if limit > 0 {
		u += "&limit=" + strconv.Itoa(limit)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.authHeader)

	var data struct {
		Users []Brief `json:"users"`
	}
	if err := c.do(req, "users/search", clientKind, &data); err != nil {
		return nil, err
	}
	out := make([]*Brief, len(data.Users))
	for i := range data.Users {
		u := data.Users[i]
		out[i] = &u
	}
	return out, nil
}

func (c *Client) Invalidate(id uint) {
	c.cache.Delete(id)
	c.notFound.Delete(id)
}

func (c *Client) fetchBatch(ctx context.Context, ids []uint) (map[uint]*Brief, []uint, error) {
	type result struct {
		users    map[uint]*Brief
		notFound []uint
	}
	key := joinIDs(ids)
	v, err, _ := c.sf.Do(key, func() (any, error) {
		// Shared by every caller waiting on this key, so it must not end when
		// the first of them does.
		ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), defaultTimeout)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/users/batch?ids="+key, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", c.authHeader)

		var data struct {
			Users    []Brief `json:"users"`
			NotFound []uint  `json:"not_found"`
		}
		if err := c.do(req, "users/batch", clientKind, &data); err != nil {
			return nil, err
		}
		users := make(map[uint]*Brief, len(data.Users))
		for i := range data.Users {
			u := data.Users[i]
			users[u.ID] = &u
		}
		return result{users: users, notFound: data.NotFound}, nil
	})
	if err != nil {
		return nil, nil, err
	}
	r := v.(result)
	return r.users, r.notFound, nil
}

// clientKind reads a failure of a face moyu calls with its own client Basic
// credential: nothing the reader sent can make it 4xx.
func clientKind(status, _ int) upstream.Kind {
	switch {
	case status >= 500:
		return upstream.Unavailable
	case status == http.StatusTooManyRequests:
		return upstream.RateLimited
	default:
		return upstream.Internal
	}
}

func (c *Client) do(req *http.Request, op string, kind func(status, code int) upstream.Kind, out any) error {
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return upstream.Transport("oauth", op, err)
	}
	defer resp.Body.Close()
	body, err := upstream.ReadBody(resp.Body)
	if err != nil {
		return upstream.Transport("oauth", op, err)
	}

	fail := func(code int, detail string, cause error) error {
		return &upstream.Error{
			Service:    "oauth",
			Op:         op,
			Kind:       kind(resp.StatusCode, code),
			Status:     resp.StatusCode,
			Code:       strconv.Itoa(code),
			Detail:     detail,
			RequestID:  resp.Header.Get("X-Request-ID"),
			RetryAfter: upstream.RetryAfter(resp.Header),
			Cause:      cause,
		}
	}
	var env struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return fail(0, "", fmt.Errorf("decode envelope: %w", err))
	}
	if resp.StatusCode >= 400 || env.Code != 0 {
		return fail(env.Code, env.Message, nil)
	}
	if len(env.Data) == 0 {
		return nil
	}
	if err := json.Unmarshal(env.Data, out); err != nil {
		return fail(0, "", fmt.Errorf("decode data: %w", err))
	}
	return nil
}

func chunk(ids []uint, n int) [][]uint {
	if len(ids) <= n {
		return [][]uint{ids}
	}
	out := make([][]uint, 0, (len(ids)+n-1)/n)
	for i := 0; i < len(ids); i += n {
		end := min(i+n, len(ids))
		out = append(out, ids[i:end])
	}
	return out
}

func joinIDs(ids []uint) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.FormatUint(uint64(id), 10)
	}
	return strings.Join(parts, ",")
}
