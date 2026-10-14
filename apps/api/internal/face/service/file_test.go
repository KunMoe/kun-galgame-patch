package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	patchModel "kun-galgame-patch-api/internal/patch/model"
	"kun-galgame-patch-api/pkg/artifactclient"
	"kun-galgame-patch-api/pkg/problem"
)

// The download face answers https://dl.imoe.uk/moyu/<uuid> for any uuid but
// gone, which answers 404.
func fakeArtifacts(t *testing.T, status int, meta, gone string) *artifactclient.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		uuid, isDownload := strings.CutSuffix(strings.TrimPrefix(r.URL.Path, "/api/v1/artifacts/"), "/download")
		switch {
		case status != 200:
			w.WriteHeader(status)
			_, _ = w.Write([]byte(meta))
		case isDownload && uuid == gone:
			w.WriteHeader(404)
			_, _ = w.Write([]byte(`{"code":50001,"message":"资源不存在"}`))
		case isDownload:
			_, _ = w.Write([]byte(`{"code":0,"message":"成功","data":{"url":"https://dl.imoe.uk/moyu/` + uuid + `"}}`))
		default:
			_, _ = w.Write([]byte(meta))
		}
	}))
	t.Cleanup(srv.Close)
	return artifactclient.New(artifactclient.Config{BaseURL: srv.URL, ClientID: "moyu", ClientSecret: "secret"})
}

func TestResourceFileShape(t *testing.T) {
	const uuid = "d874a243-0000-4000-8000-000000007007"
	art := fakeArtifacts(t, 200,
		`{"code":0,"message":"成功","data":{"uuid":"`+uuid+`","name":"传说奶茶A 汉化补丁.rar","file_size":32135794,"checksum":"","public":true,"site_key":"moyu","status":1}}`,
		"")
	svc := New(nil, nil, nil, art, "https://www.moyu.moe")
	row := patchModel.PatchResource{
		ID:           7007,
		ArtifactUUID: uuid,
		Blake3:       strings.Repeat("ab", 32),
		Password:     "moyu.moe",
		UpdateTime:   time.Date(2026, 10, 14, 18, 30, 0, 0, time.FixedZone("CST", 8*3600)),
	}

	got, prob := svc.resourceFile(context.Background(), &row, nil)
	if prob != nil {
		t.Fatal(prob)
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"object":"patch_resource_file","resource_id":"7007","artifact_uuid":"` + uuid + `",` +
		`"download_url":"https://dl.imoe.uk/moyu/` + uuid + `","filename":"传说奶茶A 汉化补丁.rar",` +
		`"bytes":32135794,"blake3":"` + strings.Repeat("ab", 32) + `","password":"moyu.moe",` +
		`"updated_at":"2026-10-14T10:30:00Z","repack":null}`
	if string(encoded) != want {
		t.Errorf("got  %s\nwant %s", encoded, want)
	}
}

func TestResourceFileArtifactFailures(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		code   string
	}{
		{"file deleted from storage", 404, `{"code":50001,"message":"资源不存在"}`, problem.CodeNotFound},
		{"artifact service failing", 500, `{"code":50013,"message":"制品存储失败"}`, problem.CodeServiceUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := New(nil, nil, nil, fakeArtifacts(t, tc.status, tc.body, ""), "https://www.moyu.moe")
			row := patchModel.PatchResource{ID: 7007, ArtifactUUID: "u1"}
			_, prob := svc.resourceFile(context.Background(), &row, nil)
			if prob == nil || prob.Code != tc.code {
				t.Fatalf("problem = %+v, want code %s", prob, tc.code)
			}
		})
	}
}

func TestResourceFileCarriesItsRepack(t *testing.T) {
	const source, repacked = "aaaaaaaa-0000-4000-8000-000000000001", "bbbbbbbb-0000-4000-8000-000000000002"
	meta := `{"code":0,"message":"成功","data":{"uuid":"` + source + `","name":"p.rar","file_size":10,"public":true,"status":1}}`
	row := patchModel.PatchResource{ID: 7007, ArtifactUUID: source}
	repack := patchModel.PatchResourceRepack{
		ResourceID: 7007, ArtifactUUID: repacked, SourceArtifactUUID: source,
		Blake3: strings.Repeat("cd", 32), Bytes: 42, FileCount: 3, SourceBlake3: strings.Repeat("ef", 32),
		CreatedAt: time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC),
	}

	got, prob := New(nil, nil, nil, fakeArtifacts(t, 200, meta, ""), "").resourceFile(context.Background(), &row, &repack)
	if prob != nil {
		t.Fatal(prob)
	}
	encoded, _ := json.Marshal(got.Repack)
	want := `{"object":"patch_resource_repack","artifact_uuid":"` + repacked + `",` +
		`"download_url":"https://dl.imoe.uk/moyu/` + repacked + `","blake3":"` + strings.Repeat("cd", 32) + `",` +
		`"bytes":42,"file_count":3,"source_artifact_uuid":"` + source + `",` +
		`"source_blake3":"` + strings.Repeat("ef", 32) + `","created_at":"2026-10-14T12:00:00Z"}`
	if string(encoded) != want {
		t.Errorf("got  %s\nwant %s", encoded, want)
	}

	// An operator can delete an artifact from the console without moyu
	// hearing of it. The original is still served; only the repack reads null.
	got, prob = New(nil, nil, nil, fakeArtifacts(t, 200, meta, repacked), "").resourceFile(context.Background(), &row, &repack)
	if prob != nil {
		t.Fatal(prob)
	}
	if got.Repack != nil || got.DownloadURL == "" {
		t.Errorf("repack = %+v, download_url = %q; want a null repack beside the original", got.Repack, got.DownloadURL)
	}
}
