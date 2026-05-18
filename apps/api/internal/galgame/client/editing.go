package client

// This file implements the generic pass-through used by the galgame editing
// surface that docs/galgame_wiki/00-handbook-for-downstream.md §15 makes
// MANDATORY for kungal AND moyu: revisions, PRs, links/aliases/contributors
// and the tag/official/engine/series taxonomy CRUD.
//
// Unlike Submit/Claim (which have local side effects — galgame_stats /
// moemoepoint), every §15 endpoint is a pure relay: forward the request
// verbatim, return Wiki's `data` unchanged, and surface Wiki's business
// code+message via *WikiError so the handler can forward it to the frontend.
// Wiki itself validates the JWT and enforces creator/admin/role rules — the
// site backend never re-implements authorization (it would drift; see §15
// "鉴权语义以 wiki 端为准，下游不得放宽或收紧").

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
)

// Proxy relays a GET / JSON-write to the Wiki Service.
//
// pathAndQuery is the Wiki path WITH its leading slash and any query string,
// e.g. "/galgame/8329/revisions?page=2" or "/tag/%E6%A0%A1%E5%9B%AD?tag_id=42"
// (already URL-encoded — pass it through untouched). accessToken is forwarded
// as a Bearer header when non-empty (public GETs may pass ""). body is the raw
// request body to relay (nil for GET / body-less DELETE); contentType defaults
// to application/json when a body is present.
func (c *Client) Proxy(
	ctx context.Context,
	method, pathAndQuery, accessToken string,
	body []byte,
	contentType string,
) (json.RawMessage, error) {
	var rdr io.Reader
	if len(body) > 0 {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+pathAndQuery, rdr)
	if err != nil {
		return nil, fmt.Errorf("build wiki %s %s: %w", method, pathAndQuery, err)
	}
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}
	if len(body) > 0 {
		if contentType == "" {
			contentType = "application/json"
		}
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("Accept", "application/json")
	return c.doEnvelope(req, method, pathAndQuery)
}

// ProxyMultipart relays a multipart write (a `data` JSON part + an optional
// `file` part) — used by POST /galgame/:gid/prs so a PR proposal can carry a
// new banner thumbnail for the reviewer (docs/galgame_wiki/02-revisions-and-prs.md
// §PR, same convention as Create/Update in 01-galgame.md §Banner 上传).
func (c *Client) ProxyMultipart(
	ctx context.Context,
	method, pathAndQuery, accessToken string,
	dataJSON []byte,
	fileName string,
	fileContent []byte,
	fileMime string,
) (json.RawMessage, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if err := w.WriteField("data", string(dataJSON)); err != nil {
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

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+pathAndQuery, &buf)
	if err != nil {
		return nil, fmt.Errorf("build wiki %s %s: %w", method, pathAndQuery, err)
	}
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Accept", "application/json")
	return c.doEnvelope(req, method, pathAndQuery)
}

// doEnvelope executes req, decodes the standard {code,message,data} envelope
// and returns the raw `data` (so the handler can re-wrap without an extra
// unmarshal) or *WikiError on a non-zero envelope code.
func (c *Client) doEnvelope(req *http.Request, method, pathAndQuery string) (json.RawMessage, error) {
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("wiki %s %s: %w", method, pathAndQuery, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read wiki response: %w", err)
	}
	var env wikiResponse[json.RawMessage]
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("decode wiki envelope: %w (body=%s)", err, truncate(string(raw), 200))
	}
	if env.Code != 0 {
		return nil, &WikiError{Code: env.Code, Message: env.Message}
	}
	return env.Data, nil
}
