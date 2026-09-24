package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"kun-galgame-patch-api/internal/middleware"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"
	"kun-galgame-patch-api/pkg/upstream"

	"github.com/gofiber/fiber/v3"
)

// OAuth's house codes on the preference faces (infra preference_handler.go
// preferenceGate). 18001 is "this access token does not carry the `preferences`
// scope": the grant is frozen at /oauth/authorize, so a session minted before
// moyu asked for the scope can never gain it by refreshing. 18002 and 18003
// refuse the namespace, which moyu names itself.
const (
	oauthScopeRequiredCode    = 18001
	oauthNamespaceInvalidCode = 18002
	oauthNamespaceDeniedCode  = 18003
)

type preferencesRequest struct {
	Doc json.RawMessage `json:"doc"`
	// The version the client read, echoed back as If-Match. `0` is a real value
	// — it claims the first write of a namespace nobody has written yet — so a
	// pointer is what distinguishes it from "send no If-Match at all".
	Version *int64 `json:"version"`
}

func (h *AuthHandler) UpdateNsfwDisplay(c fiber.Ctx) error {
	token := middleware.GetAccessToken(c)
	if token == "" {
		return response.Error(c, errors.ErrUnauthorized())
	}

	status, raw, err := h.service.ProxyUserToOAuth(c.Context(),
		fiber.MethodPut, "/auth/me/nsfw", token, c.Body(), "application/json")
	if err != nil {
		return response.Upstream(c, err, "")
	}
	if upstreamEnvelope(raw).Code == oauthScopeRequiredCode {
		return response.Error(c, errors.ErrPreferencesReauthRequired(""))
	}
	return relayOAuth(c, "PUT /auth/me/nsfw", status, raw)
}

func (h *AuthHandler) GetPreferences(c fiber.Ctx) error {
	token, path, appErr := h.preferencesTarget(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}

	status, raw, err := h.service.ProxyUserToOAuth(c.Context(), fiber.MethodGet, path, token, nil, "")
	if err != nil {
		slog.Warn("OAuth preferences read proxy failed", "error", err)
		return response.Error(c, errors.ErrPreferencesUnavailable(""))
	}
	if upstreamEnvelope(raw).Code == oauthScopeRequiredCode {
		return response.Error(c, errors.ErrPreferencesUnavailable(""))
	}
	return relayOAuth(c, "GET /auth/me/preferences", status, raw)
}

func (h *AuthHandler) UpdatePreferences(c fiber.Ctx) error {
	token, path, appErr := h.preferencesTarget(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}

	var req preferencesRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Error(c, errors.ErrBadRequest("偏好内容不是合法的 JSON"))
	}

	headers := map[string]string{}
	if req.Version != nil {
		headers["If-Match"] = strconv.Quote(strconv.FormatInt(*req.Version, 10))
	}
	body, err := json.Marshal(map[string]any{"doc": req.Doc})
	if err != nil {
		return response.Error(c, errors.ErrBadRequest("偏好内容无法序列化"))
	}

	status, raw, err := h.service.ProxyUserToOAuthWithHeaders(c.Context(),
		fiber.MethodPut, path, token, body, "application/json", headers)
	if err != nil {
		slog.Warn("OAuth preferences write proxy failed", "error", err)
		return response.Error(c, errors.ErrPreferencesUnavailable(""))
	}
	if upstreamEnvelope(raw).Code == oauthScopeRequiredCode {
		return response.Error(c, errors.ErrPreferencesUnavailable(""))
	}
	return relayOAuth(c, "PUT /auth/me/preferences", status, raw)
}

// The namespace is resolved here and never accepted from the client: it is
// moyu's own OAuth client_id, and upstream 403s on any other name anyway.
func (h *AuthHandler) preferencesTarget(c fiber.Ctx) (token, path string, appErr *errors.AppError) {
	token = middleware.GetAccessToken(c)
	if token == "" {
		return "", "", errors.ErrUnauthorized()
	}
	namespace := h.service.PreferencesNamespace()
	if namespace == "" {
		slog.Warn("preferences namespace unavailable: KUN_OAUTH_CLIENT_ID is empty")
		return "", "", errors.ErrPreferencesUnavailable("")
	}
	return token, "/auth/me/preferences/" + namespace, nil
}

type oauthEnvelope struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func upstreamEnvelope(raw []byte) oauthEnvelope {
	var env oauthEnvelope
	_ = json.Unmarshal(raw, &env)
	return env
}

// relayOAuth hands the browser OAuth's own answer when it is about what the
// reader sent — 18006 on a version clash, 18007 on a bad value, 10014 for a
// banned account — because each is a different thing for the page to say. The
// rest are moyu's to answer: a 401 refuses a token moyu has just judged live, a
// 404 is an origin moyu is pointed at, and 18002/18003 refuse the namespace
// moyu names. Relayed verbatim, a 401 reached the page as OAuth's own code.
func relayOAuth(c fiber.Ctx, op string, status int, raw []byte) error {
	env := upstreamEnvelope(raw)
	var kind upstream.Kind
	switch {
	case status >= http.StatusInternalServerError:
		kind = upstream.Unavailable
	case status == http.StatusTooManyRequests:
		kind = upstream.RateLimited
	case status == http.StatusUnauthorized, status == http.StatusNotFound,
		env.Code == oauthNamespaceInvalidCode, env.Code == oauthNamespaceDeniedCode:
		kind = upstream.Internal
	default:
		return sendUpstream(c, status, raw)
	}
	return response.Upstream(c, &upstream.Error{
		Service: "oauth", Op: op, Kind: kind, Status: status,
		Code: strconv.Itoa(env.Code), Detail: env.Message,
	}, "")
}

func sendUpstream(c fiber.Ctx, status int, raw []byte) error {
	c.Set("Content-Type", "application/json")
	c.Set("Cache-Control", "no-store")
	return c.Status(status).Send(raw)
}
