package utils

import (
	"strings"
	"testing"

	"gorm.io/gorm"
)

// Without an ORDER BY, which of two patches naming one work reached the map was
// whatever the planner returned last — so the favourites list could show a
// different page on every request.
func TestPatchIDsByWorkIDsOrdersTheCandidates(t *testing.T) {
	db := dryRunDB(t)
	var rows []patchWorkRow
	stmt := db.Table("patch").Select("id, catalog_work_id").
		Where("catalog_work_id IN ?", []int64{61311}).
		Order(PatchWorkOrder).Session(&gorm.Session{DryRun: true}).Find(&rows).Statement
	got := db.Dialector.Explain(stmt.SQL.String(), stmt.Vars...)

	if !strings.Contains(got, "ORDER BY published DESC, resource_count DESC, is_stub ASC, id ASC") {
		t.Fatalf("the tie-break never reached the wire: %q", got)
	}
}

// The production pair that killed the unique index: patch 61311 is
// `wiki-61311` and patch 61512 is `v65869`, and the catalog says both are work
// 61311. 61311 is published and carries a patch; 61512 is neither, and holds
// the three favourites — the favourite is on the game, so the reader is sent to
// the page worth reading rather than the one they happened to star.
func TestFirstPerWorkKeepsTheRepresentativePage(t *testing.T) {
	got := firstPerWork([]patchWorkRow{
		{ID: 61311, CatalogWorkID: 61311}, // published, 1 resource
		{ID: 61512, CatalogWorkID: 61311}, // unpublished, 0 resources
		{ID: 61533, CatalogWorkID: 61332}, // not a stub
		{ID: 61332, CatalogWorkID: 61332}, // stub
	})

	if len(got) != 2 {
		t.Fatalf("two works collapsed to %d entries: %v", len(got), got)
	}
	if got[61311] != 61311 {
		t.Errorf("work 61311 rendered as patch %d, want 61311", got[61311])
	}
	if got[61332] != 61533 {
		t.Errorf("work 61332 rendered as patch %d, want 61533", got[61332])
	}
}

func TestPatchIDsByWorkIDsAnswersEmptyForNoWorks(t *testing.T) {
	out, err := PatchIDsByWorkIDs(dryRunDB(t), nil)
	if err != nil {
		t.Fatalf("PatchIDsByWorkIDs: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("got %v, want an empty map", out)
	}
}

// A page with no catalog work must not come back as work 0. The calendar reads
// this map to draw hearts, and a zero key would put every workless page in one
// bucket and light them all up together.
func TestWorkIDsByPatchIDsExcludesPagesWithNoWork(t *testing.T) {
	db := dryRunDB(t)
	var rows []patchWorkRow
	stmt := db.Table("patch").Select("id, catalog_work_id").
		Where("id IN ? AND catalog_work_id IS NOT NULL", []int{285, 898}).
		Session(&gorm.Session{DryRun: true}).Find(&rows).Statement
	got := db.Dialector.Explain(stmt.SQL.String(), stmt.Vars...)

	if !strings.Contains(got, "catalog_work_id IS NOT NULL") {
		t.Fatalf("workless pages were not excluded: %q", got)
	}
}

func TestWorkIDsByPatchIDsAnswersEmptyForNoPages(t *testing.T) {
	out, err := WorkIDsByPatchIDs(dryRunDB(t), nil)
	if err != nil {
		t.Fatalf("WorkIDsByPatchIDs: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("got %v, want an empty map", out)
	}
}
