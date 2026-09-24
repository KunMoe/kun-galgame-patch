package upload

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"kun-galgame-patch-api/internal/constants"
	"kun-galgame-patch-api/pkg/artifactclient"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

const testUUID = "0b9d2f6e-3c1a-4f5e-9a7b-2d4c6e8f0a1b"

func writeEnvelope(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "message": "成功", "data": data})
}

func newTestService(t *testing.T) (*Service, *redis.Client) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/artifacts", func(w http.ResponseWriter, _ *http.Request) {
		writeEnvelope(w, map[string]any{
			"uuid": testUUID, "multipart": false,
			"upload_url": "https://store.example/put", "expires_at": "2026-09-24T12:00:00Z",
		})
	})
	mux.HandleFunc("GET /api/v1/artifacts/{uuid}/resume", func(w http.ResponseWriter, r *http.Request) {
		writeEnvelope(w, map[string]any{
			"uuid": r.PathValue("uuid"), "multipart": false,
			"upload_url": "https://store.example/put", "expires_at": "2026-09-24T12:00:00Z",
		})
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected artifact call %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	rdb := redis.NewClient(&redis.Options{Addr: miniredis.RunT(t).Addr()})
	art := artifactclient.New(artifactclient.Config{BaseURL: srv.URL, ClientID: "moyu", ClientSecret: "secret"})
	return New(art, nil, rdb), rdb
}

func TestResumeIsOwnerOnly(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	init, err := svc.Init(ctx, 7, constants.AdminUploadTier, InitRequest{GalgameID: 1, FileName: "patch.zip", FileSize: 10})
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := svc.Resume(ctx, 8, ResumeRequest{ArtifactUUID: init.ArtifactUUID}); !errors.Is(err, errNotUploadOwner) {
		t.Fatalf("resume by another user: got %v, want errNotUploadOwner", err)
	}
	if _, err := svc.Resume(ctx, 7, ResumeRequest{ArtifactUUID: init.ArtifactUUID}); err != nil {
		t.Fatalf("resume by uploader: %v", err)
	}
}

func TestAbortRefusesAnotherUsersUpload(t *testing.T) {
	svc, rdb := newTestService(t)
	ctx := context.Background()
	rdb.Set(ctx, uploadOwnerKey(testUUID), 7, uploadOwnerTTL)

	if err := svc.Abort(ctx, 8, AbortRequest{ArtifactUUID: testUUID}); !errors.Is(err, errNotUploadOwner) {
		t.Fatalf("abort by another user: got %v, want errNotUploadOwner", err)
	}
}

func TestUploadWithNoOwnerRecordIsRefused(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	if err := svc.Abort(ctx, 7, AbortRequest{ArtifactUUID: testUUID}); !errors.Is(err, errNotUploadOwner) {
		t.Fatalf("abort: got %v, want errNotUploadOwner", err)
	}
	_, err := svc.Complete(ctx, 7, constants.AdminUploadTier, CompleteRequest{ArtifactUUID: testUUID, DeclaredSize: 10})
	if !errors.Is(err, errNotUploadOwner) {
		t.Fatalf("complete: got %v, want errNotUploadOwner", err)
	}
}
