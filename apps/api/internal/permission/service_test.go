package permission

import (
	"testing"

	"kun-galgame-patch-api/pkg/perm"

	"github.com/stretchr/testify/assert"
)

func TestValidateRoleChanges(t *testing.T) {
	admin := Caller{ID: 1, Roles: []string{"admin"}}
	grant := []perm.Override{{Permission: perm.ResourceReorder, Effect: perm.EffectGrant}}
	revoke := []perm.Override{{Permission: perm.ResourceReorder, Effect: perm.EffectRevoke}}

	cases := []struct {
		name    string
		caller  Caller
		changes []RoleChange
		stored  map[string][]perm.Override
		ok      bool
	}{
		{"admin grants creator", admin, []RoleChange{{"creator", grant}}, nil, true},
		{"admin revokes moderator", admin, []RoleChange{{"moderator", revoke}}, nil, true},
		{"reset is an empty set", admin, []RoleChange{{"moderator", nil}}, nil, true},
		{"admin cannot touch its own rank", admin, []RoleChange{{"admin", revoke}}, nil, false},
		{"ren is locked", Caller{ID: 1, Roles: []string{"ren"}}, []RoleChange{{"ren", revoke}}, nil, false},
		{"unknown role", admin, []RoleChange{{"user", grant}}, nil, false},
		{"a grant the baseline already has", admin, []RoleChange{{"moderator", grant}}, nil, false},
		{"repeated role", admin, []RoleChange{{"creator", grant}, {"creator", nil}}, nil, false},
		{"unknown key", admin, []RoleChange{{"creator", []perm.Override{{Permission: "x.y", Effect: perm.EffectGrant}}}}, nil, false},
		{
			"moderator may not exceed admin",
			Caller{ID: 1, Roles: []string{"ren"}},
			[]RoleChange{{"admin", revoke}},
			nil, false,
		},
		{
			"admin and moderator revoked together",
			Caller{ID: 1, Roles: []string{"ren"}},
			[]RoleChange{{"admin", revoke}, {"moderator", revoke}},
			nil, true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			vs := validateRoleChanges(tc.caller, tc.stored, tc.changes)
			if tc.ok {
				assert.Empty(t, vs)
			} else {
				assert.NotEmpty(t, vs)
			}
		})
	}
}

func TestCallerMustHoldWhatTheyGrant(t *testing.T) {
	t.Cleanup(func() { perm.Load(nil, nil) })
	perm.Load(nil, map[int][]perm.Override{1: {{Permission: perm.ResourceReorder, Effect: perm.EffectRevoke}}})

	grant := []perm.Override{{Permission: perm.ResourceReorder, Effect: perm.EffectGrant}}
	caller := Caller{ID: 1, Roles: []string{"admin"}}

	assert.NotEmpty(t, validateOverrides(caller, nil, nil, grant, "creator"), "granting a key the caller lacks")
	assert.Empty(t, validateOverrides(caller, nil, grant, grant, "creator"), "carrying a stored override over untouched")
	assert.NotEmpty(t, validateOverrides(caller, nil, grant, nil, "creator"), "removing an override on a key the caller lacks")
}
