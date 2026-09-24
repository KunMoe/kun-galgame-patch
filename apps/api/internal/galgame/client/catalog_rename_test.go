package client

import "testing"

func TestClaimSiteAcceptedOnBothSpellings(t *testing.T) {
	cases := []struct {
		site string
		want int
	}{
		{"kungal", 4321},
		{"galgame_wiki", 4321},
		{"moyu", 0},
		{"", 0},
	}
	for _, tc := range cases {
		t.Run(tc.site, func(t *testing.T) {
			c := &catalogClaimedBy{Site: tc.site, WorkID: 4321, State: catalogClaimStateLive}
			if got := c.forumGID(); got != tc.want {
				t.Errorf("forumGID() on site %q = %d, want %d", tc.site, got, tc.want)
			}
			if got := c.live(); got != (tc.want != 0) {
				t.Errorf("live() on site %q = %v, want %v", tc.site, got, tc.want != 0)
			}
			if got := claimStateOf(c); (got == catalogClaimStateLive) != (tc.want != 0) {
				t.Errorf("claimStateOf on site %q = %q", tc.site, got)
			}
		})
	}
}

// The page id is the catalog work id and nothing else elects it. The claim's
// site_work_id is the FORUM's number for the same game, and this test exists
// because the old code returned that as our own: work 900 claimed by page 7
// answered 7, so one integer meant two games depending on who asked.
func TestPublicGIDIsAlwaysTheCatalogID(t *testing.T) {
	unclaimed := &catalogWorkListItem{ID: 930}
	if got := unclaimed.publicGID(); got != 930 {
		t.Errorf("unclaimed publicGID = %d, want the catalog id 930", got)
	}
	claimed := &catalogWorkListItem{
		ID:        900,
		ClaimedBy: &catalogClaimedBy{Site: catalogClaimSiteKungal, WorkID: 7, State: catalogClaimStateLive},
	}
	if got := claimed.publicGID(); got != 900 {
		t.Errorf("claimed publicGID = %d, want the catalog id 900", got)
	}
	if got := claimed.ClaimedBy.forumGID(); got != 7 {
		t.Errorf("forumGID = %d, want the forum's own page id 7", got)
	}
	hidden := &catalogWorkListItem{
		ID:        921,
		ClaimedBy: &catalogClaimedBy{Site: catalogClaimSiteKungal, WorkID: 21, State: catalogClaimStateHidden},
	}
	if hidden.ClaimedBy.renderable() {
		t.Error("a hidden claim must not be renderable")
	}
}
