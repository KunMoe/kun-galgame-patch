package client

import "testing"

func TestNormalizeCreditName(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"保住圭", "保住圭"},
		{"保住圭 (Hozumi Kei)", "保住圭"},
		{"保住圭（ほずみけい）", "保住圭"},
		{"(有)PINA(ぴなぽんな)", "(有)PINA"},
		{"(有)スタジオナレッジ", "(有)スタジオナレッジ"},
		{"（株）ビジュアルアーツ", "（株）ビジュアルアーツ"},
		{"(ぴなぽんな)", "(ぴなぽんな)"},
		{"", ""},
	} {
		if got := normalizeCreditName(tc.in); got != tc.want {
			t.Errorf("normalizeCreditName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestCompanyCreditsSurviveStaffFolding(t *testing.T) {
	groups := []catalogCreditGroup{{
		RoleKey: "other-staff",
		Credits: []catalogCreditItem{
			{catalogPersonRef: catalogPersonRef{ID: 20488, DisplayName: "(有)PINA(ぴなぽんな)", Lang: "ja"}},
			{catalogPersonRef: catalogPersonRef{ID: 19357, DisplayName: "(有)スタジオナレッジ", Lang: "ja"}},
		},
	}}
	staff := catalogStaff(groups, nil)
	n := 0
	for _, g := range staff {
		n += len(g.People)
	}
	if n != 2 {
		t.Fatalf("catalogStaff kept %d credits, want 2: %+v", n, staff)
	}
}
