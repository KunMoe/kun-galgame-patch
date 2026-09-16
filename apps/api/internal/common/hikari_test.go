package common

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"kun-galgame-patch-api/internal/middleware"
	patchModel "kun-galgame-patch-api/internal/patch/model"

	"github.com/gofiber/fiber/v3"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestGetHikariIntegration(t *testing.T) {
	dsn := os.Getenv("HIKARI_TEST_DSN")
	if dsn == "" {
		t.Skip("HIKARI_TEST_DSN not set; skipping Hikari integration test")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db.Exec("DROP TABLE IF EXISTS patch_resource, patch CASCADE")
	if err := db.AutoMigrate(&patchModel.Patch{}, &patchModel.PatchResource{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	released := time.Date(2016, 11, 25, 0, 0, 0, 0, time.UTC)
	patch := patchModel.Patch{
		VndbID:      "v19658",
		Status:      0,
		Download:    100,
		View:        5000,
		Type:        patchModel.JSONArray{"chinese"},
		Language:    patchModel.JSONArray{"zh-Hans"},
		Platform:    patchModel.JSONArray{"windows"},
		ReleaseDate: &released,
		UserID:      3,
	}
	if err := db.Create(&patch).Error; err != nil {
		t.Fatalf("seed patch: %v", err)
	}

	visible := patchModel.PatchResource{
		Storage:   "s3",
		Name:      "汉化补丁 v1.0",
		ModelName: "PC",
		Size:      "120 MB",
		Note:      "解压后覆盖安装目录",
		Blake3:    "abc123hashvalue",
		Code:      "SECRET-EXTRACT-CODE",
		Password:  "SECRET-ARCHIVE-PW",
		S3Key:     "patches/v19658/secret.zip",
		Content:   "https://secret-download-link.example/v19658.zip",
		Type:      patchModel.JSONArray{"patch"},
		Language:  patchModel.JSONArray{"zh-Hans"},
		Platform:  patchModel.JSONArray{"windows"},
		Download:  42,
		Status:    0,
		UserID:    7,
		GalgameID: patch.ID,
	}
	disabled := patchModel.PatchResource{
		Storage: "s3", Name: "下架资源", Status: 1, UserID: 7, GalgameID: patch.ID,
	}
	if err := db.Create(&visible).Error; err != nil {
		t.Fatalf("seed visible resource: %v", err)
	}
	if err := db.Create(&disabled).Error; err != nil {
		t.Fatalf("seed disabled resource: %v", err)
	}

	h := NewHandler(db, nil, nil, nil, nil, nil)
	app := fiber.New()
	api := app.Group("/api/v1")
	api.Use("/hikari", middleware.HikariCORS())
	api.Get("/hikari", h.GetHikari)

	do := func(t *testing.T, target, origin string) (int, string, http.Header) {
		req, _ := http.NewRequest(http.MethodGet, "http://localhost"+target, nil)
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(b), resp.Header
	}

	t.Run("success returns legacy envelope, no secrets, NSFW-agnostic", func(t *testing.T) {
		status, body, hdr := do(t, "/api/v1/hikari?vndb_id=v19658", "https://touchgal.ink")
		t.Logf("status=%d\nbody=%s", status, body)
		if status != http.StatusOK {
			t.Fatalf("want 200, got %d", status)
		}
		if got := hdr.Get("Access-Control-Allow-Origin"); got != "https://touchgal.ink" {
			t.Errorf("ACAO: want https://touchgal.ink, got %q", got)
		}

		var env struct {
			Success bool   `json:"success"`
			Message string `json:"message"`
			Data    struct {
				ID       int              `json:"id"`
				VndbID   string           `json:"vndb_id"`
				Released string           `json:"released"`
				Status   int              `json:"status"`
				UserID   int              `json:"user_id"`
				User     map[string]any   `json:"user"`
				Resource []map[string]any `json:"resource"`
			} `json:"data"`
		}
		if err := json.Unmarshal([]byte(body), &env); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if !env.Success || env.Message != "Patch found successfully" {
			t.Errorf("envelope: success=%v message=%q", env.Success, env.Message)
		}
		if env.Data.VndbID != "v19658" {
			t.Errorf("vndb_id: want v19658, got %q", env.Data.VndbID)
		}
		if env.Data.Released != "2016-11-25" {
			t.Errorf("released: want 2016-11-25, got %q", env.Data.Released)
		}
		if len(env.Data.Resource) != 1 {
			t.Fatalf("want 1 resource (disabled filtered out), got %d", len(env.Data.Resource))
		}
		r := env.Data.Resource[0]
		if r["hash"] != "abc123hashvalue" || r["patch_id"] == nil || r["update_time"] == nil {
			t.Errorf("legacy field names missing/renamed: %v", r)
		}
		for _, leaked := range []string{"code", "password", "s3_key", "content"} {
			if _, ok := r[leaked]; ok {
				t.Errorf("LEAK: resource exposes %q", leaked)
			}
		}
		for _, leaked := range []string{"SECRET-EXTRACT-CODE", "SECRET-ARCHIVE-PW", "patches/v19658/secret.zip", "secret-download-link"} {
			if strings.Contains(body, leaked) {
				t.Errorf("LEAK: body contains secret %q", leaked)
			}
		}
		if got := r["user_id"]; got != float64(7) {
			t.Errorf("resource user_id: want 7, got %v", got)
		}
		if ru, ok := r["user"].(map[string]any); !ok || ru["id"] != float64(7) {
			t.Errorf("resource.user missing/wrong: %v", r["user"])
		}
		if env.Data.UserID != 3 || env.Data.User == nil || env.Data.User["id"] != float64(3) {
			t.Errorf("patch user_id/user wrong: user_id=%d user=%v", env.Data.UserID, env.Data.User)
		}
	})

	t.Run("missing vndb_id -> 400 legacy message", func(t *testing.T) {
		status, body, _ := do(t, "/api/v1/hikari", "")
		t.Logf("status=%d body=%s", status, body)
		if status != http.StatusBadRequest || !strings.Contains(body, "Missing required parameter: vndb_id") {
			t.Errorf("want 400 + legacy message, got %d %s", status, body)
		}
		if !strings.Contains(body, `"success":false`) || !strings.Contains(body, `"data":null`) {
			t.Errorf("want legacy fail envelope, got %s", body)
		}
	})

	t.Run("unknown vndb_id -> 404 legacy message", func(t *testing.T) {
		status, body, _ := do(t, "/api/v1/hikari?vndb_id=v00000", "")
		t.Logf("status=%d body=%s", status, body)
		if status != http.StatusNotFound || !strings.Contains(body, "No patch found for VNDB ID: v00000") {
			t.Errorf("want 404 + legacy message, got %d %s", status, body)
		}
	})
}
