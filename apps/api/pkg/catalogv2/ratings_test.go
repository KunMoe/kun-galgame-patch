package catalogv2

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"kun-galgame-patch-api/pkg/upstream"
)

type capture struct {
	method string
	path   string
	auth   string
	body   map[string]json.RawMessage
}

func ratingServer(t *testing.T, status int, reply string, got *capture) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.method, got.path, got.auth = r.Method, r.URL.RequestURI(), r.Header.Get("Authorization")
		if raw, _ := io.ReadAll(r.Body); len(raw) > 0 {
			got.body = map[string]json.RawMessage{}
			if err := json.Unmarshal(raw, &got.body); err != nil {
				t.Errorf("request body is not an object: %s", raw)
			}
		}
		ct := "application/json"
		if status >= 400 {
			ct = "application/problem+json"
		}
		w.Header().Set("Content-Type", ct)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(reply))
	}))
	t.Cleanup(srv.Close)
	return New(srv.URL, "nmk_test_key")
}

const ratingJSON = `{"object":"rating","id":"501","work_id":"58","user_id":"2","overall":9,
	"aspect_scores":{"art":8,"story":null,"music":null,"character":10,"route":null,"system":null,"voice":null,"replay_value":null},
	"recommend":"strong_yes","spoiler_level":"portion","short_summary":"好","client_id":"df3f","site":"kungal",
	"work_state":{"state":"done","completion":"all"},
	"created_at":"2026-10-12T08:00:00Z","updated_at":"2026-10-12T09:30:00Z"}`

// PUT replaces the whole record: a part the body carries no key for is deleted
// upstream, so a nil part must be absent, and a present one must be sent whole.
func TestPutWorkRecordSendsOnlyThePartsItHas(t *testing.T) {
	ctx := context.Background()
	var got capture
	c := ratingServer(t, 200, `{"object":"work_record","work_id":"58","work_state":{"state":"wish","completion":null},"playtime":null,"rating":null}`, &got)

	rec, err := c.PutWorkRecord(ctx, "user_tok", 58, WorkRecordInput{WorkState: &WorkStateInput{State: "wish"}})
	if err != nil {
		t.Fatalf("PutWorkRecord: %v", err)
	}
	if got.method != http.MethodPut || got.path != "/v2/me/work-records/58" || got.auth != "Bearer user_tok" {
		t.Fatalf("request = %s %s (%s), want PUT /v2/me/work-records/58 with the reader's token", got.method, got.path, got.auth)
	}
	for _, k := range []string{"rating", "playtime"} {
		if _, ok := got.body[k]; ok {
			t.Errorf("body carries %q although the form left it out: %s", k, got.body[k])
		}
	}
	if string(got.body["work_state"]) != `{"state":"wish"}` {
		t.Errorf("work_state = %s, want the state without a completion", got.body["work_state"])
	}
	if rec.State != "wish" || rec.Rating != nil || rec.Minutes != 0 {
		t.Errorf("record = %+v", rec)
	}

	nine := 9
	_, err = c.PutWorkRecord(ctx, "user_tok", 58, WorkRecordInput{
		WorkState: &WorkStateInput{State: "done", Completion: "main"},
		Playtime:  &PlaytimeInput{Minutes: 600},
		Rating: &RatingInput{Overall: 8, Recommend: "yes", SpoilerLevel: "none",
			AspectScores: &RatingAspects{Art: &nine}},
	})
	if err != nil {
		t.Fatalf("PutWorkRecord: %v", err)
	}
	if string(got.body["playtime"]) != `{"minutes":600}` {
		t.Errorf("playtime = %s", got.body["playtime"])
	}
	var r map[string]json.RawMessage
	if err := json.Unmarshal(got.body["rating"], &r); err != nil {
		t.Fatalf("rating = %s", got.body["rating"])
	}
	if string(r["aspect_scores"]) != `{"art":9,"story":null,"music":null,"character":null,"route":null,"system":null,"voice":null,"replay_value":null}` {
		t.Errorf("aspect_scores = %s, want every key with unrated ones null", r["aspect_scores"])
	}
}

func TestWorkRecordReadsCatalogsShape(t *testing.T) {
	var got capture
	c := ratingServer(t, 200, `{"object":"work_record","work_id":"58",
		"work_state":{"object":"work_state","work_id":"58","state":"done","completion":null},
		"playtime":{"object":"playtime","work_id":"58","minutes":1200},
		"rating":`+ratingJSON+`}`, &got)
	rec, err := c.WorkRecord(context.Background(), "user_tok", 58)
	if err != nil {
		t.Fatalf("WorkRecord: %v", err)
	}
	if rec.State != "done" || rec.Completion != nil || rec.Minutes != 1200 {
		t.Errorf("record = %+v", rec)
	}
	r := rec.Rating
	if r == nil || r.ID != 501 || r.WorkID != 58 || r.UserID != 2 || r.Overall != 9 {
		t.Fatalf("rating = %+v", r)
	}
	if r.Aspects.Art == nil || *r.Aspects.Art != 8 || r.Aspects.Story != nil || *r.Aspects.Character != 10 {
		t.Errorf("aspects = %+v", r.Aspects)
	}
	if r.State != "done" || r.Completion == nil || *r.Completion != "all" || r.ClientID != "df3f" {
		t.Errorf("author state / client = %q %v %q", r.State, r.Completion, r.ClientID)
	}
	if !r.UpdatedAt.Equal(time.Date(2026, 10, 12, 9, 30, 0, 0, time.UTC)) {
		t.Errorf("updated_at = %v", r.UpdatedAt)
	}
}

func TestRatingChangesIsTheUncachedMirrorChannel(t *testing.T) {
	if readCacheable("/v2/ratings/changes?limit=100") {
		t.Fatal("the ratings feed drives a cursor loop; a cached page would skip a window")
	}
	var got capture
	c := ratingServer(t, 200, `{"object":"list","items":[
		{"object":"rating_change","id":"501","work_id":"58","gone":false,"changed_at":"2026-10-12T09:30:00.123Z","rating":`+ratingJSON+`},
		{"object":"rating_change","id":"502","work_id":"58","gone":true,"changed_at":"2026-10-12T09:31:00.5Z","rating":null}
	],"next_cursor":"cur_x"}`, &got)
	page, err := c.RatingChanges(context.Background(), "cur_prev")
	if err != nil {
		t.Fatalf("RatingChanges: %v", err)
	}
	if got.path != "/v2/ratings/changes?cursor=cur_prev&limit=100" || got.auth != "Bearer nmk_test_key" {
		t.Errorf("request = %s (%s), want the application key", got.path, got.auth)
	}
	if len(page.Items) != 2 || page.NextCursor != "cur_x" {
		t.Fatalf("page = %+v", page)
	}
	if it := page.Items[0]; it.ID != 501 || it.Gone || it.Rating == nil || it.Rating.WorkID != 58 {
		t.Errorf("written entry = %+v", it)
	}
	if it := page.Items[1]; it.ID != 502 || !it.Gone || it.Rating != nil {
		t.Errorf("gone entry = %+v", it)
	}
}

// The rating delete button shows for moyu's moderators, who need not hold
// catalog.rating.moderate; the refusal has to stay recognisable as one.
func TestModerationDeleteRefusalKeepsItsCode(t *testing.T) {
	var got capture
	c := ratingServer(t, 403, `{"type":"about:blank","title":"Forbidden","status":403,
		"code":"PERMISSION_REQUIRED","detail":"needs catalog.rating.moderate"}`, &got)
	err := c.ModerationDeleteRating(context.Background(), "mod_tok", 501)
	if got.method != http.MethodDelete || got.path != "/v2/moderation/ratings/501" || got.auth != "Bearer mod_tok" {
		t.Fatalf("request = %s %s (%s)", got.method, got.path, got.auth)
	}
	e, ok := upstream.As(err)
	if !ok || e.Code != "PERMISSION_REQUIRED" || e.Kind != upstream.Rejected || e.Status != 403 {
		t.Fatalf("err = %#v, want a Rejected PERMISSION_REQUIRED 403", err)
	}
}
