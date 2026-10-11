package utils

import (
	"os"
	"slices"
	"strings"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestScopeExcludedIDs(t *testing.T) {
	db := dryRunDB(t)
	explain := func(stmt *gorm.Statement) string {
		return db.Dialector.Explain(stmt.SQL.String(), stmt.Vars...)
	}

	var rows []patchRow
	got := explain(ScopeExcludedIDs(db.Model(&patchRow{}), "patch.id", []int{12, 3021}).
		Session(&gorm.Session{DryRun: true}).Find(&rows).Statement)
	if !strings.Contains(got, `patch.id NOT IN (SELECT unnest('{12,3021}'::int[]))`) {
		t.Errorf("produced %q", got)
	}

	var n int64
	got = explain(ScopeExcludedIDs(db.Model(&patchRow{}), "patch.id", []int{12}).
		Session(&gorm.Session{DryRun: true}).Count(&n).Statement)
	if !strings.Contains(got, "count(*)") || !strings.Contains(got, `patch.id NOT IN (SELECT unnest('{12}'::int[]))`) {
		t.Errorf("COUNT produced %q", got)
	}

	got = explain(ScopeExcludedIDs(db.Model(&patchRow{}), "patch.id", nil).
		Session(&gorm.Session{DryRun: true}).Find(&rows).Statement)
	if strings.Contains(got, "unnest") {
		t.Errorf("no exclusion produced %q, want no predicate", got)
	}
}

// One tag tree already excludes 48,620 works; the predicate has to survive a
// real round trip past the 65,535 bind parameters an IN list would need.
func TestScopeExcludedIDsOnPostgres(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN is not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	excluded := make([]int, 0, 70_000)
	for id := 2; len(excluded) < cap(excluded); id += 2 {
		excluded = append(excluded, id)
	}
	q := db.Table("(SELECT generate_series(1, 10) AS id) AS patch")

	var kept []int
	if err := ScopeExcludedIDs(q.Session(&gorm.Session{}), "patch.id", excluded).Order("id").Pluck("id", &kept).Error; err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(kept, []int{1, 3, 5, 7, 9}) {
		t.Fatalf("kept %v, want the odd ids", kept)
	}
	var n int64
	if err := ScopeExcludedIDs(q.Session(&gorm.Session{}), "patch.id", excluded).Count(&n).Error; err != nil || n != 5 {
		t.Fatalf("count = %d, %v; want 5", n, err)
	}
}
