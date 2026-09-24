package engagement

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"kun-galgame-patch-api/pkg/communityclient"
	"kun-galgame-patch-api/pkg/upstream"
)

func TestSubscribedThreshold(t *testing.T) {
	if toState(1, &communityclient.ThreadUserView{NotificationLevel: 0}).Subscribed {
		t.Error("muted is subscribed")
	}
	if toState(1, &communityclient.ThreadUserView{NotificationLevel: 1}).Subscribed {
		t.Error("normal is subscribed")
	}
	if !toState(1, &communityclient.ThreadUserView{NotificationLevel: 3}).Subscribed {
		t.Error("watching is not subscribed")
	}
}

type reply struct {
	status int
	body   string
}

func ok(data any) reply {
	b, _ := json.Marshal(map[string]any{"code": 0, "message": "成功", "data": data})
	return reply{200, string(b)}
}

type community struct {
	mu    sync.Mutex
	calls []string
}

func fakeCommunity(t *testing.T, routes map[string]reply) (*Service, *community) {
	t.Helper()
	fc := &community{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fc.mu.Lock()
		fc.calls = append(fc.calls, r.Method+" "+r.URL.Path+"?"+r.URL.RawQuery)
		fc.mu.Unlock()
		rep, found := routes[r.Method+" "+r.URL.Path]
		if !found {
			t.Errorf("unexpected community call %s %s", r.Method, r.URL.Path)
			rep = reply{404, `{"code":404,"message":"Cannot ` + r.Method + ` ` + r.URL.Path + `"}`}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(rep.status)
		_, _ = io.WriteString(w, rep.body)
	}))
	t.Cleanup(srv.Close)
	cc := communityclient.New(communityclient.Config{BaseURL: srv.URL, ClientID: "moyu", ClientSecret: "secret"})
	return New(cc, nil, nil), fc
}

var (
	resourceWall   = ok(map[string]any{"thread": map[string]any{"id": 70, "anchor_kind": 2, "anchor_id": "678"}, "posts": []any{}})
	noThreadYet    = ok(map[string]any{"posts": []any{}})
	anchorWatching = ok(map[string]any{"user_id": 3, "anchor_kind": 2, "anchor_id": "678", "notification_level": 3})
	threadWatching = ok(map[string]any{"thread_id": 70, "user_id": 3, "notification_level": 3})
)

// The page used to send the thread id beside the anchor, and a crafted one set
// the level of whatever thread it named. The thread now comes from community,
// filed under the anchor being followed.
func TestFollowActsOnTheThreadFiledUnderTheAnchor(t *testing.T) {
	svc, fc := fakeCommunity(t, map[string]reply{
		"GET /comments":                 resourceWall,
		"POST /anchors/notification":    anchorWatching,
		"POST /threads/70/notification": threadWatching,
		"POST /threads/70/read":         threadWatching,
	})
	state, err := svc.SetWallLevel(context.Background(), 3, communityclient.AnchorSiteResource, "678", communityclient.NotificationWatching)
	if err != nil {
		t.Fatal(err)
	}
	if state.ThreadID != 70 || !state.Subscribed {
		t.Errorf("state = %+v", state)
	}
	if !strings.Contains(fc.calls[0], "anchor_kind=2") || !strings.Contains(fc.calls[0], "anchor_id=678") {
		t.Errorf("thread looked up as %q", fc.calls[0])
	}
}

func TestFollowBeforeTheFirstCommentIsOnlyAnAnchorSubscription(t *testing.T) {
	svc, fc := fakeCommunity(t, map[string]reply{
		"GET /comments":              noThreadYet,
		"POST /anchors/notification": anchorWatching,
	})
	state, err := svc.SetWallLevel(context.Background(), 3, communityclient.AnchorSiteResource, "678", communityclient.NotificationWatching)
	if err != nil {
		t.Fatal(err)
	}
	if state.ThreadID != 0 || !state.Subscribed || len(fc.calls) != 2 {
		t.Errorf("state = %+v after %v", state, fc.calls)
	}
}

func TestAFailedThreadLevelAfterTheAnchorWriteIsSurfaced(t *testing.T) {
	svc, _ := fakeCommunity(t, map[string]reply{
		"GET /comments":                 resourceWall,
		"POST /anchors/notification":    anchorWatching,
		"POST /threads/70/notification": {502, "<html>502 Bad Gateway</html>"},
	})
	_, err := svc.SetWallLevel(context.Background(), 3, communityclient.AnchorSiteResource, "678", communityclient.NotificationNormal)
	if upstream.KindOf(err) != upstream.Unavailable {
		t.Fatalf("err = %v, want Unavailable", err)
	}
}

// The read receipt is a background call the page does not wait on, so it
// answers an empty state either way; what differs is how loudly it says so.
func TestReadReceiptLogsALostBindingAsAnError(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	svc, _ := fakeCommunity(t, map[string]reply{
		"GET /comments": {403, `{"code":5,"message":"client is not bound to a site; it cannot act on the community"}`},
	})
	state := svc.ReadWall(context.Background(), 3, communityclient.AnchorSiteGame, "42")
	if state.ThreadID != 0 || state.Subscribed {
		t.Errorf("state = %+v", state)
	}
	if !strings.Contains(buf.String(), "level=ERROR") {
		t.Errorf("log = %q, want an ERROR line", buf.String())
	}
}
