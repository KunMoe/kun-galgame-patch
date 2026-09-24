package trustclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"kun-galgame-patch-api/pkg/upstream"
)

func fake(t *testing.T, status int, contentType, body string) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("X-Request-ID", "req-3")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return New(Config{BaseURL: srv.URL, ClientID: "moyu", ClientSecret: "secret"})
}

func submit(c *Client) error {
	_, err := c.SubmitReport(context.Background(), ReportRequest{SubjectKind: "patch_resource", SubjectID: "7", ReasonKey: "spam", ReporterID: 1})
	return err
}

func decide(c *Client) error {
	_, err := c.DecideReviewItem(context.Background(), "tok", 9, []byte(`{}`))
	return err
}

// Bodies are what infra's trust service writes: handler/s2s.go mapIntakeErr and
// auth.go siteBinding / S2SAuth for reports; middleware/jwt_auth.go,
// middleware/permission.go and handler/admin.go for the review queue.
func TestFailureKinds(t *testing.T) {
	const jsonCT = "application/json"
	for _, tc := range []struct {
		name   string
		status int
		ct     string
		body   string
		call   func(*Client) error
		kind   upstream.Kind
	}{
		{"an unregistered subject kind is moyu's onboarding", 422, jsonCT,
			`{"code":7,"message":"subject_kind is not registered for this site"}`, submit, upstream.Internal},
		{"an unknown reason is the reader's", 422, jsonCT,
			`{"code":7,"message":"unknown report reason for this site"}`, submit, upstream.Rejected},
		{"a bad subject link is the reader's", 422, jsonCT,
			`{"code":7,"message":"subject_url must be an absolute http(s) link of at most 512 chars"}`, submit, upstream.Rejected},
		{"the reporter's rate limit", 429, jsonCT,
			`{"code":10,"message":"reporter rate limit exceeded; try again later"}`, submit, upstream.RateLimited},
		{"an unbound client is moyu's configuration", 403, jsonCT,
			`{"code":5,"message":"client is not bound to a site; it cannot submit reports"}`, submit, upstream.Internal},
		{"a rejected client credential", 401, jsonCT,
			`{"code":10001,"message":"未授权，请先登录"}`, submit, upstream.Internal},
		{"the moderator's token failing trust's verification never logs them out", 401, jsonCT,
			`{"code":10003,"message":"令牌已过期，请重新登录"}`, decide, upstream.Internal},
		{"trust's key set is unreachable", 503, jsonCT,
			`{"code":10,"message":"令牌验证暂不可用，请稍后重试"}`, decide, upstream.Unavailable},
		{"a moderator without the queue permission", 403, jsonCT,
			`{"code":5,"message":"访问被拒绝"}`, decide, upstream.Rejected},
		{"a site-scoped moderator on an unbound client", 403, jsonCT,
			`{"code":5,"message":"site-scoped moderator's client is not bound to a site"}`, decide, upstream.Internal},
		{"an invalid decision", 400, jsonCT, `{"code":7,"message":"trust: invalid decision"}`, decide, upstream.Rejected},
		{"already decided", 409, jsonCT, `{"code":10,"message":"trust: illegal review-item state transition"}`, decide, upstream.Conflict},
		{"no such item", 404, jsonCT, `{"code":4,"message":"资源不存在"}`, decide, upstream.NotFound},
		{"a proxy's 502 page", 502, "text/html", `<html>Bad Gateway</html>`, decide, upstream.Unavailable},
		{"a proxy's 404 page is the wrong origin", 404, "text/html", `<html>Not Found</html>`, submit, upstream.Internal},
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
			if e.RequestID != "req-3" {
				t.Errorf("request id = %q", e.RequestID)
			}
		})
	}
}

func TestSubmitReportDecodesTheResult(t *testing.T) {
	c := fake(t, 200, "application/json", `{"code":0,"message":"成功","data":{"report_id":41,"review_item_id":8}}`)
	res, err := c.SubmitReport(context.Background(), ReportRequest{SubjectKind: "patch_resource", SubjectID: "7", ReasonKey: "spam", ReporterID: 1})
	if err != nil {
		t.Fatal(err)
	}
	if res.ReportID != 41 {
		t.Fatalf("report id = %d", res.ReportID)
	}
}
