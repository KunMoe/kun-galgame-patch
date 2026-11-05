package client

import "testing"

func TestClaimSiteAcceptedOnBothSpellings(t *testing.T) {
	cases := []struct {
		site string
		want bool
	}{
		{"kungal", true},
		{"galgame_wiki", true},
		{"moyu", false},
		{"", false},
	}
	for _, tc := range cases {
		t.Run(tc.site, func(t *testing.T) {
			c := &catalogClaimedBy{Site: tc.site, WorkID: 4321, State: catalogClaimStateLive}
			if got := c.live(); got != tc.want {
				t.Errorf("live() on site %q = %v, want %v", tc.site, got, tc.want)
			}
			if got := claimStateOf(c); (got == catalogClaimStateLive) != tc.want {
				t.Errorf("claimStateOf on site %q = %q", tc.site, got)
			}
		})
	}
}

func TestHiddenClaimIsNotRenderable(t *testing.T) {
	hidden := &catalogClaimedBy{Site: catalogClaimSiteKungal, WorkID: 921, State: catalogClaimStateHidden}
	if hidden.renderable() {
		t.Error("a hidden claim must not be renderable")
	}
}
