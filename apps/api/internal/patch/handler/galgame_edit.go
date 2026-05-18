package handler

// Galgame editing surface mandated by docs/galgame_wiki/00-handbook-for-downstream.md
// §15 for kungal AND moyu: revisions, PRs, links/aliases/contributors and the
// tag/official/engine/series taxonomy CRUD. Every endpoint is a verbatim proxy
// to the Wiki Service:
//
//   - Route paths mirror Wiki 1:1 (sans the /api/v1 prefix), so the Wiki path
//     is derived by stripping /api/v1 from the original URL — there are no
//     per-route path templates to keep in sync.
//   - Reads are public (optionalAuth: a token is forwarded only if the caller
//     happens to be logged in). Writes require login (auth) so a Bearer token
//     exists to forward; Wiki enforces creator/admin/role rules and we forward
//     its business code+message verbatim. We deliberately do NOT re-implement
//     authorization locally — §15: "鉴权语义以 wiki 端为准，下游不得放宽或收紧".

import (
	"io"
	"strings"

	"kun-galgame-patch-api/internal/middleware"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"

	"github.com/gofiber/fiber/v2"
)

const apiV1Prefix = "/api/v1"

// wikiPathFromRequest turns the incoming original URL into the Wiki path,
// preserving exact path/query encoding. c.OriginalURL() keeps the raw,
// undecoded path+query — important for the cosmetic non-ASCII /tag/:name
// segment (Wiki ignores :name and queries by tag_id, but the URL must stay
// syntactically valid).
func wikiPathFromRequest(c *fiber.Ctx) string {
	return strings.TrimPrefix(c.OriginalURL(), apiV1Prefix)
}

// WikiEditProxy is the generic GET / JSON-write pass-through used by every
// §15 endpoint except the multipart-capable PR submit (see WikiPRSubmit).
func (h *PatchHandler) WikiEditProxy(c *fiber.Ctx) error {
	method := c.Method()
	accessToken := middleware.GetAccessToken(c)
	if method != fiber.MethodGet && accessToken == "" {
		return response.Error(c, errors.ErrUnauthorized())
	}

	var body []byte
	if method != fiber.MethodGet {
		body = c.Body()
	}
	data, err := h.wiki.Proxy(
		c.Context(),
		method,
		wikiPathFromRequest(c),
		accessToken,
		body,
		string(c.Request().Header.ContentType()),
	)
	return writeWikiResult(c, data, err)
}

// WikiPRSubmit handles POST /api/v1/galgame/:gid/prs. Like Submit/Update it
// accepts JSON or multipart/form-data (`data` JSON + optional `file` banner)
// so a PR proposal can carry a new banner thumbnail for the reviewer
// (docs/galgame_wiki/02-revisions-and-prs.md §PR).
func (h *PatchHandler) WikiPRSubmit(c *fiber.Ctx) error {
	accessToken := middleware.GetAccessToken(c)
	if accessToken == "" {
		return response.Error(c, errors.ErrUnauthorized())
	}
	wikiPath := wikiPathFromRequest(c)
	ctype := string(c.Request().Header.ContentType())

	if !strings.HasPrefix(ctype, "multipart/form-data") {
		data, err := h.wiki.Proxy(
			c.Context(), fiber.MethodPost, wikiPath, accessToken, c.Body(), ctype,
		)
		return writeWikiResult(c, data, err)
	}

	form, err := c.MultipartForm()
	if err != nil {
		return response.Error(c, errors.ErrBadRequest("multipart 表单解析失败"))
	}
	dataStrs := form.Value["data"]
	if len(dataStrs) == 0 {
		return response.Error(c, errors.ErrBadRequest("缺少 data 字段"))
	}
	dataJSON := []byte(dataStrs[0])

	var fileName, fileMime string
	var fileBytes []byte
	if fhs := form.File["file"]; len(fhs) > 0 {
		fh := fhs[0]
		if fh.Size > 10*1024*1024 {
			return response.Error(c, errors.ErrBadRequest("banner 超过 10MB 上限"))
		}
		f, oerr := fh.Open()
		if oerr != nil {
			return response.Error(c, errors.ErrBadRequest("无法读取上传文件"))
		}
		defer f.Close()
		b, rerr := io.ReadAll(f)
		if rerr != nil {
			return response.Error(c, errors.ErrBadRequest("读取上传文件失败"))
		}
		fileBytes = b
		fileName = fh.Filename
		fileMime = fh.Header.Get("Content-Type")
		if fileMime == "" {
			fileMime = "application/octet-stream"
		}
	}

	data, callErr := h.wiki.ProxyMultipart(
		c.Context(), fiber.MethodPost, wikiPath, accessToken,
		dataJSON, fileName, fileBytes, fileMime,
	)
	return writeWikiResult(c, data, callErr)
}
