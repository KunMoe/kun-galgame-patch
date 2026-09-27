package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"kun-galgame-patch-api/internal/community/following"
	"kun-galgame-patch-api/internal/middleware"
	"kun-galgame-patch-api/pkg/communityclient"
	"kun-galgame-patch-api/pkg/response"

	"github.com/gofiber/fiber/v3"
)

type communityCall struct {
	method, path, query, body string
}

// Bodies as infra's community service writes them (handler/errors.go mapErr).
func followingApp(t *testing.T, status int, body string) (*fiber.App, *[]communityCall) {
	t.Helper()
	var calls []communityCall
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		calls = append(calls, communityCall{r.Method, r.URL.Path, r.URL.RawQuery, string(raw)})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)

	community := communityclient.New(communityclient.Config{BaseURL: srv.URL, ClientID: "moyu", ClientSecret: "secret"})
	h := NewFollowingHandler(following.New(community, nil))

	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Locals("user", &middleware.UserInfo{ID: 3})
		return c.Next()
	})
	app.Get("/community/following/group/:id", h.GroupItems)
	app.Get("/community/activity-settings", h.ActivitySetting)
	app.Put("/community/activity-settings", h.SetActivitySetting)
	return app, &calls
}

func send(t *testing.T, app *fiber.App, method, path, body string) (int, response.Response) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	var out response.Response
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return res.StatusCode, out
}

// The /following page drops a group whose expand answers 40400: its author
// hid their activities after the page loaded.
func TestAHiddenAuthorsGroupIs40400ForTheReader(t *testing.T) {
	app, calls := followingApp(t, http.StatusNotFound, `{"code":4,"message":"资源不存在"}`)

	status, res := send(t, app, http.MethodGet, "/community/following/group/5?limit=20", "")
	if status != http.StatusNotFound || res.Code != 40400 {
		t.Fatalf("status = %d, code = %d", status, res.Code)
	}
	got := (*calls)[0]
	if got.path != "/activity-groups/5/items" || !strings.Contains(got.query, "viewer_id=3") {
		t.Errorf("community call = %+v", got)
	}
}

func TestSetActivitySettingSendsAnExplicitFalse(t *testing.T) {
	app, calls := followingApp(t, http.StatusOK,
		`{"code":0,"message":"成功","data":{"user_id":3,"hidden":false,"updated_at":"2026-09-27T07:30:00Z"}}`)

	status, res := send(t, app, http.MethodPut, "/community/activity-settings", `{"hidden":false}`)
	if status != http.StatusOK || res.Code != 0 {
		t.Fatalf("status = %d, res = %+v", status, res)
	}
	got := (*calls)[0]
	if got.method != http.MethodPut || got.path != "/users/3/activity-settings" || got.body != `{"hidden":false}` {
		t.Errorf("community call = %+v", got)
	}
	data, _ := json.Marshal(res.Data)
	if string(data) != `{"hidden":false,"updated_at":"2026-09-27T07:30:00Z","user_id":3}` {
		t.Errorf("data = %s", data)
	}
}

func TestSetActivitySettingNeedsHidden(t *testing.T) {
	app, calls := followingApp(t, http.StatusOK, `{"code":0,"message":"成功","data":{}}`)

	status, _ := send(t, app, http.MethodPut, "/community/activity-settings", `{}`)
	if status != http.StatusBadRequest || len(*calls) != 0 {
		t.Errorf("status = %d, community calls = %+v", status, *calls)
	}
}

func TestAnUnsetActivitySettingReadsShownWithNoTimestamp(t *testing.T) {
	app, calls := followingApp(t, http.StatusOK,
		`{"code":0,"message":"成功","data":{"user_id":3,"hidden":false,"updated_at":null}}`)

	status, res := send(t, app, http.MethodGet, "/community/activity-settings", "")
	if status != http.StatusOK || (*calls)[0].path != "/users/3/activity-settings" {
		t.Fatalf("status = %d, calls = %+v", status, *calls)
	}
	data, _ := json.Marshal(res.Data)
	if string(data) != `{"hidden":false,"updated_at":null,"user_id":3}` {
		t.Errorf("data = %s", data)
	}
}
