package handler

import (
	"encoding/json"
	"log/slog"
	"strconv"

	"kun-galgame-patch-api/internal/middleware"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"

	"github.com/gofiber/fiber/v3"
)

// OAuth's house code for "this access token does not carry the `preferences`
// scope". The grant is frozen at /oauth/authorize, so a session minted before
// moyu asked for the scope can never gain it by refreshing.
const oauthScopeRequiredCode = 18001

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

	status, raw, err := h.service.ProxyUserToOAuth(
		fiber.MethodPut, "/auth/me/nsfw", token, c.Body(), "application/json")
	if err != nil {
		slog.Error("OAuth nsfw preference proxy failed", "error", err)
		return response.Error(c, errors.ErrInternal("OAuth 服务不可达"))
	}
	if upstreamCode(raw) == oauthScopeRequiredCode {
		return response.Error(c, errors.ErrPreferencesReauthRequired(""))
	}
	return sendUpstream(c, status, raw)
}

func (h *AuthHandler) GetPreferences(c fiber.Ctx) error {
	token, path, appErr := h.preferencesTarget(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}

	status, raw, err := h.service.ProxyUserToOAuth(fiber.MethodGet, path, token, nil, "")
	if err != nil {
		slog.Error("OAuth preferences read proxy failed", "error", err)
		return response.Error(c, errors.ErrPreferencesUnavailable(""))
	}
	if upstreamCode(raw) == oauthScopeRequiredCode {
		return response.Error(c, errors.ErrPreferencesUnavailable(""))
	}
	return sendUpstream(c, status, raw)
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

	status, raw, err := h.service.ProxyUserToOAuthWithHeaders(
		fiber.MethodPut, path, token, body, "application/json", headers)
	if err != nil {
		slog.Error("OAuth preferences write proxy failed", "error", err)
		return response.Error(c, errors.ErrPreferencesUnavailable(""))
	}
	if upstreamCode(raw) == oauthScopeRequiredCode {
		return response.Error(c, errors.ErrPreferencesUnavailable(""))
	}
	return sendUpstream(c, status, raw)
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

func upstreamCode(raw []byte) int {
	var env struct {
		Code int `json:"code"`
	}
	if json.Unmarshal(raw, &env) != nil {
		return 0
	}
	return env.Code
}

// Everything else — 18006 on a version clash, 18008 when the account never
// attested its age, 18007 on a bad value — reaches the browser verbatim,
// because each of those is a different thing for the page to say.
func sendUpstream(c fiber.Ctx, status int, raw []byte) error {
	c.Set("Content-Type", "application/json")
	c.Set("Cache-Control", "no-store")
	return c.Status(status).Send(raw)
}
