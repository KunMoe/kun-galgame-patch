package main

import (
	"bufio"
	"fmt"
	"os"
	"sort"
)

const (
	// Transient, and only ever inside the renumber transaction: 9,519 of the
	// old ids are also somebody's new id, so no ordering of the updates avoids
	// a primary-key collision and every row that moves has to stand somewhere
	// else first.
	parkBase = 1_000_000_000

	// Permanent home for a page catalog cannot name. Leaving it on a small
	// integer is what this whole change exists to end: catalog will eventually
	// mint a work with that number and the two would collide silently. All 20
	// such pages already answer 404 today, so nothing reachable moves here.
	localBase = 1_500_000_000
)

type move struct {
	OldID int
	NewID int
}

type fold struct {
	Target   int
	Survivor int
	Losers   []int
}

type planRow struct {
	OldID  int
	NewID  int
	Vndb   string
	Action string
}

// Plan is the whole renumber, decided before a single row is written.
type Plan struct {
	Total      int
	Identical  int
	Parked     int
	FoldLosers int
	Moves      []move
	Folds      []fold
	Redirects  []move
	Rows       []planRow
}

// survivorOf is which page keeps the game when two of them turn out to be one
// work: published first, then the one that actually carries resources, then a
// real page over a stub, then the older id. Same order the site already used to
// pick a work's representative, so the page a reader lands on does not change.
func survivorOf(rows []patchRow) patchRow {
	best := rows[0]
	for _, r := range rows[1:] {
		switch {
		case r.Published != best.Published:
			if r.Published {
				best = r
			}
		case r.ResourceCount != best.ResourceCount:
			if r.ResourceCount > best.ResourceCount {
				best = r
			}
		case r.IsStub != best.IsStub:
			if !r.IsStub {
				best = r
			}
		case r.ID < best.ID:
			best = r
		}
	}
	return best
}

func buildPlan(rows []patchRow, targets map[int]int64) *Plan {
	p := &Plan{Total: len(rows)}

	byTarget := map[int][]patchRow{}
	var unmapped []patchRow
	for _, r := range rows {
		t, ok := targets[r.ID]
		if !ok || t <= 0 {
			unmapped = append(unmapped, r)
			continue
		}
		byTarget[int(t)] = append(byTarget[int(t)], r)
	}

	targetIDs := make([]int, 0, len(byTarget))
	for t := range byTarget {
		targetIDs = append(targetIDs, t)
	}
	sort.Ints(targetIDs)

	for _, t := range targetIDs {
		group := byTarget[t]
		keep := group[0]
		if len(group) > 1 {
			keep = survivorOf(group)
			f := fold{Target: t, Survivor: keep.ID}
			for _, r := range group {
				if r.ID == keep.ID {
					continue
				}
				f.Losers = append(f.Losers, r.ID)
				p.FoldLosers++
				p.Redirects = append(p.Redirects, move{OldID: r.ID, NewID: t})
				p.Rows = append(p.Rows, planRow{r.ID, t, r.VndbID, "fold"})
			}
			sort.Ints(f.Losers)
			p.Folds = append(p.Folds, f)
		}
		// An unmoved page gets a ledger row too, and that is not redundant.
		// /patch/<n> resolves ONLY out of this table: a miss has to mean "no
		// page ever had that number", because the alternative -- treating a
		// miss as "same number, go to /galgame/<n>" -- makes one absent row
		// silently serve a different game, which is the failure this whole
		// change exists to end.
		p.Redirects = append(p.Redirects, move{OldID: keep.ID, NewID: t})
		if keep.ID == t {
			p.Identical++
			p.Rows = append(p.Rows, planRow{keep.ID, t, keep.VndbID, "identical"})
			continue
		}
		p.Moves = append(p.Moves, move{OldID: keep.ID, NewID: t})
		p.Rows = append(p.Rows, planRow{keep.ID, t, keep.VndbID, "align"})
	}

	for _, r := range unmapped {
		to := localBase + r.ID
		p.Parked++
		p.Moves = append(p.Moves, move{OldID: r.ID, NewID: to})
		p.Redirects = append(p.Redirects, move{OldID: r.ID, NewID: to})
		p.Rows = append(p.Rows, planRow{r.ID, to, r.VndbID, "park"})
	}

	sort.Slice(p.Moves, func(i, j int) bool { return p.Moves[i].OldID < p.Moves[j].OldID })
	sort.Slice(p.Redirects, func(i, j int) bool { return p.Redirects[i].OldID < p.Redirects[j].OldID })
	sort.Slice(p.Rows, func(i, j int) bool { return p.Rows[i].OldID < p.Rows[j].OldID })
	return p
}

func writePlan(path string, p *Plan) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	if _, err := fmt.Fprintln(w, "old_id\tnew_id\tvndb_id\taction"); err != nil {
		return err
	}
	for _, r := range p.Rows {
		if _, err := fmt.Fprintf(w, "%d\t%d\t%s\t%s\n", r.OldID, r.NewID, r.Vndb, r.Action); err != nil {
			return err
		}
	}
	return w.Flush()
}
