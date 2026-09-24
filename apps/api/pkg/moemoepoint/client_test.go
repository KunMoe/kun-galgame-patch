package moemoepoint

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"kun-galgame-patch-api/pkg/upstream"
)

func newTestClient(srv *httptest.Server) *Client {
	return New(Config{BaseURL: srv.URL, ClientID: "moyu", ClientSecret: "secret"})
}

func TestAdjust_Success(t *testing.T) {
	var gotBody AdjustRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/users/42/moemoepoint" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if auth := r.Header.Get("Authorization"); auth == "" || auth[:6] != "Basic " {
			t.Errorf("missing Basic auth: %q", auth)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 0, "message": "成功",
			"data": map[string]any{"user_id": 42, "balance": 45, "applied": true},
		})
	}))
	defer srv.Close()

	res, err := newTestClient(srv).Adjust(context.Background(), 42, AdjustRequest{
		Delta: 3, Reason: "content_approved", Ref: "resource:7", IdempotencyKey: "moyu:resource_publish:7",
	})
	if err != nil {
		t.Fatalf("Adjust error: %v", err)
	}
	if res.Balance != 45 || !res.Applied {
		t.Fatalf("got %+v, want balance=45 applied=true", res)
	}
	if gotBody.Delta != 3 || gotBody.Reason != "content_approved" || gotBody.IdempotencyKey != "moyu:resource_publish:7" {
		t.Fatalf("server received wrong body: %+v", gotBody)
	}
	if gotBody.ActorUserID != 0 {
		t.Fatalf("actor_user_id should default to 0, got %d", gotBody.ActorUserID)
	}
}

func TestAdjust_IdempotentReplay(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 0, "data": map[string]any{"user_id": 42, "balance": 45, "applied": false},
		})
	}))
	defer srv.Close()

	res, err := newTestClient(srv).Adjust(context.Background(), 42, AdjustRequest{Delta: 3, Reason: "content_approved", IdempotencyKey: "k"})
	if err != nil {
		t.Fatalf("Adjust error: %v", err)
	}
	if res.Applied {
		t.Fatalf("expected applied=false on replay")
	}
}

func TestAdjust_BusinessError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 16004, "message": "幂等键冲突"})
	}))
	defer srv.Close()

	if _, err := newTestClient(srv).Adjust(context.Background(), 42, AdjustRequest{Delta: 3, Reason: "content_approved", IdempotencyKey: "k"}); err == nil {
		t.Fatal("expected error on code=16004")
	}
}

func TestBalance(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{"balance": 99}})
	}))
	defer srv.Close()

	bal, err := newTestClient(srv).Balance(context.Background(), 42)
	if err != nil {
		t.Fatalf("Balance error: %v", err)
	}
	if bal != 99 {
		t.Fatalf("got %d, want 99", bal)
	}
}

func TestLog(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/users/42/moemoepoint/log" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if got := r.URL.Query().Get("limit"); got != "20" {
			t.Errorf("limit query = %q, want 20", got)
		}
		if got := r.URL.Query().Get("before_id"); got != "100" {
			t.Errorf("before_id query = %q, want 100", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 0, "data": map[string]any{
				"items": []map[string]any{
					{"id": 99, "delta": 3, "reason": "content_approved", "source_app": "moyu", "ref": "resource:7", "created_at": "2026-05-29T10:00:00Z"},
					{"id": 98, "delta": -1, "reason": "liked", "source_app": "moyu", "ref": "comment:3", "created_at": "2026-05-28T09:00:00Z"},
				},
				"has_more": true,
			},
		})
	}))
	defer srv.Close()

	items, hasMore, err := newTestClient(srv).Log(context.Background(), 42, 20, 100, "")
	if err != nil {
		t.Fatalf("Log error: %v", err)
	}
	if !hasMore {
		t.Fatal("expected has_more=true")
	}
	if len(items) != 2 || items[0].ID != 99 || items[0].Delta != 3 || items[1].Delta != -1 {
		t.Fatalf("unexpected items: %+v", items)
	}
}

func TestLog_EmptyNeverNil(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{"has_more": false}})
	}))
	defer srv.Close()

	items, _, err := newTestClient(srv).Log(context.Background(), 42, 20, 0, "")
	if err != nil {
		t.Fatalf("Log error: %v", err)
	}
	if items == nil {
		t.Fatal("items must be non-nil (empty slice) so JSON marshals to [] not null")
	}
}

// Status and body pairs are what infra answers on the s2s faces:
// middleware.OAuthClientBasicAuth for the credential, and s2sTarget / respondErr
// in apps/api/internal/platform/ledger/handler/handler.go for the rest.
func TestAdjustErrorKinds(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   upstream.Kind
	}{
		{"bad secret", 401, `{"code":15008,"message":"客户端密钥无效"}`, upstream.Internal},
		{"not an awarder", 403, `{"code":16005,"message":"该客户端无权发放萌萌点"}`, upstream.Internal},
		{"key names another award", 400, `{"code":16004,"message":"幂等键已存在但请求内容不一致"}`, upstream.Conflict},
		{"unknown user", 404, `{"code":10005,"message":"用户不存在"}`, upstream.NotFound},
		{"ledger down", 500, `{"code":10,"message":"操作失败"}`, upstream.Unavailable},
		{"gateway page", 502, `<html>bad gateway</html>`, upstream.Unavailable},
		{"wrong origin", 404, `<html>not found</html>`, upstream.Internal},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			_, err := newTestClient(srv).Adjust(context.Background(), 42,
				AdjustRequest{Delta: 1, Reason: "liked", IdempotencyKey: "moyu:resource_like:7:42"})
			if got := upstream.KindOf(err); got != tc.want {
				t.Fatalf("kind = %d, want %d (err %v)", got, tc.want, err)
			}
		})
	}
}

func TestFailureLevelIsLoudForMoyusOwnFaults(t *testing.T) {
	for kind, want := range map[upstream.Kind]slog.Level{
		upstream.Internal:    slog.LevelError,
		upstream.Conflict:    slog.LevelError,
		upstream.Unavailable: slog.LevelWarn,
		upstream.RateLimited: slog.LevelWarn,
		upstream.NotFound:    slog.LevelWarn,
	} {
		if got := failureLevel(&upstream.Error{Kind: kind}); got != want {
			t.Errorf("kind %d logs at %v, want %v", kind, got, want)
		}
	}
}
