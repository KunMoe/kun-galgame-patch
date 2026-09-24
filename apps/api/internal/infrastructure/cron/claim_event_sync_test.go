package cron

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"kun-galgame-patch-api/pkg/catalogv2"
)

func ptr[T any](v T) *T { return &v }

func TestEffectOfMapsEveryTransition(t *testing.T) {
	gid := int64(4242)
	cases := []struct {
		name string
		ev   catalogv2.ClaimEvent
		want claimEffect
	}{
		{
			name: "pending → live is an approval",
			ev: catalogv2.ClaimEvent{
				FromState: ptr(catalogv2.ClaimStatePending),
				ToState:   catalogv2.ClaimStateLive, ProductWorkID: &gid,
			},
			want: claimEffectApproved,
		},
		{
			name: "draft → live is the owner publishing, already rewarded in-request",
			ev: catalogv2.ClaimEvent{
				FromState: ptr(catalogv2.ClaimStateDraft),
				ToState:   catalogv2.ClaimStateLive, ProductWorkID: &gid,
			},
			want: claimEffectNone,
		},
		{
			name: "hidden → live is an unban",
			ev: catalogv2.ClaimEvent{
				FromState: ptr(catalogv2.ClaimStateHidden),
				ToState:   catalogv2.ClaimStateLive, ProductWorkID: &gid,
			},
			want: claimEffectUnbanned,
		},
		{
			name: "birth into live is an import, not a verdict",
			ev: catalogv2.ClaimEvent{
				ToState: catalogv2.ClaimStateLive, ProductWorkID: &gid,
			},
			want: claimEffectNone,
		},
		{
			name: "declined",
			ev: catalogv2.ClaimEvent{
				FromState: ptr(catalogv2.ClaimStatePending),
				ToState:   catalogv2.ClaimStateDeclined, ProductWorkID: &gid,
			},
			want: claimEffectDeclined,
		},
		{
			name: "hidden is a ban",
			ev: catalogv2.ClaimEvent{
				FromState: ptr(catalogv2.ClaimStateLive),
				ToState:   catalogv2.ClaimStateHidden, ProductWorkID: &gid,
			},
			want: claimEffectBanned,
		},
		{
			name: "pending records the submitter",
			ev: catalogv2.ClaimEvent{
				FromState: ptr(catalogv2.ClaimStateDraft),
				ToState:   catalogv2.ClaimStatePending, ProductWorkID: &gid,
			},
			want: claimEffectRememberSubmitter,
		},
		{
			name: "live → draft is a withdrawal, nothing to announce",
			ev: catalogv2.ClaimEvent{
				FromState: ptr(catalogv2.ClaimStateLive),
				ToState:   catalogv2.ClaimStateDraft, ProductWorkID: &gid,
			},
			want: claimEffectNone,
		},
		{
			name: "an approval with no product anchor is reported, not silently skipped",
			ev: catalogv2.ClaimEvent{
				FromState: ptr(catalogv2.ClaimStatePending),
				ToState:   catalogv2.ClaimStateLive,
			},
			want: claimEffectUnanchored,
		},
		{
			name: "a transition with nothing to do needs no anchor",
			ev: catalogv2.ClaimEvent{
				FromState: ptr(catalogv2.ClaimStateLive),
				ToState:   catalogv2.ClaimStateDraft,
			},
			want: claimEffectNone,
		},
		{
			name: "an unrecognised destination is reported",
			ev: catalogv2.ClaimEvent{
				ToState: "quarantined", ProductWorkID: &gid,
			},
			want: claimEffectUnknownState,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := effectOf(&tc.ev); got != tc.want {
				t.Errorf("effectOf = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestClaimFeedRequestPinsTheTenant(t *testing.T) {
	var queries []url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queries = append(queries, r.URL.Query())
		_, _ = w.Write([]byte(`{"object":"list","items":[]}`))
	}))
	t.Cleanup(srv.Close)
	cli := catalogv2.New(srv.URL, "nmk_test_x")

	if _, err := cli.ClaimEventHead(context.Background(), claimSyncSite); err != nil {
		t.Fatal(err)
	}
	if _, err := cli.ClaimEvents(context.Background(), 40, claimSyncBatch, claimSyncSite); err != nil {
		t.Fatal(err)
	}
	if len(queries) != 2 {
		t.Fatalf("calls %d", len(queries))
	}
	for _, q := range queries {
		if q.Get("site") != "kungal" {
			t.Errorf("every claim read must pin the tenant, else product_work_id "+
				"names another site's rows: %v", q)
		}
	}
	if got := queries[0].Get("sort"); got != "recorded_desc" {
		t.Errorf("seeding reads the head, sort=%q", got)
	}
	if got := queries[1].Get("sort"); got != "recorded_asc" {
		t.Errorf("the watermark walk is ascending, sort=%q", got)
	}
	if got := queries[1].Get("cursor"); got != catalogv2.EncodeCursor("40") {
		t.Errorf("cursor=%q must carry the stored watermark", got)
	}
	if claimSyncBatch > 100 {
		t.Fatalf("limit %d is 400 LIMIT_TOO_LARGE, not clamped", claimSyncBatch)
	}
}
