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

func fakeArtifacts(t *testing.T, status int, meta, download string) *artifactclient.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if strings.HasSuffix(r.URL.Path, "/download") {
			_, _ = w.Write([]byte(download))
			return
		}
		_, _ = w.Write([]byte(meta))
	}))
	t.Cleanup(srv.Close)
	return artifactclient.New(artifactclient.Config{BaseURL: srv.URL, ClientID: "moyu", ClientSecret: "secret"})
}

func TestResourceFileShape(t *testing.T) {
	const uuid = "d874a243-0000-4000-8000-000000007007"
	art := fakeArtifacts(t, 200,
		`{"code":0,"message":"成功","data":{"uuid":"`+uuid+`","name":"传说奶茶A 汉化补丁.rar","file_size":32135794,"checksum":"","public":true,"site_key":"moyu","status":1}}`,
		`{"code":0,"message":"成功","data":{"url":"https://dl.imoe.uk/moyu/`+uuid+`.rar"}}`)
	svc := New(nil, nil, nil, art, "https://www.moyu.moe")
	row := patchModel.PatchResource{
		ID:           7007,
		ArtifactUUID: uuid,
		Password:     "moyu.moe",
		UpdateTime:   time.Date(2026, 10, 14, 18, 30, 0, 0, time.FixedZone("CST", 8*3600)),
	}

	got, prob := svc.resourceFile(context.Background(), &row, strings.Repeat("ab", 32))
	if prob != nil {
		t.Fatal(prob)
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"object":"patch_resource_file","resource_id":"7007","artifact_uuid":"` + uuid + `",` +
		`"download_url":"https://dl.imoe.uk/moyu/` + uuid + `.rar","filename":"传说奶茶A 汉化补丁.rar",` +
		`"bytes":32135794,"blake3":"` + strings.Repeat("ab", 32) + `","password":"moyu.moe",` +
		`"updated_at":"2026-10-14T10:30:00Z"}`
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
			svc := New(nil, nil, nil, fakeArtifacts(t, tc.status, tc.body, tc.body), "https://www.moyu.moe")
			row := patchModel.PatchResource{ID: 7007, ArtifactUUID: "u1"}
			_, prob := svc.resourceFile(context.Background(), &row, "")
			if prob == nil || prob.Code != tc.code {
				t.Fatalf("problem = %+v, want code %s", prob, tc.code)
			}
		})
	}
}
