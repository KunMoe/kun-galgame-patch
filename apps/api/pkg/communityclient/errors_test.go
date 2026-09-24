package communityclient_test

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"net/http"
	"testing"

	"kun-galgame-patch-api/pkg/communityclient"
	"kun-galgame-patch-api/pkg/upstream"
)

// Each body below is what infra's community service writes, byte for byte:
//   - S2SAuth (handler/auth.go) answers a bad Basic credential through
//     pkg/response.Unauthorized, {code: 10001, message: GetMessage(10001)}.
//   - siteBinding (handler/auth.go) and mapErr (handler/errors.go) answer the
//     house envelope {code, message} with the generic codes 4/5/7/10/3.
//   - A path no route matches falls to app.errorHandler, {code: 404, message:
//     "Cannot GET …"}.
type answer struct {
	status int
	body   string
}

var (
	badCredential   = answer{401, `{"code":10001,"message":"未授权，请先登录"}`}
	unbound         = answer{403, `{"code":5,"message":"client is not bound to a site; it cannot act on the community"}`}
	notAuthor       = answer{403, `{"code":5,"message":"not the post author"}`}
	trustLevel      = answer{403, `{"code":5,"message":"replying on this board takes trust level 2"}`}
	postNotFound    = answer{404, `{"code":4,"message":"资源不存在"}`}
	routeMiss       = answer{404, `{"code":404,"message":"Cannot GET /api/v1/community/comments"}`}
	threadClosed    = answer{409, `{"code":10,"message":"thread is not open"}`}
	contentBlocked  = answer{422, `{"code":7,"message":"content blocked by word list"}`}
	badField        = answer{422, `{"code":7,"message":"anchor_kind must be 1..4 — a board hosts topics only"}`}
	dailyLimit      = answer{429, `{"code":10,"message":"sandbox limit: daily reply limit"}`}
	tooManyLinks    = answer{429, `{"code":10,"message":"sandbox limit: too many links"}`}
	badQuery        = answer{400, `{"code":9,"message":"search query must be 2-100 characters"}`}
	serverError     = answer{500, `{"code":3,"message":"服务器内部错误"}`}
	proxyBadGateway = answer{502, "<html><body><h1>502 Bad Gateway</h1></body></html>"}
	htmlUnderOK     = answer{200, "<!DOCTYPE html><html></html>"}
)

func fake(t *testing.T, a answer) *communityclient.Client {
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Request-ID", "req-7")
		w.WriteHeader(a.status)
		_, _ = w.Write([]byte(a.body))
	})
	return c
}

func TestFailuresAreClassifiedByWhoseFaultTheyAre(t *testing.T) {
	cases := []struct {
		name    string
		answer  answer
		kind    upstream.Kind
		refusal communityclient.Refusal
	}{
		{"moyu's Basic credential is refused", badCredential, upstream.Internal, communityclient.RefusalOther},
		{"moyu's client has no community site", unbound, upstream.Internal, communityclient.RefusalOther},
		{"the reader edits a post that is not theirs", notAuthor, upstream.Rejected, communityclient.RefusalNotAuthor},
		{"the reader's trust level is too low", trustLevel, upstream.Rejected, communityclient.RefusalOther},
		{"the post is gone", postNotFound, upstream.NotFound, communityclient.RefusalOther},
		{"moyu calls a face community does not have", routeMiss, upstream.Internal, communityclient.RefusalOther},
		{"the wall is closed", threadClosed, upstream.Conflict, communityclient.RefusalOther},
		{"the word list blocks the reader's text", contentBlocked, upstream.Rejected, communityclient.RefusalContentBlocked},
		{"moyu sent an invalid field", badField, upstream.Internal, communityclient.RefusalOther},
		{"the newcomer hit the daily cap", dailyLimit, upstream.RateLimited, communityclient.RefusalOther},
		{"the newcomer posted too many links", tooManyLinks, upstream.Rejected, communityclient.RefusalSandbox},
		{"moyu sent a malformed query", badQuery, upstream.Internal, communityclient.RefusalOther},
		{"community failed", serverError, upstream.Unavailable, communityclient.RefusalOther},
		{"a proxy in front of community failed", proxyBadGateway, upstream.Unavailable, communityclient.RefusalOther},
		{"the base URL answers a web page", htmlUnderOK, upstream.Internal, communityclient.RefusalOther},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := fake(t, tc.answer).EditPost(context.Background(), 900, communityclient.EditPostRequest{AuthorID: 3, Body: "x"})
			e, ok := upstream.As(err)
			if !ok {
				t.Fatalf("err = %v, want an *upstream.Error", err)
			}
			if e.Kind != tc.kind {
				t.Errorf("kind = %d, want %d (%v)", e.Kind, tc.kind, err)
			}
			if got := communityclient.RefusalOf(err); got != tc.refusal {
				t.Errorf("refusal = %d, want %d", got, tc.refusal)
			}
			if e.Service != "community" || e.Op != "editPost" || e.Status != tc.answer.status || e.RequestID != "req-7" {
				t.Errorf("error = %+v", e)
			}
		})
	}
}

// Community carries the reason in the message; it has to reach the log, which
// is the only place that can tell a lost site binding from a lost secret.
func TestTheUpstreamReasonIsKeptForTheLog(t *testing.T) {
	_, err := fake(t, unbound).GetComments(context.Background(), 1, "42", "", "30", 0)
	e, _ := upstream.As(err)
	if e == nil || e.Code != "5" || e.Detail != "client is not bound to a site; it cannot act on the community" {
		t.Fatalf("error = %+v", e)
	}
}

func TestUnreachableCommunityIsUnavailable(t *testing.T) {
	c, srv := newClient(t, func(http.ResponseWriter, *http.Request) {})
	srv.Close()
	_, err := c.GetComments(context.Background(), 1, "42", "", "30", 0)
	if upstream.KindOf(err) != upstream.Unavailable {
		t.Fatalf("err = %v, want Unavailable", err)
	}
}

func TestUnconfiguredClientNeverDials(t *testing.T) {
	c := communityclient.New(communityclient.Config{})
	if c.Configured() {
		t.Fatal("Configured() is true with no base URL")
	}
	_, err := c.GetComments(context.Background(), 1, "1", "", "", 0)
	if !stderrors.Is(err, communityclient.ErrNotConfigured) || upstream.KindOf(err) != upstream.Unavailable {
		t.Errorf("err = %v, want an Unavailable ErrNotConfigured", err)
	}
}

// keyedComments fakes POST /comments as infra #302 answers it (handler/comments.go
// comment, service/writekey.go): a key seen with the same body replays the first
// post under Idempotency-Replayed: true, the same key on another body is a 409,
// and no key always writes.
func keyedComments(t *testing.T) (*communityclient.Client, *[]string) {
	seen := map[string]string{}
	var sent []string
	nextID := 900
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		raw, _ := json.Marshal(body)
		key := r.Header.Get("Idempotency-Key")
		sent = append(sent, key)
		w.Header().Set("Content-Type", "application/json")
		if first, ok := seen[key]; ok && key != "" {
			if first != string(raw) {
				w.WriteHeader(http.StatusConflict)
				_, _ = w.Write([]byte(`{"code":10,"message":"Idempotency-Key was reused with a different request"}`))
				return
			}
			w.Header().Set("Idempotency-Replayed", "true")
		} else {
			seen[key] = string(raw)
			nextID++
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "message": "成功", "data": map[string]any{
			"thread": map[string]any{"id": 7}, "post": map[string]any{"id": nextID},
		}})
	})
	return c, &sent
}

func TestARetriedCreateReplaysTheFirstPost(t *testing.T) {
	c, sent := keyedComments(t)
	req := communityclient.CommentRequest{AnchorKind: communityclient.AnchorSiteGame, AnchorID: "42", AuthorID: 3, Body: "hi"}

	first, err := c.CommentOnAnchor(context.Background(), req, "moyu-k1")
	if err != nil || first.Replayed {
		t.Fatalf("first = %+v, %v", first, err)
	}
	retry, err := c.CommentOnAnchor(context.Background(), req, "moyu-k1")
	if err != nil || !retry.Replayed || retry.Post.ID != first.Post.ID {
		t.Fatalf("retry = %+v, %v; want a replay of post %d", retry, err, first.Post.ID)
	}
	if (*sent)[0] != "moyu-k1" || (*sent)[1] != "moyu-k1" {
		t.Errorf("keys sent = %q", *sent)
	}

	req.Body = "hi again"
	_, err = c.CommentOnAnchor(context.Background(), req, "moyu-k1")
	if upstream.KindOf(err) != upstream.Conflict || communityclient.RefusalOf(err) != communityclient.RefusalKeyReused {
		t.Errorf("reused key on another body: %v", err)
	}
}

// Two identical comments a reader meant to post are two posts: without a key
// nothing is sent, so community writes both.
func TestAnUnkeyedCreateSendsNoKey(t *testing.T) {
	c, sent := keyedComments(t)
	req := communityclient.CommentRequest{AnchorKind: communityclient.AnchorSiteGame, AnchorID: "42", AuthorID: 3, Body: "+1"}
	a, _ := c.CommentOnAnchor(context.Background(), req, "")
	b, _ := c.CommentOnAnchor(context.Background(), req, "")
	if a.Post.ID == b.Post.ID || a.Replayed || b.Replayed || (*sent)[0] != "" {
		t.Errorf("a = %+v, b = %+v, keys = %q", a, b, *sent)
	}
}
