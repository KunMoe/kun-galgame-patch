package im

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"kun-galgame-patch-api/internal/middleware"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"

	"github.com/gofiber/fiber/v3"
)

// Relay forwards /api/v1/im/* to chat's /v2/chat/* as the signed-in user. moyu
// keeps no copy of any message.
type Relay struct {
	baseURL string
	http    *http.Client
}

func NewRelay(baseURL string) *Relay {
	return &Relay{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

const responseLimit = 8 << 20

var problemMessages = map[string]string{
	"CHAT_BLOCKED":            "你们之间有拉黑关系，无法发送私信",
	"CHAT_NOT_ACCEPTING":      "对方暂不接受你的私信",
	"CHAT_REQUEST_LIMIT":      "对方接受私信请求前，你只能发最多 3 条不带链接和图片的文字消息",
	"CHAT_EDIT_WINDOW_CLOSED": "这条消息已超过可编辑的时间",
	"CHAT_NOT_PERMITTED":      "你不能对这条消息这样做",
	"NOT_FOUND":               "会话或消息不存在",
	"QUOTA_EXCEEDED":          "今天的图片额度已用完",
	"PAYLOAD_TOO_LARGE":       "图片太大了",
	"VALIDATION_FAILED":       "内容不符合要求",
	"RATE_LIMITED":            "操作太频繁，请稍后再试",
}

func (r *Relay) Forward(c fiber.Ctx) error {
	token := middleware.GetAccessToken(c)
	if token == "" {
		return response.Error(c, errors.ErrUnauthorized())
	}
	if r.baseURL == "" {
		return response.Error(c, errors.ErrChatUnavailable())
	}
	// chat's process also serves internal routes such as /trust/callback.
	rest := c.Params("*")
	if plain, err := url.PathUnescape(rest); err != nil || plain == "" ||
		strings.Contains(plain, "..") || strings.ContainsAny(plain, "?#\\") {
		return response.Error(c, errors.ErrNotFound(""))
	}
	query, err := url.ParseQuery(string(c.Request().URI().QueryString()))
	if err != nil {
		return response.Error(c, errors.ErrBadRequest(""))
	}
	// useApi appends both to every request.
	query.Del("content_limit")
	query.Del("include_empty")
	target := r.baseURL + "/v2/chat/" + rest
	if len(query) > 0 {
		target += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(c.Context(), c.Method(), target, bytes.NewReader(c.Body()))
	if err != nil {
		return response.Error(c, errors.ErrBadRequest(""))
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	if ct := c.Get(fiber.HeaderContentType); ct != "" && len(c.Body()) > 0 {
		req.Header.Set("Content-Type", ct)
	}
	if key := c.Get("Idempotency-Key"); key != "" {
		req.Header.Set("Idempotency-Key", key)
	}

	resp, err := r.http.Do(req)
	if err != nil {
		slog.Warn("chat relay failed", "path", rest, "error", err)
		return response.Error(c, errors.ErrChatUnavailable())
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, responseLimit))
	if err != nil {
		return response.Error(c, errors.ErrChatUnavailable())
	}

	if resp.StatusCode < 300 {
		env := response.Response{Code: 0, Message: "OK"}
		if json.Valid(raw) {
			env.Data = json.RawMessage(raw)
		}
		// A 204 cannot carry the envelope: the browser drops the body and the
		// page reads undefined, which is how letmoe's 接受 button broke.
		status := resp.StatusCode
		if status == http.StatusNoContent {
			status = http.StatusOK
		}
		return c.Status(status).JSON(env)
	}
	if ra := resp.Header.Get("Retry-After"); ra != "" {
		c.Set("Retry-After", ra)
	}
	return c.Status(resp.StatusCode).JSON(translateProblem(resp.StatusCode, raw))
}

func translateProblem(status int, raw []byte) response.Response {
	var p struct {
		Code string `json:"code"`
	}
	if !json.Valid(raw) || json.Unmarshal(raw, &p) != nil {
		if status >= 500 {
			down := errors.ErrChatUnavailable()
			return response.Response{Code: down.Code, Message: down.Message}
		}
		return response.Response{Code: status * 100, Message: "私信请求失败"}
	}
	var mapped *errors.AppError
	switch {
	case p.Code == "SCOPE_REQUIRED":
		mapped = errors.ErrChatScopeMissing()
	case status == http.StatusUnauthorized:
		mapped = errors.ErrAuthExpired()
	case status >= 500:
		mapped = errors.ErrChatUnavailable()
	default:
		msg, ok := problemMessages[p.Code]
		if !ok {
			msg = "私信请求失败"
		}
		mapped = errors.New(status*100, msg, status)
	}
	return response.Response{Code: mapped.Code, Message: mapped.Message, Data: json.RawMessage(raw)}
}
