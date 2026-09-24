package storelink

import (
	"testing"
	"time"

	"kun-galgame-patch-api/pkg/storeclient"
	"kun-galgame-patch-api/pkg/upstream"
)

const tmpl = "https://dlaf.jp/soft/dlaf/=/t/s/link/work/aid/kungal/id/{workno}.html"

func TestResolve_FallsBackToTemplateAndEnqueues(t *testing.T) {
	r := New(Options{LinkTemplate: tmpl, StaticCoupon: "https://coupon.example"})

	got := r.Resolve("VJ013550")
	if got.PurchaseURL != "https://dlaf.jp/soft/dlaf/=/t/s/link/work/aid/kungal/id/VJ013550.html" {
		t.Errorf("purchase = %q, want the bare affiliate template", got.PurchaseURL)
	}
	if got.CouponURL != "https://coupon.example" || got.CampaignName != "" {
		t.Errorf("static coupon must carry no campaign name, got %+v", got)
	}
}

func TestResolve_PrefersTheMintedShortLink(t *testing.T) {
	r := New(Options{LinkTemplate: tmpl})
	r.remember("RJ297925", "https://s.example/s/abc")

	if got := r.Resolve("RJ297925"); got.PurchaseURL != "https://s.example/s/abc" {
		t.Errorf("purchase = %q, want the short link", got.PurchaseURL)
	}
}

func TestResolve_RunningCampaignBeatsTheStaticCoupon(t *testing.T) {
	r := New(Options{LinkTemplate: tmpl, StaticCoupon: "https://coupon.example"})
	r.setCampaign(&storeclient.Campaign{ID: "7", Name: "夏日特惠"}, "https://s.example/s/def")

	got := r.Resolve("VJ013550")
	if got.CouponURL != "https://s.example/s/def" || got.CampaignName != "夏日特惠" {
		t.Errorf("got %+v, want the campaign's link and name", got)
	}

	// An ended campaign has to hand the standing landing page back, not leave the
	// dead campaign link on the button.
	r.setCampaign(nil, "")
	if got := r.Resolve("VJ013550"); got.CouponURL != "https://coupon.example" || got.CampaignName != "" {
		t.Errorf("after the campaign ends: %+v", got)
	}
}

func TestResolve_NoWorknoAndNoTemplateRenderNothing(t *testing.T) {
	r := New(Options{LinkTemplate: tmpl})
	if got := r.Resolve(""); got != (Links{}) {
		t.Errorf("a game with no workno must render nothing, got %+v", got)
	}

	// A coupon with no purchase link is not a usable entry: the frontend hangs
	// the coupon inside the purchase popover.
	bare := New(Options{StaticCoupon: "https://coupon.example"})
	if got := bare.Resolve("VJ013550"); got != (Links{}) {
		t.Errorf("no template and no short link must render nothing, got %+v", got)
	}
}

func TestResolve_NilResolverIsSafe(t *testing.T) {
	var r *Resolver
	if got := r.Resolve("RJ297925"); got != (Links{}) {
		t.Errorf("nil resolver = %+v", got)
	}
	if r.Configured() {
		t.Error("nil resolver must report unconfigured")
	}
}

func TestEnqueue_IsANoOpWithoutTheStoreFace(t *testing.T) {
	r := New(Options{LinkTemplate: tmpl})
	r.enqueue("RJ297925")
	if len(r.queue) != 0 {
		t.Errorf("queued %d worknos with no store client configured", len(r.queue))
	}
}

func TestMintFailed_RateLimitPausesTheMinter(t *testing.T) {
	now := time.Date(2026, 9, 24, 21, 30, 0, 0, time.UTC)
	limited := func(code string, retryAfter time.Duration) error {
		return &upstream.Error{Service: "store", Kind: upstream.RateLimited, Status: 429,
			Code: code, RetryAfter: retryAfter, Cause: storeclient.ErrRateLimited}
	}
	for _, tc := range []struct {
		name string
		err  error
		want time.Duration
	}{
		{"Retry-After wins", limited("QUOTA_EXCEEDED", 5*time.Hour), 5 * time.Hour},
		{"a spent daily quota waits for the UTC day", limited("QUOTA_EXCEEDED", 0), 150 * time.Minute},
		{"a rate limit waits a minute", limited("RATE_LIMITED", 0), time.Minute},
		{"anything else keeps the pace", storeclient.ErrUpstream, mintInterval},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := New(Options{LinkTemplate: tmpl})
			if got := r.mintFailed("RJ297925", tc.err, now); got != tc.want {
				t.Errorf("wait = %v, want %v", got, tc.want)
			}
			if r.stopped() {
				t.Error("a rate limit must pause the minter, not halt it")
			}
		})
	}
}
