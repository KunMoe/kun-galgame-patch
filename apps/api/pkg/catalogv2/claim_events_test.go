package catalogv2

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The row shape nextmoe-infra's handler/catalog_claim_events.go builds from
// repr.ClaimEvent.
func claimEventJSON(id, workID string) string {
	return `{"object":"claim_event","id":"` + id + `","work_id":"` + workID + `","from_state":"pending",` +
		`"to_state":"declined","reason":null,"actor_uid":"42","site":"kungal","product_work_id":"` + workID + `",` +
		`"created_at":"2026-09-01T00:00:00Z"}`
}

func TestClaimEventsHoldTheWatermarkAtAnUnreadableRow(t *testing.T) {
	for _, tc := range []struct {
		name string
		bad  string
	}{
		{"an id that is not a catalog id", claimEventJSON("x9", "7")},
		{"a work_id that is not a catalog id", claimEventJSON("12", "")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{"object":"list","items":[` +
					claimEventJSON("10", "7") + `,` + claimEventJSON("11", "8") + `,` + tc.bad + `,` +
					claimEventJSON("13", "9") + `]}`))
			}))
			t.Cleanup(srv.Close)

			rows, err := New(srv.URL, "nmk_test_key").ClaimEvents(context.Background(), 9, 100, SiteKungal)
			if err == nil {
				t.Fatal("want an error: skipping the row lets event 13 move the watermark past it for good")
			}
			if len(rows) != 2 || rows[0].ID != 10 || rows[1].ID != 11 {
				t.Fatalf("rows = %+v, want the two events before the unreadable one", rows)
			}
		})
	}
}

func TestFeedDropsAreLogged(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	bodies := map[string]string{
		"/v2/catalog/changes": `{"object":"list","items":[` +
			`{"object":"change","target_object":"character","id":"70","updated_at":"2026-08-01T00:00:00Z"},` +
			`{"object":"change","target_object":"work","id":"","updated_at":"2026-08-01T00:00:01Z"}]}`,
		"/v2/catalog/redirects": `{"object":"list","items":[` +
			`{"object":"redirect","target_object":"work","old_id":"8","current_id":"","merged_at":"2026-09-14T02:00:00Z"}]}`,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(bodies[r.URL.Path]))
	}))
	t.Cleanup(srv.Close)
	c := New(srv.URL, "nmk_test_key")

	if _, err := c.Changes(context.Background(), "", 100); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Redirects(context.Background(), "", 100); err != nil {
		t.Fatal(err)
	}
	logged := buf.String()
	for _, want := range []string{"character:70", "changes feed: dropped a row", "old_id=8"} {
		if !strings.Contains(logged, want) {
			t.Errorf("log lacks %q:\n%s", want, logged)
		}
	}
	if n := strings.Count(logged, "level=WARN"); n != 3 {
		t.Errorf("WARN lines = %d, want 3 (one per page of foreign rows, one per bad id)", n)
	}
}
