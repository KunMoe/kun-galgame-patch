package anchor

import (
	"errors"
	"testing"

	"kun-galgame-patch-api/pkg/communityclient"
)

type stubOwner map[int]int

func (s stubOwner) ResourcePatchIDs(resourceIDs []int) (map[int]int, error) {
	out := map[int]int{}
	for _, id := range resourceIDs {
		out[id] = s[id]
	}
	return out, nil
}

type failingOwner struct{}

func (failingOwner) ResourcePatchIDs([]int) (map[int]int, error) {
	return nil, errors.New("connection refused")
}

func TestResolveOnlyClaimsSiteLocalAnchors(t *testing.T) {
	r := New(nil, stubOwner{678: 42})

	refs := []Ref{
		{communityclient.AnchorSiteGame, "12345"},
		{communityclient.AnchorSiteResource, "678"},
		// The site feed answers catalog-anchored threads whoever opened them:
		// they are one network-wide conversation, not a moyu wall.
		{communityclient.AnchorCatalogWork, "12345"},
		{communityclient.AnchorCatalogPerson, "12345"},
		{communityclient.AnchorBoard, "1"},
		{communityclient.AnchorSiteGame, "0"},
		{communityclient.AnchorSiteGame, "moyu:12345"},
	}

	got, err := r.Resolve(refs)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("resolved %d anchors, want 2: %v", len(got), got)
	}

	game := got[Ref{communityclient.AnchorSiteGame, "12345"}]
	if game.Link != "/galgame/12345?tab=comment" || game.PatchID != 12345 {
		t.Errorf("game target = %+v", game)
	}

	res := got[Ref{communityclient.AnchorSiteResource, "678"}]
	if res.Link != "/resource/678" || res.ResourceID != 678 || res.PatchID != 42 {
		t.Errorf("resource target = %+v", res)
	}
}

func TestAnchorMinting(t *testing.T) {
	kind, id := PatchAnchor(12345)
	if kind != communityclient.AnchorSiteGame || id != "12345" {
		t.Errorf("PatchAnchor = %d/%q", kind, id)
	}
	kind, id = ResourceAnchor(678)
	if kind != communityclient.AnchorSiteResource || id != "678" {
		t.Errorf("ResourceAnchor = %d/%q", kind, id)
	}
}

// The sweeps that walk the whole corpus ask this instead of resolving, so it has
// to draw the same line Resolve does — and without a database.
func TestIsMoyuMatchesResolve(t *testing.T) {
	cases := []struct {
		kind int32
		id   string
		want bool
	}{
		{communityclient.AnchorSiteGame, "12345", true},
		{communityclient.AnchorSiteResource, "678", true},
		{communityclient.AnchorCatalogWork, "12345", false},
		{communityclient.AnchorCatalogPerson, "12345", false},
		{communityclient.AnchorBoard, "1", false},
		{communityclient.AnchorSiteGame, "moyu:12345", false},
		{communityclient.AnchorSiteGame, "", false},
	}
	r := New(nil, stubOwner{678: 42})
	for _, tc := range cases {
		if got := IsMoyu(tc.kind, tc.id); got != tc.want {
			t.Errorf("IsMoyu(%d, %q) = %v, want %v", tc.kind, tc.id, got, tc.want)
		}
		got, err := r.Resolve([]Ref{{tc.kind, tc.id}})
		if err != nil {
			t.Fatal(err)
		}
		if _, resolved := got[Ref{tc.kind, tc.id}]; resolved != tc.want {
			t.Errorf("Resolve disagrees for (%d, %q): %v", tc.kind, tc.id, resolved)
		}
	}
}

// A resource wall whose game could not be read used to resolve with PatchID 0,
// which the inbox reads as "resource deleted" and drops the notification for
// good while its cursor moves on.
func TestResolveFailsWhenTheResourceLookupFails(t *testing.T) {
	r := New(nil, failingOwner{})
	if _, err := r.Resolve([]Ref{{communityclient.AnchorSiteResource, "678"}}); err == nil {
		t.Fatal("a failed resource lookup resolved")
	}
	if _, err := r.Resolve([]Ref{{communityclient.AnchorSiteGame, "42"}}); err != nil {
		t.Fatalf("a game wall needs no lookup, got %v", err)
	}
}
