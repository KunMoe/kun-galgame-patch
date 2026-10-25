package archive

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"kun-galgame-patch-api/pkg/artifactclient"

	"github.com/gofiber/fiber/v3"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	testUser    = 1499999960
	livePage    = 1499999961
	deletedPage = 1499999962
	adultPage   = 1499999963

	hostedUUID   = "00000000-0000-4000-8000-000000970001"
	disabledUUID = "00000000-0000-4000-8000-000000970002"
	deletedUUID  = "00000000-0000-4000-8000-000000970003"
	adultUUID    = "00000000-0000-4000-8000-000000970004"
)

func testZip(t *testing.T) []byte {
	t.Helper()
	save, err := os.ReadFile("testdata/file1.rpgsave")
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	for name, body := range map[string][]byte{
		"game/readme.txt":              []byte("説明"),
		"game/www/save/config.rpgsave": save,
		"game/Game.exe":                {0x4d, 0x5a},
	} {
		w, err := z.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(body); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// files serves the zip with Range support, and art answers the artifact
// service's download call with a URL into it.
func fakeStorage(t *testing.T, body []byte) *artifactclient.Client {
	t.Helper()
	files := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "f.zip", time.Time{}, bytes.NewReader(body))
	}))
	t.Cleanup(files.Close)
	art := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uuid := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/v1/artifacts/"), "/download")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"code":0,"message":"ok","data":{"url":%q}}`, files.URL+"/"+uuid+".zip")
	}))
	t.Cleanup(art.Close)
	return artifactclient.New(artifactclient.Config{BaseURL: art.URL, ClientID: "id", ClientSecret: "secret"})
}

func archiveDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN is not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	var migrated bool
	if err := db.Raw(`SELECT to_regclass('artifact_listing') IS NOT NULL`).Scan(&migrated).Error; err != nil || !migrated {
		t.Fatalf("TEST_DATABASE_DSN is not migrated through 056: %v", err)
	}
	pages := []int{livePage, deletedPage, adultPage}
	clean := func() {
		db.Exec(`DELETE FROM patch WHERE id IN ?`, pages)
		db.Exec(`DELETE FROM "user" WHERE id = ?`, testUser)
		db.Exec(`DELETE FROM artifact_listing WHERE artifact_uuid IN ?`, []string{hostedUUID, disabledUUID, deletedUUID, adultUUID})
	}
	clean()
	t.Cleanup(clean)
	for _, q := range []string{
		`INSERT INTO "user" (id, updated) VALUES (1499999960, now())`,
		`INSERT INTO patch (id, vndb_id, user_id, published, updated) VALUES (1499999961, 'v-archive-live', 1499999960, true, now())`,
		`INSERT INTO patch (id, vndb_id, user_id, published, catalog_deleted_at, updated) VALUES (1499999962, 'v-archive-deleted', 1499999960, true, now(), now())`,
		`INSERT INTO patch (id, vndb_id, user_id, published, updated) VALUES (1499999963, 'v-archive-adult', 1499999960, true, now())`,
		`UPDATE patch SET content_limit = 'nsfw' WHERE id = 1499999963`,
		`INSERT INTO patch_resource (id, storage, artifact_uuid, user_id, galgame_id, status, update_time, created, updated) VALUES
			(1499999961, 's3', '` + hostedUUID + `', 1499999960, 1499999961, 0, now(), now(), now()),
			(1499999962, 's3', '` + disabledUUID + `', 1499999960, 1499999961, 1, now(), now(), now()),
			(1499999963, 'user', '', 1499999960, 1499999961, 0, now(), now(), now()),
			(1499999964, 's3', '` + deletedUUID + `', 1499999960, 1499999962, 0, now(), now(), now()),
			(1499999965, 's3', '` + adultUUID + `', 1499999960, 1499999963, 0, now(), now(), now())`,
	} {
		if err := db.Exec(q).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func get(t *testing.T, app *fiber.App, path string) (int, http.Header, []byte) {
	t.Helper()
	res, err := app.Test(httptest.NewRequest(http.MethodGet, path, nil), fiber.TestConfig{Timeout: 10 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	return res.StatusCode, res.Header, body
}

func TestSweptListingServesTreeAndPreviews(t *testing.T) {
	db := archiveDB(t)
	art := fakeStorage(t, testZip(t))

	sum, err := NewSweeper(db, art).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var listed []string
	if err := db.Raw(`SELECT artifact_uuid FROM artifact_listing WHERE artifact_uuid IN ? ORDER BY 1`,
		[]string{hostedUUID, disabledUUID, deletedUUID, adultUUID}).Scan(&listed).Error; err != nil {
		t.Fatal(err)
	}
	if strings.Join(listed, ",") != hostedUUID+","+adultUUID {
		t.Errorf("the sweeper reads only public hosted files on live works, read %v (summary %+v)", listed, sum)
	}

	h := NewHandler(db, art)
	app := fiber.New()
	app.Get("/resource/:id/files", h.Files)
	app.Get("/resource/:id/files/content", h.FileContent)

	status, _, body := get(t, app, "/resource/1499999961/files")
	var out struct {
		Data struct {
			Listing *Listing `json:"listing"`
			Preview []Kind   `json:"preview"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &out); err != nil || status != http.StatusOK {
		t.Fatalf("files: %d %s", status, body)
	}
	if l := out.Data.Listing; l == nil || l.State != "ready" || l.FileCount != 3 {
		t.Fatalf("listing: %s", body)
	}
	if len(out.Data.Preview) == 0 {
		t.Error("the response carries the preview allowlist")
	}

	status, _, body = get(t, app, "/resource/1499999962/files")
	if status != http.StatusOK || !strings.Contains(string(body), `"listing":null`) {
		t.Errorf("a disabled resource shows no listing: %d %s", status, body)
	}
	if status, _, _ = get(t, app, "/resource/1499999965/files"); status != http.StatusNotFound {
		t.Errorf("an adult work's listing is hidden from an SFW reader, got %d", status)
	}
	if status, _, _ = get(t, app, "/resource/1499999965/files?content_limit=nsfw"); status != http.StatusOK {
		t.Errorf("an NSFW reader sees an adult work's listing, got %d", status)
	}

	status, hdr, body := get(t, app, "/resource/1499999961/files/content?path=game/readme.txt")
	if status != http.StatusOK || string(body) != "説明" || hdr.Get("Content-Security-Policy") == "" {
		t.Errorf("text preview: %d %q %v", status, body, hdr)
	}
	status, _, body = get(t, app, "/resource/1499999961/files/content?path=game/www/save/config.rpgsave")
	if status != http.StatusOK || !strings.Contains(string(body), `"alwaysDash": true`) {
		t.Errorf("an RPG Maker save previews as decoded JSON: %d %.200s", status, body)
	}
	if status, _, _ = get(t, app, "/resource/1499999961/files/content?path=game/Game.exe"); status != http.StatusBadRequest {
		t.Errorf("an exe is outside the allowlist, got %d", status)
	}
	if status, _, _ = get(t, app, "/resource/1499999961/files/content?path=game/other.txt"); status != http.StatusNotFound {
		t.Errorf("a path the listing does not hold is not read, got %d", status)
	}
	if status, _, _ = get(t, app, "/resource/1499999962/files/content?path=game/readme.txt"); status != http.StatusNotFound {
		t.Errorf("a disabled resource's file is not read, got %d", status)
	}

	if err := db.Exec(`UPDATE patch_resource SET artifact_uuid = '' WHERE id = 1499999962`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`UPDATE patch_resource SET artifact_uuid = '` + disabledUUID + `' WHERE id = 1499999961`).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := NewSweeper(db, art).Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	var orphan int64
	db.Raw(`SELECT count(*) FROM artifact_listing WHERE artifact_uuid = ?`, hostedUUID).Scan(&orphan)
	if orphan != 0 {
		t.Error("a replaced file's listing is reclaimed")
	}
}
