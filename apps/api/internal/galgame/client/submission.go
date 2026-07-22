package client

// This file implements the user-submission + admin-review flow described in
// docs/galgame_wiki/07-submission.md and 08-messages.md. The split from
// client.go is purely organizational — all methods belong to *Client.
//
// Two auth modes are at play:
//   - User-facing methods (Submit / Claim / PatchDraft / DeleteDraft / ListMine /
//     SearchWithPending / MyMessages) transparently forward the user's OAuth
//     access_token. galgame decodes the JWT itself; this site never re-decides
//     identity. Since open-API phase 2 wave 06a the write members (Submit /
//     Claim / PatchDraft / DeleteDraft) also ride the internal face and so carry
//     the internal-tier X-API-Key alongside the Bearer (dual credential) — see
//     writeTarget in client.go.
//   - Server-to-server (MessageFeed) uses the internal-tier X-API-Key on the
//     internal face — the same key every other read carries.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strconv"
)

// ─── DTOs ──────────────────────────────────────────────

// SubmitGalgameRequest is the JSON body of POST /galgame/submit. All fields
// are pointers so callers can omit them (galgame applies its own defaults).
//
// U1: ReleaseDate / ReleaseDateTBA replace the old `released string`.
// W2 / galgame PR5: BannerImageHash dropped — banner via multipart `file` (auto
// promoted to covers[sort_order=0]) or explicit covers array.
// Covers / Screenshots presence-replace semantics (see UpdateGalgameRequest
// comment).
type SubmitGalgameRequest struct {
	VndbID           *string            `json:"vndb_id,omitempty"`
	NameEnUs         *string            `json:"name_en_us,omitempty"`
	NameJaJp         *string            `json:"name_ja_jp,omitempty"`
	NameZhCn         *string            `json:"name_zh_cn,omitempty"`
	NameZhTw         *string            `json:"name_zh_tw,omitempty"`
	Banner           *string            `json:"banner,omitempty"`
	IntroEnUs        *string            `json:"intro_en_us,omitempty"`
	IntroJaJp        *string            `json:"intro_ja_jp,omitempty"`
	IntroZhCn        *string            `json:"intro_zh_cn,omitempty"`
	IntroZhTw        *string            `json:"intro_zh_tw,omitempty"`
	ContentLimit     *string            `json:"content_limit,omitempty"`
	OriginalLanguage *string            `json:"original_language,omitempty"`
	AgeLimit         *string            `json:"age_limit,omitempty"`
	ReleaseDate      *string            `json:"release_date,omitempty"`
	ReleaseDateTBA   *bool              `json:"release_date_tba,omitempty"`
	SeriesID         *int               `json:"series_id,omitempty"`
	Aliases          *string            `json:"aliases,omitempty"`
	TagIDs           *[]int             `json:"tag_ids,omitempty"`
	OfficialIDs      *[]int             `json:"official_ids,omitempty"`
	EngineIDs        *[]int             `json:"engine_ids,omitempty"`
	Covers           *[]CoverInput      `json:"covers,omitempty"`
	Screenshots      *[]ScreenshotInput `json:"screenshots,omitempty"`
}

// MineItem mirrors one entry returned by GET /galgame/mine. Only the
// columns relevant to the "my submissions" page are typed; decline_reason is
// present only on status=4 rows.
type MineItem struct {
	ID                  int    `json:"id"`
	Status              int    `json:"status"`
	VndbID              string `json:"vndb_id"`
	NameEnUs            string `json:"name_en_us"`
	NameJaJp            string `json:"name_ja_jp"`
	NameZhCn            string `json:"name_zh_cn"`
	NameZhTw            string `json:"name_zh_tw"`
	Banner              string `json:"banner"`
	EffectiveBannerHash string `json:"effective_banner_hash"`
	ContentLimit        string `json:"content_limit"`
	Created             string `json:"created"`
	Updated             string `json:"updated"`
	DeclineReason       string `json:"decline_reason,omitempty"`
}

// MessageGalgame is the embedded galgame brief on a galgame message. It may
// be nil if the galgame was hard-deleted between event emission and read.
type MessageGalgame struct {
	ID                  int    `json:"id"`
	NameEnUs            string `json:"name_en_us"`
	NameJaJp            string `json:"name_ja_jp"`
	NameZhCn            string `json:"name_zh_cn"`
	NameZhTw            string `json:"name_zh_tw"`
	Banner              string `json:"banner"`
	EffectiveBannerHash string `json:"effective_banner_hash"`
	Status              int    `json:"status"`
	UserID              int    `json:"user_id"`
}

// GalgameMessage is one entry from /galgame/messages/mine or /galgame/messages/feed.
type GalgameMessage struct {
	ID           int64           `json:"id"`
	Type         string          `json:"type"`
	GalgameID    int             `json:"galgame_id"`
	Galgame      *MessageGalgame `json:"galgame,omitempty"`
	ActorUserID  *int            `json:"actor_user_id,omitempty"`
	TargetUserID *int            `json:"target_user_id,omitempty"`
	Payload      json.RawMessage `json:"payload,omitempty"`
	CreatedAt    string          `json:"created_at"`
}

// SearchPending is the additional `pending` array surfaced by
// /galgame/search?include_pending=true (only when the caller is the owner of
// the matching status ∈ {3,4} entries).
type SearchPending struct {
	Items   []GalgameHit `json:"items"`
	Pending []GalgameHit `json:"pending"`
	Total   int64        `json:"total"`
}

// ─── User-facing methods (transparent JWT forwarding) ──

// SubmitGalgame proxies POST /galgame/submit in JSON mode. Use
// SubmitGalgameMultipart when the user also uploads a banner file.
func (c *Client) SubmitGalgame(ctx context.Context, accessToken string, body any) (json.RawMessage, error) {
	return c.writeUserJSON(ctx, http.MethodPost, "/galgame/submit", accessToken, body)
}

// SubmitGalgameMultipart proxies POST /galgame/submit in multipart mode. The
// `data` part is the JSON body, the `file` part is the optional banner image.
func (c *Client) SubmitGalgameMultipart(
	ctx context.Context,
	accessToken string,
	jsonBody any,
	fileName string,
	fileContent []byte,
	fileMime string,
) (json.RawMessage, error) {
	return c.writeUserMultipart(ctx, http.MethodPost, "/galgame/submit",
		accessToken, jsonBody, fileName, fileContent, fileMime)
}

// ClaimGalgame proxies POST /galgame/:gid/claim — claim a VNDB draft (status=2)
// and immediately publish it (status=0). Returns the published galgame.
func (c *Client) ClaimGalgame(ctx context.Context, accessToken string, gid int) (json.RawMessage, error) {
	return c.writeUserJSON(ctx, http.MethodPost,
		fmt.Sprintf("/galgame/%d/claim", gid), accessToken, map[string]any{})
}

// PatchGalgameDraft proxies PATCH /galgame/:gid (status ∈ {3,4}). Editing
// a declined draft auto-flips it back to pending review.
func (c *Client) PatchGalgameDraft(ctx context.Context, accessToken string, gid int, body any) (json.RawMessage, error) {
	return c.writeUserJSON(ctx, http.MethodPatch,
		fmt.Sprintf("/galgame/%d", gid), accessToken, body)
}

// PatchGalgameDraftMultipart proxies PATCH /galgame/:gid with a new banner file.
func (c *Client) PatchGalgameDraftMultipart(
	ctx context.Context,
	accessToken string,
	gid int,
	jsonBody any,
	fileName string,
	fileContent []byte,
	fileMime string,
) (json.RawMessage, error) {
	return c.writeUserMultipart(ctx, http.MethodPatch,
		fmt.Sprintf("/galgame/%d", gid),
		accessToken, jsonBody, fileName, fileContent, fileMime)
}

// DeleteGalgameDraft proxies DELETE /galgame/:gid (hard delete, status ∈ {3,4}).
func (c *Client) DeleteGalgameDraft(ctx context.Context, accessToken string, gid int) error {
	path := fmt.Sprintf("/galgame/%d", gid)
	base, apiKey := c.writeTarget(path)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, base+path, nil)
	if err != nil {
		return fmt.Errorf("build galgame delete: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("galgame DELETE draft: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read galgame delete response: %w", err)
	}
	var env galgameResponse[json.RawMessage]
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("decode galgame envelope: %w (body=%s)", err, truncate(string(raw), 200))
	}
	if env.Code != 0 {
		return &GalgameError{Code: env.Code, Message: env.Message}
	}
	return nil
}

// ListMyGalgames proxies GET /galgame/mine. Status filter is csv (default "3,4"
// when the caller passes an empty string).
func (c *Client) ListMyGalgames(ctx context.Context, accessToken string, status string, page, limit int) (*Paginated[MineItem], error) {
	q := url.Values{}
	if status != "" {
		q.Set("status", status)
	}
	if page > 0 {
		q.Set("page", strconv.Itoa(page))
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	base, apiKey := c.readTarget("/galgame/mine")
	u := base + "/galgame/mine"
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("build galgame /galgame/mine: %w", err)
	}
	// Dual credential: the user JWT rides Authorization; the service key (when
	// configured) rides X-API-Key on the internal read face.
	req.Header.Set("Authorization", "Bearer "+accessToken)
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("galgame /galgame/mine: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read galgame response: %w", err)
	}
	var env galgameResponse[Paginated[MineItem]]
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("decode galgame envelope: %w (body=%s)", err, truncate(string(raw), 200))
	}
	if env.Code != 0 {
		return nil, &GalgameError{Code: env.Code, Message: env.Message}
	}
	if env.Data.Items == nil {
		env.Data.Items = []MineItem{}
	}
	return &env.Data, nil
}

// SearchGalgameForPublish wraps /galgame/search?include_pending=true. With a
// non-empty access token, galgame decodes the JWT and additionally surfaces the
// caller's own status ∈ {3,4} matches in `pending`. Anonymous calls behave
// like the regular search.
func (c *Client) SearchGalgameForPublish(ctx context.Context, accessToken, q string, limit int) (*SearchPending, error) {
	params := url.Values{}
	if q != "" {
		params.Set("q", q)
	}
	if limit > 0 {
		params.Set("limit", strconv.Itoa(limit))
	}
	params.Set("include_pending", "true")
	// Publish wizard surfaces published games (0) AND claimable VNDB drafts (2);
	// without status=2 it can't find the bulk of the catalog (unclaimed drafts).
	params.Set("status", "0,2")
	params.Set("facets", "false")
	params.Set("highlight", "false")

	base, apiKey := c.readTarget("/galgame/search")
	u := base + "/galgame/search?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("build galgame search: %w", err)
	}
	// Dual credential: the (optional) user JWT rides Authorization to surface
	// the caller's own pending drafts; the service key rides X-API-Key.
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("galgame search: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read galgame response: %w", err)
	}
	var env galgameResponse[SearchPending]
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("decode galgame envelope: %w (body=%s)", err, truncate(string(raw), 200))
	}
	if env.Code != 0 {
		return nil, &GalgameError{Code: env.Code, Message: env.Message}
	}
	// Normalize: a nil slice marshals back to JSON `null`, which crashes
	// frontend code doing `results.pending.length`. Guarantee `[]`.
	if env.Data.Items == nil {
		env.Data.Items = []GalgameHit{}
	}
	if env.Data.Pending == nil {
		env.Data.Pending = []GalgameHit{}
	}
	return &env.Data, nil
}

// GetMyGalgameMessages proxies GET /galgame/messages/mine (Bearer). Used by the
// notification center to merge galgame notifications with local ones.
func (c *Client) GetMyGalgameMessages(ctx context.Context, accessToken string, sinceID int64, limit int) (*Paginated[GalgameMessage], error) {
	q := url.Values{}
	if sinceID > 0 {
		q.Set("since_id", strconv.FormatInt(sinceID, 10))
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	base, apiKey := c.readTarget("/galgame/messages/mine")
	u := base + "/galgame/messages/mine"
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("build galgame /messages/mine: %w", err)
	}
	// Dual credential: the user JWT rides Authorization; the service key (when
	// configured) rides X-API-Key on the internal read face.
	req.Header.Set("Authorization", "Bearer "+accessToken)
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("galgame /messages/mine: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read galgame response: %w", err)
	}
	var env galgameResponse[Paginated[GalgameMessage]]
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("decode galgame envelope: %w (body=%s)", err, truncate(string(raw), 200))
	}
	if env.Code != 0 {
		return nil, &GalgameError{Code: env.Code, Message: env.Message}
	}
	return &env.Data, nil
}

// ─── Service-to-service (internal-tier X-API-Key) ──────

// GalgameMessageFeedResult is the decoded `data` payload of /galgame/messages/feed.
type GalgameMessageFeedResult struct {
	Items   []GalgameMessage `json:"items"`
	HasMore bool             `json:"has_more"`
}

// GetGalgameMessageFeed proxies GET /galgame/messages/feed for the cron job.
// Authenticated via the internal-tier X-API-Key on the internal read face — the
// same key every other read carries (the legacy /api Basic-Auth feed retired in
// open-API phase 2 wave 05).
func (c *Client) GetGalgameMessageFeed(ctx context.Context, sinceID int64, limit int) (*GalgameMessageFeedResult, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("galgame message feed: internal-tier API key not configured (KUN_NEXTMOE_API_KEY)")
	}
	q := url.Values{}
	if sinceID > 0 {
		q.Set("since_id", strconv.FormatInt(sinceID, 10))
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	u := c.internalBase + "/galgame/messages/feed"
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("build galgame /messages/feed: %w", err)
	}
	req.Header.Set("X-API-Key", c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("galgame /messages/feed: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read galgame response: %w", err)
	}
	var env galgameResponse[GalgameMessageFeedResult]
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("decode galgame envelope: %w (body=%s)", err, truncate(string(raw), 200))
	}
	if env.Code != 0 {
		return nil, &GalgameError{Code: env.Code, Message: env.Message}
	}
	return &env.Data, nil
}

// ─── shared write helpers ──────────────────────────────

func (c *Client) writeUserJSON(ctx context.Context, method, path, accessToken string, body any) (json.RawMessage, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encode body: %w", err)
	}
	base, apiKey := c.writeTarget(path)
	req, err := http.NewRequestWithContext(ctx, method, base+path, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build galgame %s %s: %w", method, path, err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("galgame %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read galgame response: %w", err)
	}
	var env galgameResponse[json.RawMessage]
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("decode galgame envelope: %w (body=%s)", err, truncate(string(raw), 200))
	}
	if env.Code != 0 {
		return nil, &GalgameError{Code: env.Code, Message: env.Message}
	}
	return env.Data, nil
}

func (c *Client) writeUserMultipart(
	ctx context.Context,
	method, path, accessToken string,
	jsonBody any,
	fileName string,
	fileContent []byte,
	fileMime string,
) (json.RawMessage, error) {
	payload, err := json.Marshal(jsonBody)
	if err != nil {
		return nil, fmt.Errorf("encode body: %w", err)
	}
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if err := w.WriteField("data", string(payload)); err != nil {
		return nil, fmt.Errorf("write data field: %w", err)
	}
	if len(fileContent) > 0 {
		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition",
			fmt.Sprintf(`form-data; name="file"; filename=%q`, fileName))
		if fileMime != "" {
			h.Set("Content-Type", fileMime)
		}
		fw, err := w.CreatePart(h)
		if err != nil {
			return nil, fmt.Errorf("create file part: %w", err)
		}
		if _, err := fw.Write(fileContent); err != nil {
			return nil, fmt.Errorf("write file part: %w", err)
		}
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	base, apiKey := c.writeTarget(path)
	req, err := http.NewRequestWithContext(ctx, method, base+path, &buf)
	if err != nil {
		return nil, fmt.Errorf("build galgame %s %s: %w", method, path, err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("galgame %s %s (multipart): %w", method, path, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read galgame response: %w", err)
	}
	var env galgameResponse[json.RawMessage]
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("decode galgame envelope: %w (body=%s)", err, truncate(string(raw), 200))
	}
	if env.Code != 0 {
		return nil, &GalgameError{Code: env.Code, Message: env.Message}
	}
	return env.Data, nil
}
