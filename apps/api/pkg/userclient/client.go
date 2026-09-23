package userclient

import (
	"bytes"
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
	"time"

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

	cache    sync.Map
	notFound sync.Map
	sf       singleflight.Group
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
		cfg.HTTPClient = &http.Client{Timeout: defaultTimeout}
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

func (c *Client) User(ctx context.Context, id uint) (*Brief, error) {
	m, err := c.Users(ctx, []uint{id})
	if err != nil {
		return nil, err
	}
	return m[id], nil
}

func (c *Client) Search(ctx context.Context, q string, limit int) ([]*Brief, error) {
	u := fmt.Sprintf("%s/users/search?q=%s", c.baseURL, url.QueryEscape(q))
	if limit > 0 {
		u += "&limit=" + strconv.Itoa(limit)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("oauth users/search: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("oauth users/search: status=%d", resp.StatusCode)
	}
	var env struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Users []Brief `json:"users"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return nil, fmt.Errorf("oauth users/search decode: %w", err)
	}
	if env.Code != 0 {
		return nil, fmt.Errorf("oauth users/search: code=%d msg=%s", env.Code, env.Message)
	}
	out := make([]*Brief, len(env.Data.Users))
	for i := range env.Data.Users {
		u := env.Data.Users[i]
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
	v, err, _ := c.sf.Do(singleflightKey(ids), func() (any, error) {
		parts := make([]string, len(ids))
		for i, id := range ids {
			parts[i] = strconv.FormatUint(uint64(id), 10)
		}
		u := fmt.Sprintf("%s/users/batch?ids=%s", c.baseURL, strings.Join(parts, ","))
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", c.authHeader)
		req.Header.Set("Accept", "application/json")
		resp, err := c.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("oauth users/batch: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("oauth users/batch: status=%d", resp.StatusCode)
		}
		var env struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Data    struct {
				Users    []Brief `json:"users"`
				NotFound []uint  `json:"not_found"`
			} `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
			return nil, fmt.Errorf("oauth users/batch decode: %w", err)
		}
		if env.Code != 0 {
			return nil, fmt.Errorf("oauth users/batch: code=%d msg=%s", env.Code, env.Message)
		}
		users := make(map[uint]*Brief, len(env.Data.Users))
		for i := range env.Data.Users {
			u := env.Data.Users[i]
			users[u.ID] = &u
		}
		return result{users: users, notFound: env.Data.NotFound}, nil
	})
	if err != nil {
		return nil, nil, err
	}
	r := v.(result)
	return r.users, r.notFound, nil
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

func singleflightKey(ids []uint) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.FormatUint(uint64(id), 10)
	}
	return strings.Join(parts, ",")
}

type CreatorApplication struct {
	ID            int             `json:"id"`
	UserID        int             `json:"user_id"`
	Source        string          `json:"source"`
	Status        string          `json:"status"`
	Evidence      json.RawMessage `json:"evidence,omitempty"`
	Message       string          `json:"message"`
	DeclineReason string          `json:"decline_reason"`
	ReviewedAt    *string         `json:"reviewed_at,omitempty"`
	CreatedAt     string          `json:"created_at"`
}

type CreatorAPIError struct {
	Code    int
	Message string
}

func (e *CreatorAPIError) Error() string { return e.Message }

func (c *Client) CreateCreatorApplication(ctx context.Context, token, source string, evidence json.RawMessage, message string) (*CreatorApplication, error) {
	payload, _ := json.Marshal(map[string]any{"source": source, "evidence": evidence, "message": message})
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/creator/applications", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	return c.creatorApplicationReq(req)
}

func (c *Client) GetMyCreatorApplication(ctx context.Context, token string) (*CreatorApplication, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/creator/applications/me", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return c.creatorApplicationReq(req)
}

func (c *Client) creatorApplicationReq(req *http.Request) (*CreatorApplication, error) {
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("userclient: creator application: %w", err)
	}
	defer resp.Body.Close()
	var env struct {
		Code    int                 `json:"code"`
		Message string              `json:"message"`
		Data    *CreatorApplication `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return nil, fmt.Errorf("userclient: creator application decode: %w", err)
	}
	if env.Code != 0 {
		return nil, &CreatorAPIError{Code: env.Code, Message: env.Message}
	}
	return env.Data, nil
}
