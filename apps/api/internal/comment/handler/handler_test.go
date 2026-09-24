package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"kun-galgame-patch-api/internal/comment/service"
	"kun-galgame-patch-api/internal/community/anchor"
	"kun-galgame-patch-api/internal/middleware"
	"kun-galgame-patch-api/pkg/communityclient"
	"kun-galgame-patch-api/pkg/response"
	"kun-galgame-patch-api/pkg/upstream"

	"github.com/gofiber/fiber/v3"
)

// Bodies as infra's community service writes them: handler/auth.go for the
// credential and the site binding, handler/errors.go mapErr for the rest.
const (
	badCredential  = `{"code":10001,"message":"未授权，请先登录"}`
	unbound        = `{"code":5,"message":"client is not bound to a site; it cannot act on the community"}`
	notAuthor      = `{"code":5,"message":"not the post author"}`
	contentBlocked = `{"code":7,"message":"content blocked by word list"}`
	badField       = `{"code":7,"message":"anchor_id is required"}`
	dailyLimit     = `{"code":10,"message":"sandbox limit: daily reply limit"}`
	tooManyLinks   = `{"code":10,"message":"sandbox limit: too many links"}`
	serverError    = `{"code":3,"message":"服务器内部错误"}`
)

type reply struct {
	status int
	body   string
	header map[string]string
}

func ok(data any) reply {
	b, _ := json.Marshal(map[string]any{"code": 0, "message": "成功", "data": data})
	return reply{status: 200, body: string(b)}
}

// The post a reply or an edit resolves first: on the game wall of patch 42.
var resolvedPost = ok(map[string]any{"posts": []any{map[string]any{
	"post":   map[string]any{"id": 900, "thread_id": 7, "author_id": 5, "content_raw": "原文"},
	"thread": map[string]any{"thread_id": 7, "anchor_kind": 1, "anchor_id": "42"},
}}})

var noPosts = ok(map[string]any{"posts": []any{}})

func testApp(t *testing.T, routes map[string]reply) *fiber.App {
	app, _ := recordingApp(t, routes)
	return app
}

func recordingApp(t *testing.T, routes map[string]reply) (*fiber.App, *[]*http.Request) {
	t.Helper()
	var seen []*http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Clone(r.Context()))
		rep, found := routes[r.Method+" "+r.URL.Path]
		if !found {
			t.Errorf("unexpected community call %s %s", r.Method, r.URL.Path)
			rep = reply{status: 404, body: `{"code":404,"message":"Cannot ` + r.Method + ` ` + r.URL.Path + `"}`}
		}
		w.Header().Set("Content-Type", "application/json")
		for k, v := range rep.header {
			w.Header().Set(k, v)
		}
		w.WriteHeader(rep.status)
		_, _ = io.WriteString(w, rep.body)
	}))
	t.Cleanup(srv.Close)

	community := communityclient.New(communityclient.Config{BaseURL: srv.URL, ClientID: "moyu", ClientSecret: "secret"})
	svc := service.New(community, nil, anchor.New(nil, nil), nil, nil, nil, nil, nil, nil)
	h := New(svc, nil, nil)

	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Locals("user", &middleware.UserInfo{ID: 3})
		return c.Next()
	})
	app.Get("/patch/:id/comment", h.GetPatchComments)
	app.Post("/patch/:id/comment", h.CreatePatchComment)
	app.Put("/patch/comment/:postId", h.UpdateComment)
	app.Put("/patch/comment/:postId/like", h.LikeComment)
	app.Delete("/patch/comment/:postId/like", h.UnlikeComment)
	return app, &seen
}

func call(t *testing.T, app *fiber.App, method, path, body string) (int, response.Response) {
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

func TestWallRead(t *testing.T) {
	cases := []struct {
		name   string
		answer reply
		status int
		code   int
	}{
		// An empty wall with no log line was the answer to all three until the
		// client learned whose fault each one is.
		{"community down reads as an empty wall", reply{status: 500, body: serverError}, 200, 0},
		{"a lost site binding is moyu's 500, not an empty wall", reply{status: 403, body: unbound}, 500, 50000},
		{"a refused credential is moyu's 500, never the reader's 401", reply{status: 401, body: badCredential}, 500, 50000},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := testApp(t, map[string]reply{"GET /comments": tc.answer})
			status, body := call(t, app, http.MethodGet, "/patch/42/comment", "")
			if status != tc.status || body.Code != tc.code {
				t.Errorf("got %d/%d %q, want %d/%d", status, body.Code, body.Message, tc.status, tc.code)
			}
		})
	}
}

func TestCreateAnswers(t *testing.T) {
	cases := []struct {
		name    string
		routes  map[string]reply
		body    string
		status  int
		code    int
		message string
	}{
		{"an unreachable parent lookup is a 503, not a missing parent",
			map[string]reply{"POST /posts/resolve": {status: 503, body: serverError}},
			`{"content":"好","reply_to_post_id":900}`, 503, 50321, ""},
		{"a parent that is gone is the reader's 400",
			map[string]reply{"POST /posts/resolve": noPosts},
			`{"content":"好","reply_to_post_id":900}`, 400, 40000, "parent comment not found"},
		{"the word list names what to change",
			map[string]reply{"POST /comments": {status: 422, body: contentBlocked}},
			`{"content":"好"}`, 422, 42200, "评论包含违禁词，请修改后再发布"},
		{"too many links is not a wait",
			map[string]reply{"POST /comments": {status: 429, body: tooManyLinks}},
			`{"content":"好"}`, 400, 40000, "新用户的评论中链接、图片或提及过多，请减少后再发布"},
		{"the daily cap is a wait",
			map[string]reply{"POST /comments": {status: 429, body: dailyLimit}},
			`{"content":"好"}`, 429, 42900, ""},
		{"a field moyu got wrong is moyu's 500",
			map[string]reply{"POST /comments": {status: 422, body: badField}},
			`{"content":"好"}`, 500, 50000, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, body := call(t, testApp(t, tc.routes), http.MethodPost, "/patch/42/comment", tc.body)
			if status != tc.status || body.Code != tc.code {
				t.Errorf("got %d/%d %q, want %d/%d", status, body.Code, body.Message, tc.status, tc.code)
			}
			if tc.message != "" && body.Message != tc.message {
				t.Errorf("message = %q, want %q", body.Message, tc.message)
			}
		})
	}
}

func TestEditingSomeoneElsesCommentIsTheReadersRefusal(t *testing.T) {
	app := testApp(t, map[string]reply{
		"POST /posts/resolve": resolvedPost,
		"PATCH /posts/900":    {status: 403, body: notAuthor},
	})
	status, body := call(t, app, http.MethodPut, "/patch/comment/900", `{"content":"改"}`)
	if status != 403 || body.Code != 40300 || body.Message != "只能编辑或删除自己的评论" {
		t.Errorf("got %d/%d %q", status, body.Code, body.Message)
	}
}

var createdPost = ok(map[string]any{
	"thread": map[string]any{"id": 7, "anchor_kind": 1, "anchor_id": "42"},
	"post":   map[string]any{"id": 901, "thread_id": 7, "post_number": 3, "author_id": 3, "content_raw": "好"},
})

// Community scopes an idempotency key to the site, so the page's submit key is
// never forwarded as it came: under another reader's id it would replay their
// post. And a replayed answer is a post whose counters already moved, which is
// why this service has no repository here: running them again would panic.
func TestARetriedSubmitIsForwardedUnderTheAuthorsKeyAndReplayed(t *testing.T) {
	replayed := createdPost
	replayed.header = map[string]string{"Idempotency-Replayed": "true"}
	app, seen := recordingApp(t, map[string]reply{"POST /comments": replayed})

	status, body := call(t, app, http.MethodPost, "/patch/42/comment", `{"content":"好","submit_key":"k-1"}`)
	if status != 200 || body.Code != 0 {
		t.Fatalf("got %d/%d %q", status, body.Code, body.Message)
	}
	got := (*seen)[0].Header.Get("Idempotency-Key")
	if got == "" || got == "k-1" || got != upstream.IdempotencyKey("3", "comment", "k-1") {
		t.Errorf("Idempotency-Key = %q", got)
	}
}

func TestASubmitWithoutAKeySendsNone(t *testing.T) {
	app, seen := recordingApp(t, map[string]reply{"POST /comments": {status: 409, body: `{"code":10,"message":"thread is not open"}`}})
	status, body := call(t, app, http.MethodPost, "/patch/42/comment", `{"content":"好"}`)
	if status != 409 || body.Message != "该评论区已关闭" {
		t.Errorf("got %d %q", status, body.Message)
	}
	if got := (*seen)[0].Header.Get("Idempotency-Key"); got != "" {
		t.Errorf("Idempotency-Key = %q, want none", got)
	}
}

func TestAKeyReusedOnAnotherBodyIsTheReadersConflict(t *testing.T) {
	app := testApp(t, map[string]reply{"POST /comments": {status: 409,
		body: `{"code":10,"message":"Idempotency-Key was reused with a different request"}`}})
	status, body := call(t, app, http.MethodPost, "/patch/42/comment", `{"content":"改过","submit_key":"k-1"}`)
	if status != 409 || body.Code != 40900 || body.Message != "这次提交的内容与重试前不一致，请重新发布" {
		t.Errorf("got %d/%d %q", status, body.Code, body.Message)
	}
}

// The like was a toggle, and a retried click undid itself. The page now says
// which state it wants, and the retry lands on the same one.
func TestLikeAndUnlikeSayWhichStateTheyWant(t *testing.T) {
	answer := func(added bool) reply {
		return ok(map[string]any{
			"added": added, "changed": false, "reaction_count": 4,
			"author_id": 3, "thread_id": 7, "anchor_kind": 1, "anchor_id": "42",
		})
	}
	app, seen := recordingApp(t, map[string]reply{
		"POST /posts/resolve":        resolvedPost,
		"PUT /posts/900/reaction":    answer(true),
		"DELETE /posts/900/reaction": answer(false),
	})

	for _, tc := range []struct {
		method string
		liked  bool
	}{{http.MethodPut, true}, {http.MethodDelete, false}} {
		status, body := call(t, app, tc.method, "/patch/comment/900/like", "")
		data, _ := body.Data.(map[string]any)
		if status != 200 || data["liked"] != tc.liked || data["like_count"] != float64(4) {
			t.Errorf("%s: got %d %v", tc.method, status, body.Data)
		}
	}
	for _, r := range *seen {
		if r.Method == http.MethodPost && r.URL.Path == "/posts/900/reaction" {
			t.Error("the toggle face was called")
		}
	}
}
