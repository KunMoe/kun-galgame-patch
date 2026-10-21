package utils

import (
	"strings"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type patchRow struct {
	ID int
}

func (patchRow) TableName() string { return "patch" }

func dryRunDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(
		postgres.New(postgres.Config{DSN: "postgres://u:p@127.0.0.1:1/none", PreferSimpleProtocol: true}),
		&gorm.Config{DryRun: true, DisableAutomaticPing: true},
	)
	if err != nil {
		t.Fatalf("open dry-run db: %v", err)
	}
	return db
}

func sqlOf(t *testing.T, db *gorm.DB, contentLimit string) string {
	t.Helper()
	var rows []patchRow
	stmt := ScopeListablePatches(db.Model(&patchRow{}), contentLimit).
		Session(&gorm.Session{DryRun: true}).Find(&rows).Statement
	return db.Dialector.Explain(stmt.SQL.String(), stmt.Vars...)
}

func TestScopeListablePatches(t *testing.T) {
	db := dryRunDB(t)

	t.Run("an unmirrored row passes every gate", func(t *testing.T) {
		for _, cl := range []string{ContentLimitSFW, ContentLimitNSFW} {
			got := sqlOf(t, db, cl)
			if !strings.Contains(got, "patch.content_limit IS NULL OR patch.content_limit = '"+cl+"'") {
				t.Errorf("content_limit=%s produced %q", cl, got)
			}
		}
	})

	t.Run("all and the empty gate add no predicate", func(t *testing.T) {
		for _, cl := range []string{ContentLimitAll, ""} {
			if got := sqlOf(t, db, cl); strings.Contains(got, "content_limit") {
				t.Errorf("content_limit=%q produced %q, want no predicate", cl, got)
			}
		}
	})

	t.Run("the same builder carries the predicate into COUNT", func(t *testing.T) {
		var n int64
		stmt := ScopeListablePatches(db.Model(&patchRow{}), ContentLimitSFW).
			Session(&gorm.Session{DryRun: true}).Count(&n).Statement
		got := db.Dialector.Explain(stmt.SQL.String(), stmt.Vars...)
		if !strings.Contains(got, "count(*)") || !strings.Contains(got, "content_limit") {
			t.Errorf("COUNT query = %q, want the axis filtered there too", got)
		}
	})
}
