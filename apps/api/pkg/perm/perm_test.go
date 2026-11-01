package perm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBaselineGivesReorderToModeratorAndAbove(t *testing.T) {
	t.Cleanup(func() { Load(nil, nil) })
	Load(nil, nil)

	assert.False(t, CanUser(1, nil, ResourceReorder))
	assert.False(t, CanUser(1, []string{"user", "creator"}, ResourceReorder))
	for _, r := range []string{"moderator", "admin", "ren"} {
		assert.True(t, CanUser(1, []string{r}, ResourceReorder), r)
	}
}

func TestRoleAndUserOverrides(t *testing.T) {
	t.Cleanup(func() { Load(nil, nil) })
	Load(
		map[string][]Override{
			"moderator": {{ResourceReorder, EffectRevoke}},
			"creator":   {{ResourceReorder, EffectGrant}},
			"ren":       {{ResourceReorder, EffectRevoke}},
		},
		map[int][]Override{
			7: {{ResourceReorder, EffectGrant}},
			8: {{ResourceReorder, EffectRevoke}},
		},
	)

	assert.False(t, CanUser(1, []string{"moderator"}, ResourceReorder))
	assert.True(t, CanUser(1, []string{"creator"}, ResourceReorder))
	assert.True(t, CanUser(1, []string{"ren"}, ResourceReorder), "ren ignores stored overrides")
	assert.True(t, CanUser(7, nil, ResourceReorder), "a personal grant works without any role")
	assert.False(t, CanUser(8, []string{"admin"}, ResourceReorder), "a personal revoke beats the role")
	assert.True(t, CanUser(8, []string{"ren"}, ResourceReorder), "nothing reduces a ren holder")
}

func TestUnknownKeysAreIgnored(t *testing.T) {
	got := Apply(nil, []Override{{"no.such_key", EffectGrant}, {ResourceReorder, EffectGrant}})
	assert.Equal(t, []Permission{ResourceReorder}, got)
}

func TestRank(t *testing.T) {
	assert.Equal(t, 0, Rank([]string{"user"}))
	assert.Less(t, Rank([]string{"moderator"}), Rank([]string{"user", "admin"}))
	assert.Less(t, Rank([]string{"admin"}), Rank([]string{"ren"}))
}

func TestBaselinesStayInsideTheCatalogAndModeratorInsideAdmin(t *testing.T) {
	for role, keys := range baselines {
		for _, k := range keys {
			assert.True(t, IsKnown(k), "%s baseline names unknown key %s", role, k)
		}
	}
	admin := Baseline("admin")
	for _, k := range Baseline("moderator") {
		assert.Contains(t, admin, k)
	}
	for _, e := range Catalog() {
		assert.Regexp(t, `^[a-z][a-z_]*(\.[a-z][a-z_]*)+$`, string(e.Key))
		assert.NotEmpty(t, e.Label)
		assert.Contains(t, Groups(), GroupEntry{e.Group, groupLabel(e.Group)})
	}
}

// The cutover from hard-coded role lists must not move any power by itself.
func TestBaselineMatchesTheRoleListsItReplaced(t *testing.T) {
	t.Cleanup(func() { Load(nil, nil) })
	Load(nil, nil)

	moderatorOnly := []Permission{ResourceEditAny, ResourceDeleteAny, ResourceDisableAny, PatchEditAny,
		CommentEditAny, CommentDeleteAny, ChatMessageDeleteAny, AdminAccess, TrustReview, DocManage}
	adminOnly := []Permission{PatchDeleteAny, RatingDeleteAny, ChatRoomCreate, SettingManage, CatalogMergeUndo, UserPurge}

	for _, p := range moderatorOnly {
		assert.False(t, CanUser(1, []string{"creator"}, p), p)
		assert.True(t, CanUser(1, []string{"moderator"}, p), p)
		assert.True(t, CanUser(1, []string{"admin"}, p), p)
	}
	for _, p := range adminOnly {
		assert.False(t, CanUser(1, []string{"moderator"}, p), p)
		assert.True(t, CanUser(1, []string{"admin"}, p), p)
		assert.True(t, CanUser(1, []string{"ren"}, p), p)
	}
	assert.True(t, CanUser(1, []string{"creator"}, GalgamePublishRestricted))
	assert.False(t, CanUser(1, nil, GalgamePublishRestricted))
}

func groupLabel(g Group) string {
	for _, e := range groups {
		if e.Key == g {
			return e.Label
		}
	}
	return ""
}
