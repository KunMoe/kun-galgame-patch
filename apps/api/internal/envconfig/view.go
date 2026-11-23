package envconfig

import (
	"context"
	"sort"
	"time"

	"kun-galgame-patch-api/pkg/config"
)

type Issue string

const (
	IssuePanelRedundant      Issue = "panel_redundant"
	IssuePanelOverridesStore Issue = "panel_overrides_store"
	IssueRestartPending      Issue = "restart_pending"
	IssueNotRead             Issue = "not_read"
)

type Config struct {
	IsStoreEnabled  bool       `json:"is_store_enabled"`
	LoadedAt        time.Time  `json:"loaded_at"`
	Variables       []Variable `json:"variables"`
	UnreadVariables []string   `json:"unread_variables"`
}

type Variable struct {
	Variable     string     `json:"variable"`
	Kind         string     `json:"variable_kind"`
	ResolvedFrom *string    `json:"resolved_from"`
	Running      *Value     `json:"running"`
	DefaultValue *string    `json:"default_value"`
	Stored       *Value     `json:"stored"`
	StoredAt     *time.Time `json:"stored_at"`
	Issues       []Issue    `json:"issues"`
	IsStorable   bool       `json:"is_storable"`
}

// Value never carries a secret: one is shown by its keyed fingerprint, which
// is equal for equal values, and its last characters.
type Value struct {
	IsSet       bool    `json:"is_set"`
	Plain       *string `json:"plain"`
	Fingerprint *string `json:"fingerprint"`
	Hint        *string `json:"hint"`
}

func kind(name string) string {
	switch {
	case config.IsBootstrap(name):
		return "bootstrap"
	case config.IsTopology(name):
		return "topology"
	case config.IsSecret(name):
		return "secret"
	default:
		return "plain"
	}
}

func valueOf(name, value string) *Value {
	v := &Value{IsSet: value != ""}
	if !config.IsSecret(name) {
		v.Plain = &value
	} else if value != "" {
		fp, hint := config.Fingerprint(value), config.Hint(value)
		v.Fingerprint, v.Hint = &fp, &hint
	}
	return v
}

func variableOf(running *config.Var, stored *Stored) Variable {
	name := ""
	if running != nil {
		name = running.Key
	} else {
		name = stored.Key
	}
	out := Variable{Variable: name, Kind: kind(name), IsStorable: config.Storable(name)}
	if running != nil {
		from := string(running.Source)
		out.ResolvedFrom = &from
		out.Running = valueOf(name, running.Value)
		if !config.IsSecret(name) {
			def := running.Default
			out.DefaultValue = &def
		}
	}
	if stored != nil {
		out.Stored = valueOf(name, stored.Value)
		at := stored.Updated
		out.StoredAt = &at
	}
	out.Issues = issues(running, stored)
	return out
}

func issues(running *config.Var, stored *Stored) []Issue {
	out := []Issue{}
	fromEnv := running != nil && running.Source == config.SourceEnv
	switch {
	case stored != nil && running == nil:
		return append(out, IssueNotRead)
	case stored != nil && fromEnv && running.Value == stored.Value:
		out = append(out, IssuePanelRedundant)
	case stored != nil && fromEnv:
		out = append(out, IssuePanelOverridesStore)
	}
	switch {
	case stored != nil && !fromEnv && stored.Updated.After(config.LoadedAt()):
		out = append(out, IssueRestartPending)
	case stored == nil && running != nil && running.Source == config.SourceStore:
		out = append(out, IssueRestartPending)
	}
	return out
}

func (s *Store) Snapshot(ctx context.Context) (Config, error) {
	stored, err := s.All(ctx)
	if err != nil {
		return Config{}, err
	}
	byName := map[string]*Stored{}
	for i := range stored {
		byName[stored[i].Key] = &stored[i]
	}
	vars := config.Vars()
	out := Config{
		IsStoreEnabled: s.Enabled(), LoadedAt: config.LoadedAt(),
		Variables: make([]Variable, 0, len(vars)+len(stored)), UnreadVariables: config.UnreadEnv(),
	}
	for i := range vars {
		out.Variables = append(out.Variables, variableOf(&vars[i], byName[vars[i].Key]))
		delete(byName, vars[i].Key)
	}
	for _, st := range byName {
		out.Variables = append(out.Variables, variableOf(nil, st))
	}
	sort.Slice(out.Variables, func(i, j int) bool { return out.Variables[i].Variable < out.Variables[j].Variable })
	if out.UnreadVariables == nil {
		out.UnreadVariables = []string{}
	}
	return out, nil
}

func (s *Store) Variable(ctx context.Context, name string) (Variable, error) {
	snap, err := s.Snapshot(ctx)
	if err != nil {
		return Variable{}, err
	}
	for _, v := range snap.Variables {
		if v.Variable == name {
			return v, nil
		}
	}
	return variableOf(&config.Var{Key: name, Source: config.SourceDefault}, nil), nil
}
