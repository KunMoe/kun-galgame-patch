package repository

import "testing"

func TestResourcesNarrowByType(t *testing.T) {
	db := feedDB(t)
	repo := New(db)
	mustExec(t, db, `INSERT INTO patch_resource (id, storage, user_id, galgame_id, type, updated)
		VALUES (?, 'user', ?, ?, '["save"]', now()), (?, 'user', ?, ?, '["manual", "fix"]', now())`,
		feedRes, feedUser, feedPatch, feedRes2, feedUser, feedPatch)

	for _, tc := range []struct {
		types []string
		want  []int
	}{
		{nil, []int{feedRes, feedRes2}},
		{[]string{"save"}, []int{feedRes}},
		{[]string{"fix", "crack"}, []int{feedRes2}},
		{[]string{"crack"}, nil},
	} {
		rows, err := repo.ListResources(feedPatch, tc.types, 0, 10)
		if err != nil {
			t.Fatal(err)
		}
		var got []int
		for _, r := range rows {
			got = append(got, r.ID)
		}
		n, err := repo.CountResources(feedPatch, tc.types)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != len(tc.want) || int(n) != len(tc.want) {
			t.Errorf("type %v listed %v (count %d), want %v", tc.types, got, n, tc.want)
			continue
		}
		for _, id := range tc.want {
			found := false
			for _, g := range got {
				found = found || g == id
			}
			if !found {
				t.Errorf("type %v listed %v, want %v", tc.types, got, tc.want)
			}
		}
	}
}
