package artifactclient

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"kun-galgame-patch-api/pkg/upstream"
)

func fake(t *testing.T, status int, contentType, body string) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("X-Request-ID", "req-7")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return New(Config{BaseURL: srv.URL, ClientID: "moyu", ClientSecret: "secret"})
}

// Bodies are what infra's artifact handler writes (internal/platform/artifact/
// handler/huma_api.go and middleware/auth.go): the house {code, message} with
// the codes from pkg/errors/codes.go.
func TestFailureKinds(t *testing.T) {
	const jsonCT = "application/json"
	for _, tc := range []struct {
		name   string
		status int
		ct     string
		body   string
		call   func(*Client) error
		kind   upstream.Kind
		cause  error
	}{
		{"init: file too big is the reader's", 413, jsonCT, `{"code":50004,"message":"文件过大"}`, initCall, upstream.Rejected, ErrTooBig},
		{"init: mime denied is the reader's", 400, jsonCT, `{"code":50017,"message":"该站点不接受此文件类型"}`, initCall, upstream.Rejected, ErrMIMEDenied},
		{"init: site quota", 429, jsonCT, `{"code":50012,"message":"当日配额已用完"}`, initCall, upstream.RateLimited, ErrQuotaExceeded},
		{"init: upload switched off", 503, jsonCT, `{"code":50014,"message":"制品上传功能暂未开放，敬请期待"}`, initCall, upstream.Unavailable, ErrUploadDisabled},
		{"bad secret is moyu's credential", 401, jsonCT, `{"code":50008,"message":"客户端密钥错误"}`, initCall, upstream.Internal, nil},
		{"site disabled is moyu's configuration", 403, jsonCT, `{"code":50009,"message":"该站点未开启制品服务"}`, initCall, upstream.Internal, nil},
		{"complete: size mismatch", 400, jsonCT, `{"code":50015,"message":"上传文件大小与声明不符"}`, completeCall, upstream.Rejected, ErrSizeMismatch},
		{"resume: already finished", 409, jsonCT, `{"code":50011,"message":"请求格式错误"}`, resumeCall, upstream.Conflict, nil},
		{"download: gone", 404, jsonCT, `{"code":50001,"message":"资源不存在"}`, downloadCall, upstream.NotFound, ErrNotFound},
		{"store failure", 500, jsonCT, `{"code":50013,"message":"制品存储失败"}`, downloadCall, upstream.Unavailable, nil},
		{"a proxy's 502 page", 502, "text/html", `<html>Bad Gateway</html>`, downloadCall, upstream.Unavailable, nil},
		{"a proxy's 404 page is the wrong origin", 404, "text/html", `<html>Not Found</html>`, downloadCall, upstream.Internal, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call(fake(t, tc.status, tc.ct, tc.body))
			e, ok := upstream.As(err)
			if !ok {
				t.Fatalf("err = %v, want an upstream.Error", err)
			}
			if e.Kind != tc.kind {
				t.Errorf("kind = %d, want %d (%v)", e.Kind, tc.kind, err)
			}
			if tc.cause != nil && !errors.Is(err, tc.cause) {
				t.Errorf("err = %v, want it to wrap %v", err, tc.cause)
			}
			if e.RequestID != "req-7" {
				t.Errorf("request id = %q", e.RequestID)
			}
		})
	}
}

func TestResumeDecodesUploadedParts(t *testing.T) {
	c := fake(t, 200, "application/json", `{"code":0,"message":"成功","data":{
		"uuid":"u1","multipart":true,"expires_at":"2026-09-24T12:00:00Z","part_size":8388608,
		"part_urls":[{"part_number":2,"url":"https://s.example/2"}],
		"uploaded_parts":[{"part_number":1,"etag":"\"e1\"","size":8388608}]}}`)
	got, err := c.Resume(context.Background(), "u1")
	if err != nil {
		t.Fatal(err)
	}
	if got.UploadedParts == nil || len(*got.UploadedParts) != 1 || (*got.UploadedParts)[0].Etag != `"e1"` {
		t.Fatalf("uploaded parts = %+v", got.UploadedParts)
	}
	if got.PartUrls == nil || (*got.PartUrls)[0].PartNumber != 2 {
		t.Fatalf("part urls = %+v", got.PartUrls)
	}
}

func TestUnconfigured(t *testing.T) {
	c := New(Config{BaseURL: "http://artifact.test"})
	if c.Configured() {
		t.Fatal("no credentials must leave the client unconfigured")
	}
	if _, err := c.Download(context.Background(), "u1"); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("err = %v, want ErrNotConfigured", err)
	}
}

func initCall(c *Client) error {
	_, err := c.InitUpload(context.Background(), InitUploadRequest{Name: "a.zip", FileSize: 1})
	return err
}

func completeCall(c *Client) error {
	_, err := c.CompleteUpload(context.Background(), "u1", CompleteUploadRequest{})
	return err
}

func resumeCall(c *Client) error {
	_, err := c.Resume(context.Background(), "u1")
	return err
}

func downloadCall(c *Client) error {
	_, err := c.Download(context.Background(), "u1")
	return err
}
