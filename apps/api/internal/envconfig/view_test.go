package envconfig

import (
	"reflect"
	"testing"
	"time"

	"kun-galgame-patch-api/pkg/config"
)

func TestIssues(t *testing.T) {
	booted := config.LoadedAt()
	before, after := booted.Add(-time.Hour), booted.Add(time.Hour)
	run := func(src config.Source, v string) *config.Var {
		return &config.Var{Key: "KUN_X_API_KEY", Value: v, Source: src}
	}
	stored := func(v string, at time.Time) *Stored {
		return &Stored{Key: "KUN_X_API_KEY", Value: v, Updated: at}
	}
	for _, tc := range []struct {
		why     string
		running *config.Var
		stored  *Stored
		want    []Issue
	}{
		{"env only", run(config.SourceEnv, "a"), nil, []Issue{}},
		{"panel copy of the stored value", run(config.SourceEnv, "a"), stored("a", after), []Issue{IssuePanelRedundant}},
		{"panel differs from the store", run(config.SourceEnv, "a"), stored("b", before), []Issue{IssuePanelOverridesStore}},
		{"in use from the store", run(config.SourceStore, "a"), stored("a", before), []Issue{}},
		{"stored after the start", run(config.SourceDefault, ""), stored("a", after), []Issue{IssueRestartPending}},
		{"deleted after the start", run(config.SourceStore, "a"), nil, []Issue{IssueRestartPending}},
		{"stored, never read", nil, stored("a", before), []Issue{IssueNotRead}},
	} {
		if got := issues(tc.running, tc.stored); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: issues = %v, want %v", tc.why, got, tc.want)
		}
	}
}

func TestSecretsNeverShowTheirValue(t *testing.T) {
	v := variableOf(&config.Var{Key: "KUN_X_API_KEY", Value: "nmk_live_secret", Source: config.SourceEnv},
		&Stored{Key: "KUN_X_API_KEY", Value: "nmk_live_secret"})
	if v.Kind != "secret" || v.Running.Plain != nil || v.Stored.Plain != nil || v.DefaultValue != nil ||
		v.Running.Fingerprint == nil || *v.Running.Hint != "…cret" {
		t.Fatalf("secret shown as %+v / %+v", v.Running, v.Stored)
	}
	if p := variableOf(&config.Var{Key: config.DatabaseURLEnv, Source: config.SourceEnv}, nil); p.IsStorable || p.Kind != "bootstrap" {
		t.Errorf("bootstrap variable %+v", p)
	}
}
