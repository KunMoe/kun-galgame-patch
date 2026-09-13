package main

import "testing"

func rows(in ...patchRow) []patchRow { return in }

func targetOf(t *testing.T, p *Plan, oldID int) (int, string) {
	t.Helper()
	for _, r := range p.Rows {
		if r.OldID == oldID {
			return r.NewID, r.Action
		}
	}
	t.Fatalf("no plan row for %d", oldID)
	return 0, ""
}

func TestPlanRenumbersToTheCatalogWork(t *testing.T) {
	p := buildPlan(
		rows(
			patchRow{ID: 4115, VndbID: "v1523", ResourceCount: 2, Published: true},
			patchRow{ID: 900, VndbID: "v9", Published: true},
		),
		map[int]int64{4115: 4082, 900: 900},
	)
	if to, action := targetOf(t, p, 4115); to != 4082 || action != "align" {
		t.Errorf("4115 -> %d (%s), want 4082 align", to, action)
	}
	if _, action := targetOf(t, p, 900); action != "identical" {
		t.Errorf("900 action = %s, want identical", action)
	}
	if p.Identical != 1 || len(p.Moves) != 1 {
		t.Errorf("identical = %d, moves = %d, want 1 and 1", p.Identical, len(p.Moves))
	}
	// Every pre-037 page id is in the ledger, moved or not. /patch/<n> reads
	// nothing else, so a miss has to mean "no page ever had that number"; if a
	// miss meant "same number" instead, one absent row would quietly serve a
	// different game.
	if len(p.Redirects) != 2 {
		t.Fatalf("redirects = %+v, want one per page including the unmoved one", p.Redirects)
	}
}

// Two pages naming one work is what catalog's own merges leave behind, and the
// renumber cannot land both on the same primary key. The survivor is the page a
// reader should keep landing on.
func TestPlanFoldsTwoPagesOntoTheSurvivor(t *testing.T) {
	p := buildPlan(
		rows(
			patchRow{ID: 4107, VndbID: "v8739", ResourceCount: 1, Published: true},
			patchRow{ID: 4115, VndbID: "v1523", ResourceCount: 2, Published: true},
		),
		map[int]int64{4107: 4082, 4115: 4082},
	)
	if len(p.Folds) != 1 {
		t.Fatalf("folds = %+v, want one", p.Folds)
	}
	f := p.Folds[0]
	if f.Target != 4082 || f.Survivor != 4115 || len(f.Losers) != 1 || f.Losers[0] != 4107 {
		t.Fatalf("fold = %+v, want 4107 folded into 4115 at work 4082", f)
	}
	if to, action := targetOf(t, p, 4107); to != 4082 || action != "fold" {
		t.Errorf("loser 4107 -> %d (%s), want 4082 fold", to, action)
	}
	// The loser redirects to the WORK, not to the survivor's pre-renumber id:
	// the survivor is about to move too.
	for _, r := range p.Redirects {
		if r.OldID == 4107 && r.NewID != 4082 {
			t.Errorf("4107 redirects to %d, want the final id 4082", r.NewID)
		}
	}
}

func TestSurvivorPrefersPublishedThenResources(t *testing.T) {
	stub := patchRow{ID: 5235, IsStub: true}
	real := patchRow{ID: 1245, ResourceCount: 1, Published: true}
	if got := survivorOf([]patchRow{stub, real}); got.ID != 1245 {
		t.Errorf("survivor = %d, want the published page 1245", got.ID)
	}
	older := patchRow{ID: 10, Published: true, ResourceCount: 1}
	newer := patchRow{ID: 20, Published: true, ResourceCount: 1}
	if got := survivorOf([]patchRow{newer, older}); got.ID != 10 {
		t.Errorf("survivor = %d, want the older id on a tie", got.ID)
	}
}

// A page catalog cannot name has to leave the small-integer space entirely.
// Left where it is, catalog eventually mints a work with that number and the
// two collide with nothing to say they are different games.
func TestPlanParksPagesCatalogCannotName(t *testing.T) {
	p := buildPlan(rows(patchRow{ID: 7668, VndbID: "v134628", ResourceCount: 1, Published: true}), nil)
	to, action := targetOf(t, p, 7668)
	if action != "park" || to != localBase+7668 {
		t.Fatalf("7668 -> %d (%s), want %d park", to, action, localBase+7668)
	}
	if p.Parked != 1 || len(p.Moves) != 1 || len(p.Redirects) != 1 {
		t.Errorf("parked = %d, moves = %d, redirects = %d", p.Parked, len(p.Moves), len(p.Redirects))
	}
}

// Every destination has to be unique before a single row is written: the
// renumber parks all of them and then lands them, and a duplicate target turns
// that second statement into a primary-key violation halfway through.
func TestPlanTargetsAreUnique(t *testing.T) {
	p := buildPlan(
		rows(
			patchRow{ID: 1, VndbID: "v1"},
			patchRow{ID: 2, VndbID: "v2"},
			patchRow{ID: 3, VndbID: "v3", Published: true},
			patchRow{ID: 4, VndbID: "v4"},
		),
		map[int]int64{1: 2, 2: 1, 3: 9, 4: 9},
	)
	seen := map[int]bool{}
	for _, r := range p.Rows {
		if r.Action == "fold" {
			continue
		}
		if seen[r.NewID] {
			t.Fatalf("target %d assigned twice: %+v", r.NewID, p.Rows)
		}
		seen[r.NewID] = true
	}
}
