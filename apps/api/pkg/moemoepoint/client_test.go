package moemoepoint

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
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
	// source_app must NOT be sent (server derives it).
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
