package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"kun-galgame-patch-api/pkg/trustclient"
)

type ensureCall struct {
	path, auth string
	kinds      []map[string]any
}

// The answer is what infra's ensureSubjectKinds handler writes
// (trust/handler/s2s.go, dto.EnsureSubjectKindsResponse).
func ensureServer(t *testing.T, status int, answer string) (*TrustService, *ensureCall) {
	t.Helper()
	got := &ensureCall{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.path, got.auth = r.URL.Path, r.Header.Get("Authorization")
		raw, _ := io.ReadAll(r.Body)
		var body struct {
			Kinds []map[string]any `json:"kinds"`
		}
		_ = json.Unmarshal(raw, &body)
		got.kinds = body.Kinds
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(answer))
	}))
	t.Cleanup(srv.Close)
	cli := trustclient.New(trustclient.Config{BaseURL: srv.URL, ClientID: "moyu", ClientSecret: "secret"})
	return NewTrustService(cli, "moyu"), got
}

const ensured = `{"code":0,"message":"成功","data":{"results":[{"key":"patch_resource","result":"created"},{"key":"user","result":"unchanged"}]}}`

func TestRegisterSubjectKindsDeclaresTheCallback(t *testing.T) {
	svc, got := ensureServer(t, 200, ensured)
	svc.RegisterSubjectKinds(context.Background(), "https://www.moyu.moe/api/v1/trust/callback", "cb-secret")

	if got.path != "/api/v1/trust/subject-kinds/ensure" || got.auth == "" {
		t.Fatalf("called %q with auth %q", got.path, got.auth)
	}
	if len(got.kinds) != 2 {
		t.Fatalf("declared %d kinds: %+v", len(got.kinds), got.kinds)
	}
	resource, user := got.kinds[0], got.kinds[1]
	if resource["key"] != "patch_resource" ||
		resource["callback_url"] != "https://www.moyu.moe/api/v1/trust/callback" ||
		resource["callback_secret"] != "cb-secret" || resource["notify_on_dismiss"] != true {
		t.Errorf("patch_resource = %+v", resource)
	}
	if user["key"] != "user" || len(user) != 1 {
		t.Errorf("user must carry no callback: %+v", user)
	}
}

func TestRegisterSubjectKindsWithoutASecretSetsNoCallback(t *testing.T) {
	svc, got := ensureServer(t, 200, ensured)
	svc.RegisterSubjectKinds(context.Background(), "https://www.moyu.moe/api/v1/trust/callback", "")

	if _, ok := got.kinds[0]["callback_url"]; ok {
		t.Errorf("an unsigned callback would be refused; got %+v", got.kinds[0])
	}
	if got.kinds[0]["notify_on_dismiss"] != true {
		t.Errorf("patch_resource = %+v", got.kinds[0])
	}
}

func TestRegisterSubjectKindsFailureDoesNotStopBoot(t *testing.T) {
	svc, _ := ensureServer(t, 403, `{"code":5,"message":"client is not bound to a site; it cannot submit reports"}`)
	svc.RegisterSubjectKinds(context.Background(), "https://www.moyu.moe/api/v1/trust/callback", "cb-secret")
}
