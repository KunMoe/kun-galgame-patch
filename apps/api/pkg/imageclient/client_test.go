package imageclient

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"kun-galgame-patch-api/pkg/upstream"
)

func fake(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return New(Config{BaseURL: srv.URL, ClientID: "moyu", ClientSecret: "secret"})
}

func answering(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Request-ID", "req-9")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

// Bodies are what infra's image handler writes (internal/platform/image/handler/
// handler.go Upload + respondUploadError, cmd/image uploadGate, middleware/auth.go).
func TestUploadFailureKinds(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		kind   upstream.Kind
		cause  error
	}{
		{"too large", 413, `{"code":80007,"message":"文件大小超过限制"}`, upstream.Rejected, ErrFileTooLarge},
		{"format denied", 400, `{"code":80009,"message":"该 preset 不接受此格式"}`, upstream.Rejected, ErrMIMEDenied},
		{"undecodable", 400, `{"code":80010,"message":"图片解码失败"}`, upstream.Rejected, ErrDecodeFailed},
		{"moderation", 422, `{"code":60002,"message":"内容审核未通过"}`, upstream.Rejected, ErrModerationRejected},
		{"site quota", 429, `{"code":80008,"message":"当日配额已用完","details":{"used_count":500}}`, upstream.RateLimited, ErrQuotaExceeded},
		{"upload switched off", 503, `{"code":80015,"message":"图片上传功能暂未开放，敬请期待"}`, upstream.Unavailable, ErrUploadDisabled},
		{"preset denied is moyu's allowlist", 403, `{"code":80006,"message":"该站点不允许使用此 preset"}`, upstream.Internal, nil},
		{"unknown preset is moyu's", 400, `{"code":80011,"message":"未知的 preset"}`, upstream.Internal, nil},
		{"bad secret", 401, `{"code":80003,"message":"客户端密钥错误"}`, upstream.Internal, nil},
		{"store failure", 500, `{"code":80012,"message":"图片存储失败"}`, upstream.Unavailable, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := fake(t, answering(tc.status, tc.body))
			_, err := c.Upload(context.Background(), strings.NewReader("x"), "a.png", "image/png", "topic")
			e, ok := upstream.As(err)
			if !ok {
				t.Fatalf("err = %v, want an upstream.Error", err)
			}
			if e.Kind != tc.kind {
				t.Errorf("kind = %d, want %d", e.Kind, tc.kind)
			}
			if tc.cause != nil && !errors.Is(err, tc.cause) {
				t.Errorf("err = %v, want it to wrap %v", err, tc.cause)
			}
			if e.RequestID != "req-9" {
				t.Errorf("request id = %q", e.RequestID)
			}
		})
	}
}

func TestUploadDecodesTheEnvelope(t *testing.T) {
	c := fake(t, answering(200, `{"code":0,"message":"成功","data":{"hash":"ab12","url":"https://cdn/ab/12/ab12.webp","width":3,"height":4}}`))
	got, err := c.Upload(context.Background(), strings.NewReader("x"), "a.png", "image/png", "topic")
	if err != nil {
		t.Fatal(err)
	}
	if got.Hash != "ab12" || got.Width != 3 || got.VariantURLs == nil {
		t.Fatalf("got %+v", got)
	}
}

const hashA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
const hashB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

func TestMetaResolverStopsAskingDuringAnOutage(t *testing.T) {
	var calls atomic.Int32
	c := fake(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		answering(502, `<html>Bad Gateway</html>`)(w, r)
	})
	r := c.NewMetaResolver(time.Second)
	now := time.Unix(1_000_000, 0)
	r.now = func() time.Time { return now }

	r.Resolve([]string{hashA})
	r.Resolve([]string{hashB})
	if calls.Load() != 1 {
		t.Fatalf("calls = %d, want 1: a render during the outage must not wait on it", calls.Load())
	}
	now = now.Add(metaOutagePause)
	r.Resolve([]string{hashB})
	if calls.Load() != 2 {
		t.Fatalf("calls = %d, want the resolver to try again after the pause", calls.Load())
	}
}

func TestMetaResolverCachesMissesForAWhile(t *testing.T) {
	var calls atomic.Int32
	c := fake(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		answering(200, `{"code":0,"message":"成功","data":{"metas":{"`+hashA+`":{"width":10,"height":20}}}}`)(w, r)
	})
	r := c.NewMetaResolver(time.Second)
	now := time.Unix(1_000_000, 0)
	r.now = func() time.Time { return now }

	got := r.Resolve([]string{hashA, hashB})
	if got[hashA].Width != 10 {
		t.Fatalf("got %+v", got)
	}
	if _, ok := got[hashB]; ok {
		t.Fatal("an unknown hash must not be answered")
	}
	r.Resolve([]string{hashA, hashB})
	if calls.Load() != 1 {
		t.Fatalf("calls = %d, want the missing hash and the thumbhash-less one cached", calls.Load())
	}
	now = now.Add(metaRetryAfter)
	r.Resolve([]string{hashA, hashB})
	if calls.Load() != 2 {
		t.Fatalf("calls = %d, want both asked again once the backfill may have run", calls.Load())
	}
}
