package rating

import (
	"encoding/json"
	"net/http"
	"testing"

	"kun-galgame-patch-api/internal/middleware"
	"kun-galgame-patch-api/internal/testutil"
	"kun-galgame-patch-api/pkg/config"
)

func ratingApp(t *testing.T, cat *fakeCatalog) *testutil.TestApp {
	t.Helper()
	ta := testutil.NewTestApp(t)
	store := &fakeStore{rows: map[int64]Row{501: {ID: 501, WorkID: 58, UserID: 2, Overall: 7, Recommend: "yes"}}}
	h := NewHandler(NewService(store, cat, nil, nil), nil)
	oauth := config.OAuthConfig{}
	ta.App.Get("/patch/:id/rating", middleware.OptionalAuth(ta.RDB, oauth), h.List)
	ta.App.Delete("/patch/rating/:ratingId", middleware.Auth(ta.RDB, oauth), h.Delete)
	return ta
}

func listedCanDelete(t *testing.T, ta *testutil.TestApp, session string) bool {
	t.Helper()
	r := testutil.ParseResponse(t, ta.Request(t, http.MethodGet, "/patch/58/rating", "", session))
	raw, _ := json.Marshal(r.Data)
	var page struct {
		Items []Item `json:"items"`
	}
	if err := json.Unmarshal(raw, &page); err != nil || len(page.Items) != 1 {
		t.Fatalf("list = %s (%v)", raw, err)
	}
	return page.Items[0].CanDelete
}

// Catalog grants a site moderator catalog.rating.moderate, so moyu is the only
// gate between 39 site-only moderators and every site's ratings.
func TestASiteModeratorCannotDeleteAnotherReadersRating(t *testing.T) {
	cat := &fakeCatalog{}
	ta := ratingApp(t, cat)
	mod := ta.CreateTestSessionSiteRoles(t, 3, nil, []string{"moderator"})

	if listedCanDelete(t, ta, mod) {
		t.Error("a site moderator is offered the delete on another reader's rating")
	}
	resp := ta.Request(t, http.MethodDelete, "/patch/rating/501", "", mod)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want 403", resp.StatusCode)
	}
	if len(cat.moderated)+len(cat.deletedFor) != 0 {
		t.Errorf("catalog was asked to delete: moderated %v, own %v", cat.moderated, cat.deletedFor)
	}
}

func TestAnAdminDeletesThroughModeration(t *testing.T) {
	cat := &fakeCatalog{}
	ta := ratingApp(t, cat)
	admin := ta.CreateTestSession(t, 4, "admin")

	if !listedCanDelete(t, ta, admin) {
		t.Error("an admin is not offered the delete")
	}
	resp := ta.Request(t, http.MethodDelete, "/patch/rating/501", "", admin)
	if resp.StatusCode != http.StatusOK || len(cat.moderated) != 1 || cat.moderated[0] != 501 {
		t.Errorf("status %d, moderated %v; want rating 501 deleted through moderation", resp.StatusCode, cat.moderated)
	}
}
