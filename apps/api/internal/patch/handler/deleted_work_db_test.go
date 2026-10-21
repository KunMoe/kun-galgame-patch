package handler

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"

	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/internal/patch/model"
	"kun-galgame-patch-api/internal/patch/repository"
	"kun-galgame-patch-api/internal/patch/service"
	"kun-galgame-patch-api/pkg/catalogv2/catalogv2test"

	"github.com/gofiber/fiber/v3"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	deletedUser   = 1499999921
	stampedPage   = 1499999921
	unstampedPage = 1499999922
	missingWork   = 1499999923
)

// Runs only against the launcher-provided TEST_DATABASE_DSN, migrated through
// 051, with `go test -count=1 -p 1`.
func deletedWorkApp(t *testing.T) *fiber.App {
	app, _, _ := deletedWorkFixture(t)
	return app
}

func deletedWorkFixture(t *testing.T) (*fiber.App, *service.PatchService, *gorm.DB) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN is not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	clean := func() {
		db.Exec(`DELETE FROM patch WHERE id BETWEEN ? AND ?`, stampedPage, missingWork)
		db.Exec(`DELETE FROM "user" WHERE id = ?`, deletedUser)
	}
	clean()
	t.Cleanup(clean)
	for _, q := range []string{
		`INSERT INTO "user" (id, updated) VALUES (1499999921, now())`,
		`INSERT INTO patch (id, vndb_id, user_id, updated, catalog_deleted_at, catalog_deleted_reason)
			VALUES (1499999921, 'v-deleted-1', 1499999921, now(), '2026-10-23T08:00:00Z', 'out_of_scope')`,
		`INSERT INTO patch (id, vndb_id, user_id, updated) VALUES (1499999922, 'v-deleted-2', 1499999921, now())`,
	} {
		if err := db.Exec(q).Error; err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/catalog/works":
			_, _ = io.WriteString(w, `{"object":"list","items":[]}`)
		case "/v2/catalog/works/" + strconv.Itoa(unstampedPage):
			catalogv2test.Problem(w, r, "ENTITY_DELETED", "work was deleted.", map[string]any{
				"object": "work", "deleted_reason": "spam_or_test", "deleted_at": "2026-10-23T09:00:00Z",
			})
		default:
			catalogv2test.Problem(w, r, "NOT_FOUND", "No work with this id.", nil)
		}
	}))
	t.Cleanup(srv.Close)

	gal := galgameClient.NewWithKey(srv.URL, appKey)
	svc := service.New(repository.New(db), nil, db, nil, gal, nil, nil, nil, nil)
	h := New(svc, gal, nil, nil, nil)
	app := fiber.New()
	app.Get("/patch/:id", h.GetPatch)
	app.Get("/patch/resource/:resourceId/revisions", h.GetResourceRevisions)
	return app, svc, db
}

type gamePageAnswer struct {
	status int
	Code   int `json:"code"`
	Data   struct {
		DeletedReason string `json:"deleted_reason"`
		DeletedAt     string `json:"deleted_at"`
	} `json:"data"`
}

func getGamePage(t *testing.T, app *fiber.App, id int) gamePageAnswer {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/patch/"+strconv.Itoa(id), nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out gamePageAnswer
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	out.status = resp.StatusCode
	return out
}

func TestADeletedWorkSaysSoInsteadOfNotFound(t *testing.T) {
	app := deletedWorkApp(t)

	t.Run("a page the mirror stamped answers from its own row", func(t *testing.T) {
		got := getGamePage(t, app, stampedPage)
		if got.status != http.StatusGone || got.Code != 41001 ||
			got.Data.DeletedReason != "out_of_scope" || got.Data.DeletedAt != "2026-10-23T08:00:00Z" {
			t.Fatalf("answer = %+v", got)
		}
	})

	t.Run("a page the mirror has not reached asks catalog", func(t *testing.T) {
		got := getGamePage(t, app, unstampedPage)
		if got.status != http.StatusGone || got.Code != 41001 || got.Data.DeletedReason != "spam_or_test" {
			t.Fatalf("answer = %+v", got)
		}
	})

	t.Run("a work catalog never had is still a plain 404", func(t *testing.T) {
		got := getGamePage(t, app, missingWork)
		if got.status != http.StatusNotFound || got.Code != 40400 {
			t.Fatalf("answer = %+v", got)
		}
	})
}

func TestADeletedWorkTakesNoNewResourceAndShowsNoHistory(t *testing.T) {
	app, svc, db := deletedWorkFixture(t)

	err := svc.CreateResource(context.Background(), &model.PatchResource{
		GalgameID: stampedPage, Storage: "user", Content: "https://example.com/x",
	}, deletedUser)
	if !stderrors.Is(err, service.ErrWorkDeleted) {
		t.Fatalf("CreateResource on a deleted work: %v", err)
	}

	var res int
	db.Raw(`INSERT INTO patch_resource (storage, user_id, galgame_id, updated) VALUES ('user', ?, ?, now()) RETURNING id`,
		deletedUser, stampedPage).Scan(&res)
	resp, err := app.Test(httptest.NewRequest(http.MethodGet,
		"/patch/resource/"+strconv.Itoa(res)+"/revisions?page=1&limit=10", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("revisions of a deleted work's resource: HTTP %d, want 404", resp.StatusCode)
	}
}
